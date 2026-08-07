package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/core"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
)

// maxBodyBytes bounds ingest payloads. Scan output is large but not unbounded.
const maxBodyBytes = 32 << 20 // 32 MiB

// server holds the dependencies shared by every HTTP handler.
//
// This process is the sole ingest target and job broker for the compose
// deployment: workers poll it for work and post results back to it. There is
// no separate backend database service — state lives in SQLite alongside it.
//
// It holds a *core.Core rather than a store because it is a transport. Every
// decision belongs to the application layer, which the desktop build serves
// over Wails IPC; a handler that decided anything here would be a feature the
// container has and the desktop lacks, or the reverse.
type server struct {
	core   *core.Core
	store  store.Store
	bus    event.Bus
	events *broadcaster
	token  string // optional bearer token; empty disables auth

	scanMode scanMode

	// inflight tracks background work so that shutdown can wait for it.
	// Without it, a SIGTERM during a scan abandons the run halfway through and
	// leaves a partial inventory that is indistinguishable from a complete one.
	inflight sync.WaitGroup
}

func runServer() {
	cfg := loadRuntimeConfig()
	cfg.announce()

	dbStore, err := store.Open(cfg.dbDSN)
	if err != nil {
		log.Fatalf("failed to open the store: %v", err)
	}
	defer dbStore.Close()

	bus := event.NewMemoryBus()
	app, err := core.New(dbStore, bus, signals.RegistryJSON())
	if err != nil {
		log.Fatalf("failed to initialize the application layer: %v", err)
	}
	if err := app.Start(context.Background()); err != nil {
		log.Printf("WARNING: could not seed the signal registry: %v", err)
	}

	srv := &server{
		core:     app,
		store:    dbStore,
		bus:      bus,
		events:   newBroadcaster(bus),
		token:    cfg.token,
		scanMode: cfg.scanMode,
	}

	mux := http.NewServeMux()
	srv.routes(mux)

	httpServer := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// An autoscaled platform starts and stops this process routinely, so
	// shutdown is a normal event rather than an exceptional one. Treating it
	// as exceptional is how a mid-write SQLite transaction becomes a corrupt
	// database on an ordinary scale-down.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdown
		log.Println("Shutdown signalled; draining connections and work in flight.")

		ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownGrace)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown did not complete: %v", err)
		}
		srv.waitForInflight(cfg.shutdownGrace)
	}()

	log.Printf("Trawl server listening on %s", cfg.addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
	log.Println("Trawl server stopped.")
}

// waitForInflight blocks until background work finishes or the grace period
// expires, reporting which of the two happened.
//
// The wait is bounded because the platform's own grace period is bounded:
// waiting past it does not save the work, it merely replaces an orderly stop
// with a kill.
func (s *server) waitForInflight(grace time.Duration) {
	done := make(chan struct{})
	go func() {
		s.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(grace):
		log.Println("WARNING: work was still in flight at shutdown and has been abandoned; " +
			"its results are incomplete and will not have been recorded.")
	}
}

