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
)

// DefaultEndpoints are the public, unauthenticated authorities for the three
// feeds. Callers may override them for mirrors, proxies or hermetic tests; the
// client still refuses any host not present in its configured map.
func DefaultEndpoints() map[Name]string {
	return map[Name]string{
		KEV:  "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json",
		EPSS: "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz",
		NVD:  "https://services.nvd.nist.gov/rest/json/cves/2.0",
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
}

type Client struct {
	client    *http.Client
	endpoints map[Name]*url.URL

	mu    sync.Mutex
	cache map[Name]validators
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
		},
	}, nil
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
