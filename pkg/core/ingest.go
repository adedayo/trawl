package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/software"
	"github.com/adedayo/trawl/pkg/store"
)

// IngestKind names the worker payload being correlated.
type IngestKind string

const (
	// IngestDiscovery is subdomain enumeration output.
	IngestDiscovery IngestKind = "discovery"
	// IngestScan is the combined port, HTTP-probe and vulnerability output
	// posted by the scan worker. The worker posts each tool's results as they
	// finish, so one job run produces several payloads of this kind.
	IngestScan IngestKind = "scan"
	// IngestSecrets is repository secret-scan output.
	IngestSecrets IngestKind = "secrets"
)

// IngestSummary reports what correlating one payload changed.
//
// It distinguishes what was recorded from what was refused and what could not
// be read, because those three call for different responses: nothing, an
// authorisation decision, and a bug report respectively. A single "processed"
// count would collapse them.
type IngestSummary struct {
	Kind IngestKind `json:"kind"`
	// JobRunID ties the summary back to the run that produced the payload.
	JobRunID string `json:"jobRunId"`
	// Assets is the number of assets created or refreshed.
	Assets int `json:"assets"`
	// Findings is the number of findings recorded.
	Findings int `json:"findings"`
	// Refused names the targets the payload referred to that lie outside the
	// authorised scope. A worker should never have scanned them; recording the
	// refusal here is how that becomes visible rather than merely prevented.
	Refused []string `json:"refused,omitempty"`
	// Unreadable counts records that could not be parsed. They are counted
	// rather than dropped silently: a tool changing its output format presents
	// as a quiet estate, which is the failure this whole system exists to
	// avoid.
	Unreadable int `json:"unreadable,omitempty"`
}

// scanPayload is the envelope the scan worker posts. Each tool's results
// arrive in their own field, and a payload carries whichever finished.
type scanPayload struct {
	JobRunID string            `json:"jobRunId"`
	Naabu    []naabuRecord     `json:"naabu"`
	HTTPX    []httpxRecord     `json:"httpx"`
	Nuclei   []nucleiRecord    `json:"nuclei"`
	Hosts    []discoveryRecord `json:"hosts"`
}

type discoveryRecord struct {
	Host  string `json:"host"`
	Input string `json:"input"`
}

type naabuRecord struct {
	Host string `json:"host"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type httpxRecord struct {
	URL        string   `json:"url"`
	Input      string   `json:"input"`
	Host       string   `json:"host"`
	StatusCode int      `json:"status_code"`
	Title      string   `json:"title"`
	Tech       []string `json:"tech"`
	Scheme     string   `json:"scheme"`
	CDN        bool     `json:"cdn"`
}

type nucleiRecord struct {
	TemplateID string `json:"template-id"`
	Host       string `json:"host"`
	MatchedAt  string `json:"matched-at"`
	Type       string `json:"type"`
	Info       struct {
		Name        string   `json:"name"`
		Severity    string   `json:"severity"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Reference   []string `json:"reference"`
		Classif     struct {
			CVEID []string `json:"cve-id"`
		} `json:"classification"`
	} `json:"info"`
}