func (s *server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Read API consumed by the dashboard. Every capability the desktop build
	// exposes over Wails IPC has an equivalent here; the two are kept in step
	// by both delegating to pkg/core.
	mux.HandleFunc("GET /api/v1/assets", s.handleGetAssets)
	mux.HandleFunc("DELETE /api/v1/assets/{id}", s.authed(s.handleRemoveAsset))
	mux.HandleFunc("GET /api/v1/findings", s.handleGetFindings)
	mux.HandleFunc("GET /api/v1/secret-findings", s.handleGetSecretFindings)
	mux.HandleFunc("GET /api/v1/email-postures", s.handleGetEmailPostures)
	mux.HandleFunc("GET /api/v1/regressions", s.handleGetRegressions)
	mux.HandleFunc("GET /api/v1/jobs", s.handleGetJobs)

	// Measured-state assessment.
	mux.HandleFunc("GET /api/v1/assessments", s.handleGetAssessments)
	mux.HandleFunc("GET /api/v1/assessments/{domain}", s.handleGetAssessment)
	mux.HandleFunc("POST /api/v1/assessments/{domain}", s.authed(s.handleAssessDomain))

	// Scope, settings and scan control. These mutate, so they are authed even
	// though the read API is not: an unauthenticated caller must not be able
	// to widen the authorisation and then scan against it.
	mux.HandleFunc("GET /api/v1/scope", s.handleGetScope)
	mux.HandleFunc("PUT /api/v1/scope", s.authed(s.handleSaveScope))
	mux.HandleFunc("GET /api/v1/settings/{key}", s.handleGetSetting)
	mux.HandleFunc("PUT /api/v1/settings/{key}", s.authed(s.handleSaveSetting))
	mux.HandleFunc("POST /api/v1/scans", s.authed(s.handleTriggerScan))
	mux.HandleFunc("DELETE /api/v1/discovered-data", s.authed(s.handleEraseDiscoveredData))

	// Discovery and the authorisation decisions taken on its proposals. Every
	// one of these is authenticated: discovery reaches a third party, and the
	// authorise route widens the scope the scanner will act on.
	mux.HandleFunc("POST /api/v1/discovery/related", s.authed(s.handleDiscoverRelated))
	mux.HandleFunc("POST /api/v1/discovery/authorise", s.authed(s.handleAuthoriseProposed))
	mux.HandleFunc("POST /api/v1/discovery/dismiss", s.authed(s.handleDismissProposed))
	mux.HandleFunc("POST /api/v1/discovery/restore", s.authed(s.handleRestoreDismissed))
	mux.HandleFunc("GET /api/v1/discovery/dismissed", s.handleGetDismissed)

	// Job queue consumed by worker containers.
	mux.HandleFunc("POST /api/jobs", s.authed(s.handleEnqueueJob))
	mux.HandleFunc("GET /api/jobs/pop", s.authed(s.handlePopJob))
	mux.HandleFunc("POST /api/jobs/complete", s.authed(s.handleCompleteJob))

	// Result ingest posted by worker containers.
	mux.HandleFunc("POST /api/ingest/discovery", s.authed(s.handleIngestDiscovery))
	mux.HandleFunc("POST /api/ingest/scan", s.authed(s.handleIngestScan))
	mux.HandleFunc("POST /api/ingest/secrets", s.authed(s.handleIngestSecrets))
	mux.HandleFunc("POST /api/ingest/email-posture", s.authed(s.handleIngestEmailPosture))

	mux.HandleFunc("GET /api/v1/events", s.handleEventStream)
	mux.HandleFunc("/ws", s.handleWebSocket)

	// The dashboard, in builds that embed it. Registered last so that no
	// static path can shadow an API route.
	s.mountDashboard(mux)
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
	assets, err := s.core.Assets(r.Context(), store.AssetStatus(r.URL.Query().Get("status")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

// handleRemoveAsset deletes an asset and everything recorded against it.
func (s *server) handleRemoveAsset(w http.ResponseWriter, r *http.Request) {
	if err := s.core.RemoveAsset(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *server) handleGetFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := s.core.Findings(r.Context(), r.URL.Query().Get("assetId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, findings)
}

func (s *server) handleGetSecretFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := s.core.SecretFindings(r.Context(), r.URL.Query().Get("repoUrl"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, findings)
}

func (s *server) handleGetEmailPostures(w http.ResponseWriter, r *http.Request) {
	postures, err := s.core.EmailPostures(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, postures)
}

func (s *server) handleGetRegressions(w http.ResponseWriter, r *http.Request) {
	regressions, err := s.core.Regressions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, regressions)
}

func (s *server) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.core.Jobs(r.Context(), store.JobStatus(r.URL.Query().Get("status")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// ─── Assessment ──────────────────────────────────────────────────────────────

func (s *server) handleGetAssessments(w http.ResponseWriter, r *http.Request) {
	views, err := s.core.Assessments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *server) handleGetAssessment(w http.ResponseWriter, r *http.Request) {
	view, err := s.core.Assessment(r.Context(), r.PathValue("domain"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// handleAssessDomain runs an assessment synchronously.
//
// It blocks rather than returning 202 with a job identifier because the
// assessment is bounded and short, and because a caller who receives the
// result can act on it. A refusal — an unauthorised scope — is a 200 carrying
// the refused outcome, not an error: the assessment was correctly declined,
// and that is a result the operator needs recorded.
func (s *server) handleAssessDomain(w http.ResponseWriter, r *http.Request) {
	view, err := s.core.AssessDomain(r.Context(), r.PathValue("domain"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// ─── Scope, settings and scan control ────────────────────────────────────────

func (s *server) handleGetScope(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.core.Scope(r.Context()))
}

func (s *server) handleSaveScope(w http.ResponseWriter, r *http.Request) {
	var scope core.Scope
	if err := decodeJSON(r, &scope); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.SaveScope(r.Context(), scope); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, scope)
}

// handleEraseDiscoveredData clears everything the engine discovered, keeping
// the operator's configuration and authorised scope.//
// A failure is reported as one. The client will tell the operator their estate
// was erased on the strength of this response, so answering 200 for a wipe
// that did not happen would manufacture a false belief about their own data.
func (s *server) handleEraseDiscoveredData(w http.ResponseWriter, r *http.Request) {
	if err := s.core.EraseDiscoveredData(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "erased"})
}

// handleDiscoverRelated searches certificate transparency for domains related
// to the authorised ones and returns them as proposals.
//
// It assesses nothing and sends no packet to any discovered domain, so it is
// safe to run speculatively. The operator decides what is theirs afterwards.
func (s *server) handleDiscoverRelated(w http.ResponseWriter, r *http.Request) {
	var opts service.DiscoveryOptions
	// An empty body is a legitimate request for the defaults, so a decode
	// failure is only an error when something was actually sent.
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	result, err := s.core.DiscoverRelatedDomains(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// domainsRequest is the body shared by the bulk decision routes.
type domainsRequest struct {
	Domains []string `json:"domains"`
	Domain  string   `json:"domain"`
}

func (s *server) handleAuthoriseProposed(w http.ResponseWriter, r *http.Request) {
	var req domainsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.AuthoriseProposedDomains(r.Context(), req.Domains); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "authorised"})
}

func (s *server) handleDismissProposed(w http.ResponseWriter, r *http.Request) {
	var req domainsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.DismissProposedDomains(r.Context(), req.Domains); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func (s *server) handleRestoreDismissed(w http.ResponseWriter, r *http.Request) {
	var req domainsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.RestoreDismissedDomain(r.Context(), req.Domain); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

func (s *server) handleGetDismissed(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.core.DismissedDomains(r.Context()))
}

func (s *server) handleGetSetting(w http.ResponseWriter, r *http.Request) {
	value, err := s.core.Setting(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": r.PathValue("key"), "value": value})
}

func (s *server) handleSaveSetting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.SaveSetting(r.Context(), r.PathValue("key"), req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleTriggerScan runs a scan, either inline or detached, according to what
// the runtime guarantees.
//
// On a long-lived container the scan is detached and the caller gets 202: the
// work outlives the connection, so tying it to the request context would abort
// discovery the moment a proxy timed the caller out.
//
// On an autoscaled platform the scan is run inline and the caller gets 200
// with the outcome. Detaching there would be a lie: the instance is throttled
// once the response is written, so the goroutine would stall and the operator
// would hold a 202 for a scan that never happened.
func (s *server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	var req core.ScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Domain == "" && req.RepoURL == "" {
		writeError(w, http.StatusBadRequest, errors.New("one of 'domain' or 'repoUrl' is required"))
		return
	}

	if s.scanMode == scanInline {
		// The request context governs, so a client that gives up stops the
		// work rather than leaving it running against somebody's DNS.
		err := s.core.RunScan(r.Context(), req)
		s.publish(event.EventScanProgress, core.NewScanOutcome(req, err))

		if err != nil {
			// A partial scan is reported as such. The results that did
			// complete are already persisted, so this is not a failure to be
			// retried blindly — it is an outcome to be read.
			writeJSON(w, http.StatusOK, map[string]any{
				"status": core.ScanStatusPartial,
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": core.ScanStatusCompleted})
		return
	}

	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		err := s.core.RunScan(context.Background(), req)
		if err != nil {
			log.Printf("trawl: %v", err)
		}
		// The outcome travels with the event because this path has already
		// answered 202: the response cannot carry it, so the event is the
		// only place the caller can learn how the scan ended.
		s.publish(event.EventScanProgress, core.NewScanOutcome(req, err))
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
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

// handleWebSocket redirects to the Server-Sent Events stream.
//
// The live channel is /api/v1/events. This route is retained so that an older
// dashboard build fails with a message naming its replacement rather than with
// a bare connection error that looks like the server being down.
func (s *server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "websocket has been replaced by Server-Sent Events at /api/v1/events", http.StatusGone)
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
