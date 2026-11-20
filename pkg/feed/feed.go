// Package feed handles the external vulnerability catalogues. Feed traffic is
// deliberately separate from target assessment traffic: these are named
// non-target authorities, not discovered assets.
package feed

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Name string

const (
	KEV  Name = "cisa-kev"
	EPSS Name = "epss"
	NVD  Name = "nvd"

	// EPSSAPI is an endpoint key, not a feed: snapshots it produces are EPSS
	// snapshots. It names FIRST's per-CVE query API, which returns scores for
	// the CVEs asked about, where EPSS is the full daily file. When it is
	// configured, refreshing EPSS asks for the CVEs on stored findings and
	// nothing else; when only EPSS is configured (a mirror of the daily file),
	// the whole file is fetched as before.
	EPSSAPI Name = "epss-api"
)

// epssBatchSize is how many CVEs go in one request. The API takes them in the
// query string, whose length it limits; fifty keeps well inside it.
const epssBatchSize = 50

// DefaultEndpoints are the public, unauthenticated authorities for the three
// feeds. Callers may override them for mirrors, proxies or hermetic tests; the
// client still refuses any host not present in its configured map.
func DefaultEndpoints() map[Name]string {
	return map[Name]string{
		KEV:  "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json",
		EPSS: "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz",
		// Used in preference to the file above for refreshes: see EPSSAPI.
		EPSSAPI: "https://api.first.org/data/v1/epss",
		NVD:     "https://services.nvd.nist.gov/rest/json/cves/2.0",
	}
}

type Record struct {
	CVE       string
	KEVListed *bool
	EPSS      *float64
}

type Snapshot struct {
	ID            string
	Feed          Name
	SourceURL     string
	RetrievedAt   time.Time
	ContentDigest string
	RecordCount   int
	Records       map[string]Record

	// Queried is the set of CVEs a referenced-only fetch asked about, nil for a
	// feed fetched whole. A CVE absent from Records but present here was asked
	// about and is not in the feed (an answer). A CVE absent from both was
	// never asked about, and nothing is known: it must not be recorded as
	// "not found".
	Queried map[string]bool
}

type Client struct {
	client    *http.Client
	endpoints map[Name]*url.URL

	mu              sync.Mutex
	cache           map[Name]validators
	nvdAPIKey       string
	nvdRateInterval *time.Duration
}

// SetNVDAPIKey configures an optional NVD 2.0 API key sent in the apiKey
// header. With a key, NVD allows 50 requests per 30 seconds instead of 5.
func (c *Client) SetNVDAPIKey(key string) {
	c.mu.Lock()
	c.nvdAPIKey = strings.TrimSpace(key)
	c.mu.Unlock()
}

// SetNVDRateInterval overrides the minimum pause between consecutive NVD
// requests. Passing 0 disables pacing (intended for hermetic tests).
func (c *Client) SetNVDRateInterval(d time.Duration) {
	c.mu.Lock()
	c.nvdRateInterval = &d
	c.mu.Unlock()
}

func (c *Client) nvdAuthAndInterval() (string, time.Duration) {
	c.mu.Lock()
	key := c.nvdAPIKey
	override := c.nvdRateInterval
	c.mu.Unlock()
	if override != nil {
		return key, *override
	}
	if key != "" {
		return key, 650 * time.Millisecond
	}
	return "", 6 * time.Second
}

type validators struct {
	etag         string
	lastModified string
}

type FetchResult struct {
	Snapshot    *Snapshot
	NotModified bool
}

func NewClient(httpClient *http.Client, endpoints map[Name]string) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	parsed := make(map[Name]*url.URL, len(endpoints))
	for name, raw := range endpoints {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
			return nil, fmt.Errorf("feed %q has invalid endpoint %q", name, raw)
		}
		parsed[name] = u
	}
	return &Client{client: httpClient, endpoints: parsed, cache: make(map[Name]validators)}, nil
}

func (c *Client) Fetch(ctx context.Context, name Name) (*FetchResult, error) {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return nil, fmt.Errorf("feed %q is not a configured endpoint", name)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating %s request: %w", name, err)
	}
	c.mu.Lock()
	validator := c.cache[name]
	c.mu.Unlock()
	if validator.etag != "" {
		req.Header.Set("If-None-Match", validator.etag)
	}
	if validator.lastModified != "" {
		req.Header.Set("If-Modified-Since", validator.lastModified)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{NotModified: true}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: endpoint returned %s", name, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", name, err)
	}
	snapshot, err := Parse(name, endpoint.String(), time.Now().UTC(), body)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[name] = validators{etag: resp.Header.Get("ETag"), lastModified: resp.Header.Get("Last-Modified")}
	c.mu.Unlock()
	return &FetchResult{Snapshot: snapshot}, nil
}