// CorrelateIngest turns a stored worker payload into assets and findings.
//
// The ingest endpoint's contract is that nothing observed is lost, so it
// writes the payload verbatim and returns. That is deliberately not the same
// job as this one: a payload that has been durably recorded but never
// interpreted is evidence nobody has read, and a dashboard over it shows an
// estate that appears to have nothing wrong with it.
//
// Scope is re-checked here even though the worker was given in-scope targets.
// That is the defence-in-depth guardrail rather than distrust of the worker:
// the targets file is assembled from data that may itself be wrong, and a
// finding written against an asset the operator never authorised is a claim
// about somebody else's estate.
func (c *Core) CorrelateIngest(ctx context.Context, kind IngestKind, raw []byte) (IngestSummary, error) {
	summary := IngestSummary{Kind: kind}

	var payload scanPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return summary, fmt.Errorf("core: unreadable %s payload: %w", kind, err)
	}
	summary.JobRunID = payload.JobRunID

	scope := c.Scope(ctx)
	if !scope.Authorised() {
		// Nothing is correlated under an unsigned scope. The payload stays on
		// disk, so authorising later and re-running loses nothing.
		return summary, fmt.Errorf("core: refusing to correlate %s results: the scope is not authorised", kind)
	}
	guard := newScopeFilter(scope)

	now := time.Now().UTC()

	// A scan payload is a run: the findings it records are stamped with it and
	// it is entered in the ledger. Discovery payloads add assets, not findings.
	runID := "run-" + uuid.NewString()
	if kind == IngestScan {
		ctx = store.WithRun(ctx, store.RunRef{ID: runID, Kind: store.RunIngestScan})
	}

	switch kind {
	case IngestDiscovery:
		c.correlateHosts(ctx, payload.Hosts, guard, now, &summary)
	case IngestScan:
		c.correlateHosts(ctx, payload.Hosts, guard, now, &summary)
		c.correlateNaabu(ctx, payload.Naabu, guard, now, &summary)
		c.correlateHTTPX(ctx, payload.HTTPX, guard, now, &summary)
		c.correlateNuclei(ctx, payload.Nuclei, guard, now, &summary)
	case IngestSecrets:
		// Secret findings arrive already typed and are written by the secret
		// scanner's own path. There is nothing to correlate here yet, and
		// pretending otherwise would report work that did not happen.
		return summary, nil
	default:
		return summary, fmt.Errorf("core: unknown ingest kind %q", kind)
	}

	summary.Refused = guard.refused()

	if kind == IngestScan {
		if err := c.store.RecordScanRun(ctx, &store.ScanRun{
			ID: runID, Kind: store.RunIngestScan, Outcome: "completed",
			StartedAt: now, FinishedAt: time.Now().UTC(),
		}); err != nil {
			return summary, fmt.Errorf("core: recording the scan in the run ledger: %w", err)
		}
	}

	if summary.Assets > 0 || summary.Findings > 0 {
		c.bus.Publish(ctx, event.Event{
			Type:      event.EventAssetUpdated,
			Timestamp: now,
			Payload:   summary,
		})
	}

	return summary, nil
}

func (c *Core) correlateHosts(ctx context.Context, recs []discoveryRecord, guard *scopeFilter, now time.Time, sum *IngestSummary) {
	for _, r := range recs {
		host := firstNonEmpty(r.Host, r.Input)
		if host == "" {
			sum.Unreadable++
			continue
		}
		if !guard.permits(host) {
			continue
		}
		if c.upsertAsset(ctx, host, "worker-discovery", now) {
			sum.Assets++
		}
	}
}

func (c *Core) correlateNaabu(ctx context.Context, recs []naabuRecord, guard *scopeFilter, now time.Time, sum *IngestSummary) {
	// Ports are grouped per host so that one asset carries its open ports as a
	// set rather than accumulating a finding per port. An operator reasons
	// about "what is exposed on this host", and a list of twelve near-identical
	// findings buries the one that matters.
	ports := map[string][]int{}
	for _, r := range recs {
		host := firstNonEmpty(r.Host, r.IP)
		if host == "" || r.Port == 0 {
			sum.Unreadable++
			continue
		}
		if !guard.permits(host) {
			continue
		}
		ports[host] = append(ports[host], r.Port)
	}

	for host, list := range ports {
		sort.Ints(list)
		if c.upsertAsset(ctx, host, "naabu", now) {
			sum.Assets++
		}
		asset, err := c.assetByValue(ctx, host)
		if err != nil || asset == nil {
			continue
		}

		// The open-port set is recorded as a posture attribute rather than as
		// a finding. An open port is not itself a defect; a port that was not
		// open last week is the thing worth raising, and that comparison is
		// what the regression path exists to make.
		if _, err := c.store.RecordPostureObservation(ctx, asset.ID, "open_ports", joinInts(list)); err != nil {
			continue
		}
	}
}

func (c *Core) correlateHTTPX(ctx context.Context, recs []httpxRecord, guard *scopeFilter, now time.Time, sum *IngestSummary) {
	for _, r := range recs {
		host := firstNonEmpty(r.Host, hostOf(r.URL), r.Input)
		if host == "" {
			sum.Unreadable++
			continue
		}
		if !guard.permits(host) {
			continue
		}

		if c.upsertAsset(ctx, host, "httpx", now) {
			sum.Assets++
		}
		asset, err := c.assetByValue(ctx, host)
		if err != nil || asset == nil {
			continue
		}

		meta, _ := json.Marshal(map[string]any{
			"url":        r.URL,
			"statusCode": r.StatusCode,
			"title":      r.Title,
			"tech":       r.Tech,
			"cdn":        r.CDN,
		})
		asset.Metadata = string(meta)
		asset.LastSeen = now
		_ = c.store.SaveAsset(ctx, asset)

		// A service reachable over plain HTTP is recorded as posture, so that
		// a site which was HTTPS-only last week and is not now raises a
		// regression rather than a finding indistinguishable from one that has
		// been that way for years.
		if scheme := schemeOf(r.URL, r.Scheme); scheme != "" {
			_, _ = c.store.RecordPostureObservation(ctx, asset.ID, "http_scheme", scheme)
		}

		c.recordHTTPXSoftware(ctx, asset.ID, r.Tech, now)
	}
}

