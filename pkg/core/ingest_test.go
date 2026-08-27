package core

import (
	"context"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

func authorisedCore(t *testing.T, domains []string, cidrs []string) *Core {
	t.Helper()
	c := newTestCore(t)
	if err := c.SaveScope(context.Background(), Scope{
		SeedDomainsList: domains,
		SeedCidrsList:   cidrs,
		IsAuthorized:    true,
		SignerName:      "Test Operator",
	}); err != nil {
		t.Fatalf("SaveScope: %v", err)
	}
	return c
}

// TestCorrelationTurnsPayloadsIntoAssetsAndFindings is the gap this change
// closed. Before it, worker payloads were stored verbatim and nothing read
// them: the ingest endpoint reported success, the database grew, and the
// dashboard showed an estate with nothing wrong with it.
func TestCorrelationTurnsPayloadsIntoAssetsAndFindings(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	payload := []byte(`{
	  "jobRunId": "run-1",
	  "naabu": [
	    {"host": "api.example.com", "ip": "203.0.113.10", "port": 443},
	    {"host": "api.example.com", "ip": "203.0.113.10", "port": 22}
	  ],
	  "httpx": [
	    {"url": "https://api.example.com/", "host": "api.example.com",
	     "status_code": 200, "title": "API", "tech": ["nginx"]}
	  ],
	  "nuclei": [
	    {"template-id": "exposed-panel", "matched-at": "https://api.example.com/admin",
	     "type": "http",
	     "info": {"name": "Exposed admin panel", "severity": "high",
	              "description": "An administrative interface is reachable.",
	              "classification": {"cve-id": ["cve-2026-0001"]}}}
	  ]
	}`)

	summary, err := c.CorrelateIngest(ctx, IngestScan, payload)
	if err != nil {
		t.Fatalf("CorrelateIngest: %v", err)
	}

	if summary.JobRunID != "run-1" {
		t.Errorf("summary lost the job run id: %q", summary.JobRunID)
	}
	if summary.Assets != 1 {
		t.Errorf("recorded %d assets, want 1 — the same host seen by three tools "+
			"is one asset", summary.Assets)
	}
	if summary.Findings != 1 {
		t.Errorf("recorded %d findings, want 1", summary.Findings)
	}

	assets, err := c.Assets(ctx, "")
	if err != nil {
		t.Fatalf("Assets: %v", err)
	}
	if len(assets) != 1 || assets[0].Value != "api.example.com" {
		t.Fatalf("assets are %+v", assets)
	}

	findings, err := c.Findings(ctx, assets[0].ID)
	if err != nil {
		t.Fatalf("Findings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != store.SeverityHigh {
		t.Errorf("severity is %q, want %q", f.Severity, store.SeverityHigh)
	}
	if f.CVE != "CVE-2026-0001" {
		t.Errorf("CVE is %q; identifiers must be normalised or correlation against "+
			"KEV and EPSS will miss", f.CVE)
	}
	// Priority is computed deterministically downstream from KEV and EPSS. A
	// value set here would be a second source for a number the model requires
	// to be reproducible from its inputs alone.
	if f.Priority != "" {
		t.Errorf("correlation set a priority (%q); ranking is not the transport's "+
			"to decide", f.Priority)
	}
}

// TestCorrelationRefusesOutOfScopeResults is the defence-in-depth guardrail at
// the last point it can be applied.
//
// The worker was given in-scope targets, so these results should not exist.
// That is exactly why the check is here: the target list is assembled from
// data that may be wrong, and a tool can follow a redirect off the estate. A
// finding written against a host the operator never authorised is a claim
// about somebody else's infrastructure.
func TestCorrelationRefusesOutOfScopeResults(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	payload := []byte(`{
	  "jobRunId": "run-2",
	  "httpx": [{"url": "https://api.example.com/", "host": "api.example.com"}],
	  "nuclei": [
	    {"template-id": "leak", "matched-at": "https://someone-else.test/x",
	     "info": {"name": "Leak", "severity": "critical"}}
	  ]
	}`)

	summary, err := c.CorrelateIngest(ctx, IngestScan, payload)
	if err != nil {
		t.Fatalf("CorrelateIngest: %v", err)
	}

	if summary.Findings != 0 {
		t.Error("an out-of-scope finding was recorded")
	}

	assets, _ := c.Assets(ctx, "")
	for _, a := range assets {
		if a.Value == "someone-else.test" {
			t.Fatal("an out-of-scope host entered the inventory")
		}
	}

	// The refusal must be reported, not merely performed. A worker returning
	// out-of-scope results means something upstream is wrong, and silently
	// discarding the records hides that it happened.
	if len(summary.Refused) == 0 {
		t.Fatal("the refusal was silent; an operator cannot investigate what they " +
			"are not told about")
	}
	if summary.Refused[0] != "someone-else.test" {
		t.Errorf("refused list is %v, want it to name someone-else.test", summary.Refused)
	}
}

// TestCorrelationRefusesEverythingUnderAnUnsignedScope asserts the
// authorisation record gates correlation as well as scanning.
func TestCorrelationRefusesEverythingUnderAnUnsignedScope(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t) // no scope saved at all

	_, err := c.CorrelateIngest(ctx, IngestScan, []byte(
		`{"jobRunId":"run-3","httpx":[{"host":"api.example.com"}]}`))
	if err == nil {
		t.Fatal("results were correlated under an unauthorised scope")
	}

	assets, _ := c.Assets(ctx, "")
	if len(assets) != 0 {
		t.Errorf("an unauthorised correlation still wrote %d assets", len(assets))
	}
}

// TestCorrelationCountsUnreadableRecords asserts a record that cannot be
// interpreted is counted rather than dropped.
//
// A tool changing its output format otherwise presents as a quiet estate,
// which is indistinguishable from a clean one and is the precise failure this
// system exists to prevent.
func TestCorrelationCountsUnreadableRecords(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	payload := []byte(`{
	  "jobRunId": "run-4",
	  "naabu": [{"host": "", "ip": "", "port": 0}],
	  "nuclei": [{"template-id": "", "info": {"name": "nameless"}}]
	}`)

	summary, err := c.CorrelateIngest(ctx, IngestScan, payload)
	if err != nil {
		t.Fatalf("CorrelateIngest: %v", err)
	}
	if summary.Unreadable != 2 {
		t.Errorf("counted %d unreadable records, want 2", summary.Unreadable)
	}
	if summary.Assets != 0 || summary.Findings != 0 {
		t.Error("unreadable records produced inventory entries")
	}
}

// TestOpenPortsAreRecordedAsPostureNotFindings asserts the correlation records
// exposure as state to be compared, not as a defect.
//
// An open port is not itself a fault — plenty are meant to be open. The thing
// worth raising is a port that was not open last week, and that comparison is
// only possible if the port set is stored as a posture attribute rather than
// emitted as a finding per port.
func TestOpenPortsAreRecordedAsPostureNotFindings(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	first := []byte(`{"jobRunId":"run-5","naabu":[
	  {"host":"web.example.com","port":443},
	  {"host":"web.example.com","port":80}]}`)

	summary, err := c.CorrelateIngest(ctx, IngestScan, first)
	if err != nil {
		t.Fatalf("CorrelateIngest: %v", err)
	}
	if summary.Findings != 0 {
		t.Errorf("open ports produced %d findings; exposure is state, not a defect",
			summary.Findings)
	}
	if summary.Assets != 1 {
		t.Fatalf("recorded %d assets, want 1", summary.Assets)
	}

	// A second observation of the same host must refresh it rather than
	// duplicate it: the same estate scanned twice is not twice the estate.
	again, err := c.CorrelateIngest(ctx, IngestScan, first)
	if err != nil {
		t.Fatalf("second CorrelateIngest: %v", err)
	}
	if again.Assets != 0 {
		t.Errorf("re-scanning created %d new assets", again.Assets)
	}
	assets, _ := c.Assets(ctx, "")
	if len(assets) != 1 {
		t.Errorf("inventory holds %d assets after two identical scans", len(assets))
	}
}

// TestCorrelationAdmitsInScopeAddresses covers the CIDR half of the scope,
// which the domain-suffix path cannot answer.
func TestCorrelationAdmitsInScopeAddresses(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, []string{"203.0.113.0/24"})

	summary, err := c.CorrelateIngest(ctx, IngestScan, []byte(
		`{"jobRunId":"run-6","naabu":[
		   {"host":"203.0.113.10","port":443},
		   {"host":"198.51.100.7","port":443}]}`))
	if err != nil {
		t.Fatalf("CorrelateIngest: %v", err)
	}

	if summary.Assets != 1 {
		t.Errorf("recorded %d assets, want 1 — only the in-range address", summary.Assets)
	}
	assets, _ := c.Assets(ctx, "")
	for _, a := range assets {
		if a.Value == "198.51.100.7" {
			t.Fatal("an address outside the authorised range entered the inventory")
		}
		if a.Value == "203.0.113.10" && a.Type != store.AssetType("ip") {
			t.Errorf("address recorded as type %q, want ip", a.Type)
		}
	}
}
