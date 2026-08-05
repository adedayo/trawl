package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// maxBodyBytes bounds ingest payloads. Scan output is large but not unbounded.
const maxBodyBytes = 32 << 20 // 32 MiB

// server holds the dependencies shared by every HTTP handler.
//
// This process is the sole ingest target and job broker for the compose
// deployment: workers poll it for work and post results back to it. There is
// no separate backend database service — state lives in SQLite alongside it.
type server struct {
	store *sqlite.SQLiteStore
	bus   event.Bus
	token string // optional bearer token; empty disables auth
}

func runServer() {
	dbStore, err := sqlite.NewSQLiteStore(os.Getenv("TRAWL_DB_PATH"))
	if err != nil {
		log.Fatalf("failed to initialize sqlite store: %v", err)
	}
	defer dbStore.Close()

	srv := &server{
		store: dbStore,
		bus:   event.NewMemoryBus(),
		token: os.Getenv("TRAWL_AUTH_TOKEN"),
	}

	if srv.token == "" {
		log.Println("WARNING: TRAWL_AUTH_TOKEN is unset — ingest endpoints are unauthenticated")
	}

	addr := os.Getenv("TRAWL_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	srv.routes(mux)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Trawl server listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func (s *server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Read API consumed by the dashboard.
	mux.HandleFunc("GET /api/v1/assets", s.handleGetAssets)
	mux.HandleFunc("GET /api/v1/findings", s.handleGetFindings)
	mux.HandleFunc("GET /api/v1/email-postures", s.handleGetEmailPostures)
	mux.HandleFunc("GET /api/v1/jobs", s.handleGetJobs)

	// Job queue consumed by worker containers.
	mux.HandleFunc("POST /api/jobs", s.authed(s.handleEnqueueJob))
	mux.HandleFunc("GET /api/jobs/pop", s.authed(s.handlePopJob))
	mux.HandleFunc("POST /api/jobs/complete", s.authed(s.handleCompleteJob))

	// Result ingest posted by worker containers.
	mux.HandleFunc("POST /api/ingest/discovery", s.authed(s.handleIngestDiscovery))
	mux.HandleFunc("POST /api/ingest/scan", s.authed(s.handleIngestScan))
	mux.HandleFunc("POST /api/ingest/secrets", s.authed(s.handleIngestSecrets))
	mux.HandleFunc("POST /api/ingest/email-posture", s.authed(s.handleIngestEmailPosture))

	mux.HandleFunc("/ws", s.handleWebSocket)
}

// authed wraps a handler with bearer-token checking. When no token is
// configured the wrapper is a pass-through, which keeps loopback-only
// deployments frictionless while still allowing the token to be required.
func (s *server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && r.Header.Get("Authorization") != "Bearer "+s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// ─── Read API ────────────────────────────────────────────────────────────────

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleGetAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := s.store.GetAssets(r.Context(), store.AssetStatus(r.URL.Query().Get("status")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

func (s *server) handleGetFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := s.store.GetFindings(r.Context(), r.URL.Query().Get("assetId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, findings)
}

func (s *server) handleGetEmailPostures(w http.ResponseWriter, r *http.Request) {
	postures, err := s.store.GetEmailPostures(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, postures)
}

func (s *server) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.GetJobs(r.Context(), store.JobStatus(r.URL.Query().Get("status")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// ─── Job queue ───────────────────────────────────────────────────────────────

func (s *server) handleEnqueueJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string   `json:"type"`
		Targets []string `json:"targets"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	job, err := s.store.EnqueueJob(r.Context(), req.Type, req.Targets)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.publish(event.EventJobEnqueued, job)
	writeJSON(w, http.StatusCreated, job)
}

func (s *server) handlePopJob(w http.ResponseWriter, r *http.Request) {
	jobType := r.URL.Query().Get("type")
	if jobType == "" {
		writeError(w, http.StatusBadRequest, errors.New("query parameter 'type' is required"))
		return
	}

	job, err := s.store.PopJob(r.Context(), jobType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if job == nil {
		// An empty queue is the normal steady state, not a failure. Return an
		// empty object with 200 so a polling worker using `curl -f` does not
		// treat a quiet queue as an error.
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	s.publish(event.EventJobStarted, job)
	writeJSON(w, http.StatusOK, job)
}

func (s *server) handleCompleteJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	status := store.JobStatus(req.Status)
	if status == "" {
		status = store.JobStatusCompleted
	}

	if err := s.store.CompleteJob(r.Context(), req.JobID, status, req.Error); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.publish(event.EventJobCompleted, map[string]string{"jobId": req.JobID, "status": string(status)})
	writeJSON(w, http.StatusOK, map[string]string{"status": string(status)})
}

// ─── Ingest ──────────────────────────────────────────────────────────────────

// ingestEnvelope is the common shape posted by every worker.
type ingestEnvelope struct {
	JobRunID string `json:"jobRunId"`
}

func (s *server) handleIngestDiscovery(w http.ResponseWriter, r *http.Request) {
	s.ingestRaw(w, r, "discovery")
}

func (s *server) handleIngestScan(w http.ResponseWriter, r *http.Request) {
	s.ingestRaw(w, r, "scan")
}

func (s *server) handleIngestSecrets(w http.ResponseWriter, r *http.Request) {
	s.ingestRaw(w, r, "secrets")
}

// ingestRaw records a worker payload verbatim against its job run and
// announces it on the bus. Parsing into typed assets and findings belongs to
// the correlation stage; this endpoint's only contract is that nothing
// observed is lost between the worker and the database.
func (s *server) ingestRaw(w http.ResponseWriter, r *http.Request, kind string) {
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, err)
		return
	}

	var env ingestEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	key := "ingest:" + kind + ":" + env.JobRunID + ":" + time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.store.SaveSetting(r.Context(), key, string(body)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.publish(event.EventIngestReceived, map[string]string{
		"kind":     kind,
		"jobRunId": env.JobRunID,
		"key":      key,
	})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "key": key})
}

func (s *server) handleIngestEmailPosture(w http.ResponseWriter, r *http.Request) {
	var posture store.EmailPosture
	if err := decodeJSON(r, &posture); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if posture.Domain == "" {
		writeError(w, http.StatusBadRequest, errors.New("field 'domain' is required"))
		return
	}

	posture.LastChecked = time.Now().UTC()
	if err := s.store.SaveEmailPosture(r.Context(), &posture); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.publish(event.EventIngestReceived, map[string]string{"kind": "email-posture", "domain": posture.Domain})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ─── WebSocket ───────────────────────────────────────────────────────────────

// handleWebSocket is a placeholder until the bus-to-socket broadcaster lands
// (change 003, phase 2). It reports its own incompleteness rather than
// returning 200 and pretending to be a live stream.
func (s *server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "websocket streaming not yet implemented; poll /api/v1/* meanwhile", http.StatusNotImplemented)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (s *server) publish(eventType event.EventType, payload any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(context.Background(), event.Event{Type: eventType, Payload: payload})
}

func decodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes)).Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