func (c *Core) recordHTTPXSoftware(ctx context.Context, assetID string, tech []string, now time.Time) {
	if len(tech) == 0 {
		// No technology data in this scan: leave any open candidates open and
		// mark them not re-assessed rather than treating absence as fixed.
		_ = c.store.MarkAssetCandidatesNotReassessed(ctx, assetID, "Latest HTTP scan reported no technology data; software could not be re-assessed.")
		return
	}
	pm, err := software.LoadMap()
	if err != nil {
		return
	}
	for _, raw := range tech {
		for _, parsed := range software.ParseTech(raw) {
			sw := store.AssetSoftware{
				AssetID:      assetID,
				ObservedName: parsed.Name,
				Version:      parsed.Version,
				VersionState: store.SoftwareVersionUnknown,
				MappingState: store.SoftwareUnmapped,
				Source:       "httpx",
				RawString:    parsed.Raw,
				Status:       store.SoftwareCurrent,
				FirstSeen:    now,
				LastSeen:     now,
			}
			if parsed.VersionKnown {
				sw.VersionState = store.SoftwareVersionKnown
			}
			if id, ok := pm.Lookup(parsed.Name); ok {
				sw.CPEPart = id.CPEPart
				sw.Vendor = id.Vendor
				sw.Product = id.Product
				sw.MappingState = store.SoftwareMapped
				sw.MappingVersion = id.MappingVersion
			}
			_ = c.store.SaveAssetSoftware(ctx, &sw)
		}
	}
}

// knownFindingIDs returns the identifiers of an asset's findings, read once per
// asset per payload.
func (c *Core) knownFindingIDs(ctx context.Context, assetID string, cache map[string]map[string]bool) (map[string]bool, error) {
	if ids, ok := cache[assetID]; ok {
		return ids, nil
	}
	existing, err := c.store.GetFindings(ctx, assetID)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(existing))
	for _, f := range existing {
		ids[f.ID] = true
	}
	cache[assetID] = ids
	return ids, nil
}

func (c *Core) correlateNuclei(ctx context.Context, recs []nucleiRecord, guard *scopeFilter, now time.Time, sum *IngestSummary) {
	seen := map[string]map[string]bool{}
	for _, r := range recs {
		host := firstNonEmpty(hostOf(r.MatchedAt), hostOf(r.Host), r.Host)
		if host == "" || r.TemplateID == "" {
			sum.Unreadable++
			continue
		}
		if !guard.permits(host) {
			continue
		}

		if c.upsertAsset(ctx, host, "nuclei", now) {
			sum.Assets++
		}
		asset, err := c.assetByValue(ctx, host)
		if err != nil || asset == nil {
			continue
		}

		// The identity is what the finding asserts, not a fresh random value:
		// the same template matching the same place on the same asset is the
		// same finding on every scan. A random identifier here meant every scan
		// created a new row, so nothing could be compared and nothing could
		// ever be marked resolved.
		finding := store.Finding{
			ID:          store.ScanFindingID(asset.ID, r.TemplateID, firstNonEmpty(r.MatchedAt, r.Host)),
			AssetID:     asset.ID,
			Title:       firstNonEmpty(r.Info.Name, r.TemplateID),
			Description: r.Info.Description,
			Severity:    nucleiSeverity(r.Info.Severity),
			Category:    firstNonEmpty(r.Type, "network"),
			Proof:       firstNonEmpty(r.MatchedAt, r.Host),
			FirstSeen:   now,
			LastSeen:    now,
		}
		if len(r.Info.Classif.CVEID) > 0 {
			finding.CVE = strings.ToUpper(r.Info.Classif.CVEID[0])
		}

		// Priority is deliberately left unset. It is a deterministic function
		// of KEV and EPSS enrichment computed elsewhere, and a transport-side
		// guess would be a second, contradictory source for a number the whole
		// model requires to be reproducible.
		// Only a finding not seen before is announced as new. Nothing is
		// resolved here: a worker payload does not say which templates were
		// run, so a template's absence is not evidence that it was re-assessed.
		known, err := c.knownFindingIDs(ctx, asset.ID, seen)
		if err != nil {
			continue
		}
		isNew := !known[finding.ID]
		if err := c.store.SaveFinding(ctx, &finding); err == nil {
			sum.Findings++
			known[finding.ID] = true
			if isNew {
				c.bus.Publish(ctx, event.Event{
					Type:      event.EventFindingNew,
					Timestamp: now,
					Payload:   finding,
				})
			}
		}
	}
}

