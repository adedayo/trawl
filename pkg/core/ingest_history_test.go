package core

import (
	"context"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

const nucleiPayload = `{
  "jobRunId": "job-1",
  "nuclei": [
    {"template-id": "exposed-panel", "matched-at": "https://api.example.com/admin", "type": "http",
     "info": {"name": "Exposed admin panel", "severity": "high", "description": "d"}}
  ]
}`

// Re-running an unchanged scan must not create a new finding. Each scan used to
// give its findings a fresh random identifier, so nothing could be compared
// across runs and nothing could ever be resolved.
func TestIngestingTheSameScanTwiceYieldsOneFindingAndOneFirstSeenEvent(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	for i := 0; i < 2; i++ {
		if _, err := c.CorrelateIngest(ctx, IngestScan, []byte(nucleiPayload)); err != nil {
			t.Fatalf("scan %d: %v", i+1, err)
		}
	}

	findings, err := c.Findings(ctx, "")
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %d (%v), want 1", len(findings), err)
	}
	events, err := c.Store().GetFindingEvents(ctx, store.EventQuery{
		Key: store.FindingKey(store.SourceFinding, findings[0].ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != store.EventFirstSeen {
		t.Fatalf("events = %+v, want a single first_seen", events)
	}
	if events[0].RunID == "" {
		t.Error("the event does not say which run observed it")
	}

	runs, err := c.ScanRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want both scans in the ledger", len(runs))
	}
	if runs[0].ID == runs[1].ID {
		t.Error("runs must be distinct")
	}
}

func TestAChangedSeverityOnTheSameFindingIsAnEventNotANewFinding(t *testing.T) {
	ctx := context.Background()
	c := authorisedCore(t, []string{"example.com"}, nil)

	if _, err := c.CorrelateIngest(ctx, IngestScan, []byte(nucleiPayload)); err != nil {
		t.Fatal(err)
	}
	raised := []byte(`{"nuclei":[{"template-id":"exposed-panel","matched-at":"https://api.example.com/admin","type":"http",
	  "info":{"name":"Exposed admin panel","severity":"critical","description":"d"}}]}`)
	if _, err := c.CorrelateIngest(ctx, IngestScan, raised); err != nil {
		t.Fatal(err)
	}

	findings, _ := c.Findings(ctx, "")
	if len(findings) != 1 || findings[0].Severity != store.SeverityCritical {
		t.Fatalf("findings = %+v, want one, critical", findings)
	}
	events, _ := c.Store().GetFindingEvents(ctx, store.EventQuery{})
	if len(events) != 2 || events[1].Type != store.EventChanged ||
		events[1].PrevSeverity != store.SeverityHigh || events[1].Severity != store.SeverityCritical {
		t.Fatalf("events = %+v", events)
	}

	changes, err := c.FindingChanges(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Worsened) != 1 || len(changes.New) != 0 {
		t.Errorf("changes = %+v, want one worsened and none new", changes)
	}
}
