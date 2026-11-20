package feed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

// nvdPageSize is NVD 2.0's maximum resultsPerPage. Almost every single
// product+version lookup fits inside one page.
const nvdPageSize = 2000

// SoftwareQuery names one mapped product and version to look up in NVD.
// It deliberately carries no asset name, address, or identifier so no
// estate-specific data can ever leave the machine in a lookup request.
type SoftwareQuery struct {
	CPEPart string
	Vendor  string
	Product string
	Version string
}

// Key returns the canonical cache key for this query.
func (q SoftwareQuery) Key() string {
	return store.SoftwareLookupKey(q.CPEPart, q.Vendor, q.Product, q.Version)
}

// VirtualMatchString returns the CPE 2.3 virtualMatchString sent to NVD.
func (q SoftwareQuery) VirtualMatchString() string {
	part := strings.TrimSpace(q.CPEPart)
	if part == "" {
		part = "a"
	}
	return fmt.Sprintf("cpe:2.3:%s:%s:%s:%s", part, strings.TrimSpace(q.Vendor), strings.TrimSpace(q.Product), strings.TrimSpace(q.Version))
}

// SoftwareLookupBundle is the atomic outcome of querying NVD for a batch of
// distinct (cpePart, vendor, product, version) tuples.
type SoftwareLookupBundle struct {
	ByKey    map[string][]store.CandidateCVE
	Snapshot *Snapshot
}

type nvdSoftwareResponse struct {
	ResultsPerPage  int `json:"resultsPerPage"`
	StartIndex      int `json:"startIndex"`
	TotalResults    int `json:"totalResults"`
	Vulnerabilities []struct {
		CVE struct {
			ID           string `json:"id"`
			Published    string `json:"published"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics struct {
				V31 []nvdMetric `json:"cvssMetricV31"`
				V40 []nvdMetric `json:"cvssMetricV40"`
				V30 []nvdMetric `json:"cvssMetricV30"`
				V2  []nvdMetric `json:"cvssMetricV2"`
			} `json:"metrics"`
			Configurations []struct {
				Nodes []struct {
					CPEMatch []struct {
						Vulnerable bool   `json:"vulnerable"`
						Criteria   string `json:"criteria"`
					} `json:"cpeMatch"`
				} `json:"nodes"`
			} `json:"configurations"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdMetric struct {
	Type         string `json:"type"`
	BaseSeverity string `json:"baseSeverity"` // CVSS v2 places baseSeverity here
	CVSSData     struct {
		BaseSeverity string `json:"baseSeverity"` // CVSS v3/v4 places baseSeverity here
	} `json:"cvssData"`
}

// FetchNVDBySoftware queries NVD 2.0 for each distinct mapped product and
// version, respecting NVD's rate limit and optional API key.
//
// Fail-closed: if any single product or page fails, the entire lookup fails
// and returns no bundle or snapshot, so callers never persist a partial answer.
func (c *Client) FetchNVDBySoftware(ctx context.Context, queries []SoftwareQuery) (*SoftwareLookupBundle, error) {
	endpoint, ok := c.endpoints[NVD]
	if !ok {
		return nil, fmt.Errorf("feed %q is not a configured endpoint", NVD)
	}

	dedup := make(map[string]SoftwareQuery, len(queries))
	orderedKeys := make([]string, 0, len(queries))
	for _, q := range queries {
		if strings.TrimSpace(q.Vendor) == "" || strings.TrimSpace(q.Product) == "" || strings.TrimSpace(q.Version) == "" {
			continue
		}
		if q.CPEPart == "" {
			q.CPEPart = "a"
		}
		k := q.Key()
		if _, exists := dedup[k]; exists {
			continue
		}
		dedup[k] = q
		orderedKeys = append(orderedKeys, k)
	}
	sort.Strings(orderedKeys)

	apiKey, interval := c.nvdAuthAndInterval()
	byKey := make(map[string][]store.CandidateCVE, len(orderedKeys))
	records := make(map[string]Record)
	queried := make(map[string]bool)
	hasher := sha256.New()

	requestCount := 0
	for _, key := range orderedKeys {
		q := dedup[key]
		vms := q.VirtualMatchString()
		seenCVE := make(map[string]bool)
		var cves []store.CandidateCVE

		startIndex := 0
		for {
			if requestCount > 0 && interval > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(interval):
				}
			}
			requestCount++

			target := *endpoint
			params := target.Query()
			params.Set("virtualMatchString", vms)
			params.Set("resultsPerPage", strconv.Itoa(nvdPageSize))
			if startIndex > 0 {
				params.Set("startIndex", strconv.Itoa(startIndex))
			}
			target.RawQuery = params.Encode()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("creating nvd software request for %s: %w", key, err)
			}
			if apiKey != "" {
				req.Header.Set("apiKey", apiKey)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fetching nvd software %s: %w", key, err)
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("reading nvd software %s response: %w", key, readErr)
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("fetching nvd software %s: endpoint returned %s", key, resp.Status)
			}

			var page nvdSoftwareResponse
			if err := json.Unmarshal(body, &page); err != nil {
				return nil, fmt.Errorf("parsing nvd software %s: %w", key, err)
			}

			_, _ = hasher.Write([]byte(key))
			_, _ = hasher.Write([]byte{0})
			_, _ = hasher.Write(body)
			_, _ = hasher.Write([]byte{'\n'})

			productNeedle := ":" + strings.ToLower(q.Vendor) + ":" + strings.ToLower(q.Product) + ":"
			for _, v := range page.Vulnerabilities {
				cveID := normaliseCVE(v.CVE.ID)
				if cveID == "" || seenCVE[cveID] {
					continue
				}
				if !isProductVulnerable(v.CVE.Configurations, productNeedle) {
					continue
				}
				seenCVE[cveID] = true

				desc := ""
				for _, d := range v.CVE.Descriptions {
					if strings.EqualFold(d.Lang, "en") && strings.TrimSpace(d.Value) != "" {
						desc = strings.TrimSpace(d.Value)
						break
					}
				}
				sev := extractNVDSeverity(v.CVE.Metrics.V31, v.CVE.Metrics.V40, v.CVE.Metrics.V30, v.CVE.Metrics.V2)
				cves = append(cves, store.CandidateCVE{
					CVE:         cveID,
					Severity:    sev,
					Description: desc,
					Published:   v.CVE.Published,
				})
				records[cveID] = Record{CVE: cveID}
				queried[cveID] = true
			}

			nextIndex := page.StartIndex + len(page.Vulnerabilities)
			if len(page.Vulnerabilities) == 0 || nextIndex >= page.TotalResults {
				break
			}
			startIndex = nextIndex
		}

		sort.Slice(cves, func(i, j int) bool { return cves[i].CVE < cves[j].CVE })
		byKey[key] = cves
	}

	retrievedAt := time.Now().UTC()
	if len(orderedKeys) == 0 {
		_, _ = hasher.Write([]byte("empty-software:" + retrievedAt.Format(time.RFC3339Nano)))
	}
	digestText := hex.EncodeToString(hasher.Sum(nil))
	snap := &Snapshot{
		ID:            string(NVD) + "-" + digestText[:16],
		Feed:          NVD,
		SourceURL:     endpoint.String(),
		RetrievedAt:   retrievedAt,
		ContentDigest: "sha256:" + digestText,
		RecordCount:   len(records),
		Records:       records,
		Queried:       queried,
	}

	return &SoftwareLookupBundle{
		ByKey:    byKey,
		Snapshot: snap,
	}, nil
}