// upsertAsset records a host, reporting whether it was newly created.
func (c *Core) upsertAsset(ctx context.Context, value, source string, now time.Time) bool {
	existing, err := c.assetByValue(ctx, value)
	if err == nil && existing != nil {
		existing.LastSeen = now
		_ = c.store.SaveAsset(ctx, existing)
		return false
	}

	asset := store.Asset{
		ID:              uuid.NewString(),
		Type:            assetType(value),
		Value:           value,
		Status:          store.AssetStatus("active"),
		DiscoverySource: source,
		FirstSeen:       now,
		LastSeen:        now,
	}
	if err := c.store.SaveAsset(ctx, &asset); err != nil {
		return false
	}
	return true
}

func (c *Core) assetByValue(ctx context.Context, value string) (*store.Asset, error) {
	assets, err := c.store.GetAssets(ctx, "")
	if err != nil {
		return nil, err
	}
	for i := range assets {
		if strings.EqualFold(assets[i].Value, value) {
			return &assets[i], nil
		}
	}
	return nil, nil
}

// scopeFilter answers whether a host is authorised, and remembers the ones
// that were not.
//
// Refusals are collected rather than logged and forgotten. A worker returning
// out-of-scope results means either the target list was built wrongly or a
// tool followed a redirect off the estate, and both are worth knowing about;
// silently discarding the records would hide the fact that it happened.
type scopeFilter struct {
	domains []string
	cidrs   []*net.IPNet
	denied  map[string]bool
}

func newScopeFilter(s Scope) *scopeFilter {
	f := &scopeFilter{denied: map[string]bool{}}
	for _, d := range s.SeedDomainsList {
		if n := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d), ".")); n != "" {
			f.domains = append(f.domains, n)
		}
	}
	for _, c := range s.SeedCidrsList {
		if _, netw, err := net.ParseCIDR(strings.TrimSpace(c)); err == nil {
			f.cidrs = append(f.cidrs, netw)
		}
	}
	return f
}

func (f *scopeFilter) permits(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h == "" {
		return false
	}
	if hostOnly, _, err := net.SplitHostPort(h); err == nil {
		h = hostOnly
	}

	if ip := net.ParseIP(h); ip != nil {
		for _, n := range f.cidrs {
			if n.Contains(ip) {
				return true
			}
		}
		f.denied[h] = true
		return false
	}

	for _, d := range f.domains {
		if h == d || strings.HasSuffix(h, "."+d) {
			return true
		}
	}
	f.denied[h] = true
	return false
}

func (f *scopeFilter) refused() []string {
	if len(f.denied) == 0 {
		return nil
	}
	out := make([]string, 0, len(f.denied))
	for h := range f.denied {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// nucleiSeverity maps nuclei's severity vocabulary onto Trawl's.
//
// An unrecognised severity becomes info rather than being guessed upward or
// dropped. Inventing a rating from an unknown label would put a number in
// front of an operator that no rule produced.
func nucleiSeverity(s string) store.FindingSeverity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return store.SeverityCritical
	case "high":
		return store.SeverityHigh
	case "medium":
		return store.SeverityMedium
	case "low":
		return store.SeverityLow
	default:
		return store.SeverityInfo
	}
}

func assetType(value string) store.AssetType {
	if net.ParseIP(value) != nil {
		return store.AssetType("ip")
	}
	return store.AssetType("domain")
}

func hostOf(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return strings.ToLower(s)
}

func schemeOf(rawURL, declared string) string {
	if i := strings.Index(rawURL, "://"); i > 0 {
		return strings.ToLower(rawURL[:i])
	}
	return strings.ToLower(strings.TrimSpace(declared))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ",")
}