// FetchNVDReferenced queries NVD 2.0 only for the distinct CVEs already named
// by stored findings, rather than mirroring the full NVD catalogue.
//
// If any single CVE lookup fails, the entire fetch fails and returns no
// snapshot: storing a partial snapshot would record an unqueried or failed CVE
// as absent when the snapshot is applied.
func (c *Client) FetchNVDReferenced(ctx context.Context, cves []string) (*FetchResult, error) {
	endpoint, ok := c.endpoints[NVD]
	if !ok {
		return nil, fmt.Errorf("feed %q is not a configured endpoint", NVD)
	}

	seen := make(map[string]bool, len(cves))
	ordered := make([]string, 0, len(cves))
	for _, raw := range cves {
		cve := normaliseCVE(raw)
		if cve == "" || seen[cve] {
			continue
		}
		seen[cve] = true
		ordered = append(ordered, cve)
	}
	sort.Strings(ordered)

	records := make(map[string]Record, len(ordered))
	hasher := sha256.New()
	for _, cve := range ordered {
		target := *endpoint
		q := target.Query()
		q.Set("cveId", cve)
		target.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("creating nvd request for %s: %w", cve, err)
		}
		if key, _ := c.nvdAuthAndInterval(); key != "" {
			req.Header.Set("apiKey", key)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching nvd %s: %w", cve, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading nvd %s response: %w", cve, readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetching nvd %s: endpoint returned %s", cve, resp.Status)
		}
		parsed, parseErr := parseNVD(body)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing nvd %s: %w", cve, parseErr)
		}
		for k, v := range parsed {
			records[k] = v
		}
		_, _ = hasher.Write([]byte(cve))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(body)
		_, _ = hasher.Write([]byte{'\n'})
	}

	retrievedAt := time.Now().UTC()
	if len(ordered) == 0 {
		_, _ = hasher.Write([]byte("empty:" + retrievedAt.Format(time.RFC3339Nano)))
	}
	digestText := hex.EncodeToString(hasher.Sum(nil))
	return &FetchResult{
		Snapshot: &Snapshot{
			ID:            string(NVD) + "-" + digestText[:16],
			Feed:          NVD,
			SourceURL:     endpoint.String(),
			RetrievedAt:   retrievedAt,
			ContentDigest: "sha256:" + digestText,
			RecordCount:   len(records),
			Records:       records,
			Queried:       seen,
		},
	}, nil
}

// Has reports whether an endpoint is configured under the given key.
func (c *Client) Has(name Name) bool {
	_, ok := c.endpoints[name]
	return ok
}

// FetchEPSSReferenced asks FIRST's API for the EPSS scores of the given CVEs
// only, rather than downloading the daily file of every scored CVE.
//
// As with FetchNVDReferenced, one failed batch fails the whole fetch and
// returns no snapshot: a snapshot built from the batches that happened to
// succeed would record the CVEs of the failed ones as unscored.
//
// A CVE the API does not return has no EPSS score (new CVEs often have none
// yet). That is an answer, so it is recorded as not found, and the snapshot
// remembers which CVEs were asked so that a CVE nobody asked about is not.
func (c *Client) FetchEPSSReferenced(ctx context.Context, cves []string) (*FetchResult, error) {
	endpoint, ok := c.endpoints[EPSSAPI]
	if !ok {
		return nil, fmt.Errorf("feed %q is not a configured endpoint", EPSSAPI)
	}

	seen := make(map[string]bool, len(cves))
	ordered := make([]string, 0, len(cves))
	for _, raw := range cves {
		cve := normaliseCVE(raw)
		if cve == "" || seen[cve] {
			continue
		}
		seen[cve] = true
		ordered = append(ordered, cve)
	}
	sort.Strings(ordered)

	records := make(map[string]Record, len(ordered))
	dates := make(map[string]string, len(ordered))
	for start := 0; start < len(ordered); start += epssBatchSize {
		end := start + epssBatchSize
		if end > len(ordered) {
			end = len(ordered)
		}
		batch := ordered[start:end]

		target := *endpoint
		q := target.Query()
		q.Set("cve", strings.Join(batch, ","))
		q.Set("limit", strconv.Itoa(len(batch)))
		target.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("creating epss request: %w", err)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching epss scores: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading epss response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetching epss scores: endpoint returned %s", resp.Status)
		}
		parsed, parseErr := parseEPSSAPI(body, seen)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing epss response: %w", parseErr)
		}
		for cve, row := range parsed {
			records[cve] = row.record
			dates[cve] = row.date
		}
	}

	// The digest covers what was learned, not the bytes of the responses, so
	// the same scores for the same CVEs are the same snapshot however the API
	// happens to order or space its reply. With nothing to look up it is the
	// digest of nothing, a single stable empty snapshot rather than a new one
	// per refresh.
	hasher := sha256.New()
	for _, cve := range ordered {
		_, _ = hasher.Write([]byte(cve + "\x00"))
		if row, found := records[cve]; found {
			_, _ = hasher.Write([]byte(strconv.FormatFloat(*row.EPSS, 'f', -1, 64) + "\x00" + dates[cve]))
		} else {
			_, _ = hasher.Write([]byte("unscored"))
		}
		_, _ = hasher.Write([]byte{'\n'})
	}
	digestText := hex.EncodeToString(hasher.Sum(nil))
	return &FetchResult{
		Snapshot: &Snapshot{
			ID:            string(EPSS) + "-" + digestText[:16],
			Feed:          EPSS,
			SourceURL:     endpoint.String(),
			RetrievedAt:   time.Now().UTC(),
			ContentDigest: "sha256:" + digestText,
			RecordCount:   len(records),
			Records:       records,
			Queried:       seen,
		},
	}, nil
}