// isProductVulnerable checks NVD's configurations block when present: if any
// cpeMatch explicitly names vendor:product, at least one such match must have
// vulnerable=true. If configurations is absent (e.g. a CVE awaiting full CPE
// analysis), we keep the CVE rather than silently dropping it.
func isProductVulnerable(configs []struct {
	Nodes []struct {
		CPEMatch []struct {
			Vulnerable bool   `json:"vulnerable"`
			Criteria   string `json:"criteria"`
		} `json:"cpeMatch"`
	} `json:"nodes"`
}, productNeedle string) bool {
	if len(configs) == 0 {
		return true
	}
	matchedProduct := false
	for _, cfg := range configs {
		for _, node := range cfg.Nodes {
			for _, m := range node.CPEMatch {
				if strings.Contains(strings.ToLower(m.Criteria), productNeedle) {
					matchedProduct = true
					if m.Vulnerable {
						return true
					}
				}
			}
		}
	}
	return !matchedProduct
}

func extractNVDSeverity(metricSets ...[]nvdMetric) store.FindingSeverity {
	for _, list := range metricSets {
		if len(list) == 0 {
			continue
		}
		// Prefer Primary NVD metric when available.
		for _, m := range list {
			if strings.EqualFold(m.Type, "Primary") {
				if sev := parseNVDSeverity(firstNonEmptyStr(m.CVSSData.BaseSeverity, m.BaseSeverity)); sev != "" {
					return sev
				}
			}
		}
		for _, m := range list {
			if sev := parseNVDSeverity(firstNonEmptyStr(m.CVSSData.BaseSeverity, m.BaseSeverity)); sev != "" {
				return sev
			}
		}
	}
	return store.SeverityInfo
}

func parseNVDSeverity(raw string) store.FindingSeverity {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "CRITICAL":
		return store.SeverityCritical
	case "HIGH":
		return store.SeverityHigh
	case "MEDIUM":
		return store.SeverityMedium
	case "LOW":
		return store.SeverityLow
	default:
		return ""
	}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