type epssAPIRow struct {
	record Record
	date   string
}

// parseEPSSAPI reads one page of FIRST's EPSS API. Rows for CVEs that were not
// asked about are ignored rather than trusted, so the snapshot cannot claim a
// score for something outside the question.
func parseEPSSAPI(body []byte, asked map[string]bool) (map[string]epssAPIRow, error) {
	var document struct {
		Status string `json:"status"`
		Data   []struct {
			CVE  string `json:"cve"`
			EPSS string `json:"epss"`
			Date string `json:"date"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.Status != "" && !strings.EqualFold(document.Status, "OK") {
		return nil, fmt.Errorf("API status %q", document.Status)
	}
	rows := make(map[string]epssAPIRow, len(document.Data))
	for _, item := range document.Data {
		cve := normaliseCVE(item.CVE)
		if cve == "" || !asked[cve] {
			continue
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(item.EPSS), 64)
		if err != nil || score < 0 || score > 1 {
			return nil, fmt.Errorf("invalid EPSS score for %s", cve)
		}
		rows[cve] = epssAPIRow{record: Record{CVE: cve, EPSS: &score}, date: item.Date}
	}
	return rows, nil
}

func LoadFile(name Name, path string) (*Snapshot, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s feed file: %w", name, err)
	}
	return Parse(name, "file://"+path, time.Now().UTC(), body)
}

func Parse(name Name, sourceURL string, retrievedAt time.Time, body []byte) (*Snapshot, error) {
	var records map[string]Record
	var err error
	switch name {
	case KEV:
		records, err = parseKEV(body)
	case EPSS:
		records, err = parseEPSS(body)
	case NVD:
		records, err = parseNVD(body)
	default:
		return nil, fmt.Errorf("unsupported feed %q", name)
	}
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	digest := sha256.Sum256(body)
	digestText := hex.EncodeToString(digest[:])
	return &Snapshot{
		ID:            string(name) + "-" + digestText[:16],
		Feed:          name,
		SourceURL:     sourceURL,
		RetrievedAt:   retrievedAt,
		ContentDigest: "sha256:" + digestText,
		RecordCount:   len(records),
		Records:       records,
	}, nil
}

type kevDocument struct {
	Vulnerabilities []struct {
		CVEID string `json:"cveID"`
	} `json:"vulnerabilities"`
}

func parseKEV(body []byte) (map[string]Record, error) {
	var document kevDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	records := make(map[string]Record, len(document.Vulnerabilities))
	for _, item := range document.Vulnerabilities {
		cve := normaliseCVE(item.CVEID)
		if cve == "" {
			continue
		}
		listed := true
		records[cve] = Record{CVE: cve, KEVListed: &listed}
	}
	return records, nil
}

func parseEPSS(body []byte) (map[string]Record, error) {
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("decompressing gzipped EPSS feed: %w", err)
		}
		decompressed, err := io.ReadAll(gz)
		_ = gz.Close()
		if err != nil {
			return nil, fmt.Errorf("reading gzipped EPSS feed: %w", err)
		}
		body = decompressed
	}
	reader := csv.NewReader(strings.NewReader(string(body)))
	reader.Comment = '#'
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	cveColumn, scoreColumn := -1, -1
	for i, column := range header {
		switch strings.ToLower(strings.TrimSpace(column)) {
		case "cve":
			cveColumn = i
		case "epss":
			scoreColumn = i
		}
	}
	if cveColumn < 0 || scoreColumn < 0 {
		return nil, errors.New("CSV must contain cve and epss columns")
	}
	records := map[string]Record{}
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if len(row) <= cveColumn || len(row) <= scoreColumn {
			return nil, errors.New("CSV row is shorter than its header")
		}
		cve := normaliseCVE(row[cveColumn])
		score, parseErr := strconv.ParseFloat(strings.TrimSpace(row[scoreColumn]), 64)
		if cve == "" || parseErr != nil || score < 0 || score > 1 {
			return nil, fmt.Errorf("invalid EPSS row for %q", cve)
		}
		records[cve] = Record{CVE: cve, EPSS: &score}
	}
	return records, nil
}

func parseNVD(body []byte) (map[string]Record, error) {
	var document struct {
		Vulnerabilities []struct {
			CVE struct {
				ID string `json:"id"`
			} `json:"cve"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	records := make(map[string]Record, len(document.Vulnerabilities))
	for _, item := range document.Vulnerabilities {
		cve := normaliseCVE(item.CVE.ID)
		if cve != "" {
			records[cve] = Record{CVE: cve}
		}
	}
	return records, nil
}

func normaliseCVE(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "CVE-") {
		return ""
	}
	return value
}
