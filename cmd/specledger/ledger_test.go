package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoxesCountsTickedAndUntickedSeparately(t *testing.T) {
	const ledger = `# Tasks

## Phase 0

- [x] Landed
- [X] Also landed, ticked with a capital
- [ ] Not yet
      - [ ] Indented continuation counts too
- Not a checklist item at all
  - [x] Nested and ticked
`
	open, done := boxes(ledger)
	if open != 2 {
		t.Errorf("open = %d, want 2", open)
	}
	if done != 3 {
		t.Errorf("done = %d, want 3", done)
	}
}

// A ticked box may record that work landed or that a decision not to do it was
// taken. Both are conclusions, and neither is outstanding work — so a ledger
// full of withdrawals is closed, not busy.
func TestBoxesTreatsAWithdrawalAsClosed(t *testing.T) {
	open, done := boxes("- [x] Withdrawn: the three scanner runners were not deferred\n")
	if open != 0 || done != 1 {
		t.Errorf("open, done = %d, %d; want 0, 1", open, done)
	}
}

func TestAnArchivedChangeWithOpenItemsIsReported(t *testing.T) {
	problems := validate([]ledger{{ID: "003-go-sqlite-engine", Archived: true, Open: 2, Done: 9}})
	if len(problems) != 1 || !strings.Contains(problems[0], "archived with 2 open") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestAClosedLedgerLeftInChangesIsReported(t *testing.T) {
	problems := validate([]ledger{{ID: "019-vulnerability-feed-ingestion", Open: 0, Done: 12}})
	if len(problems) != 1 || !strings.Contains(problems[0], "not archived") {
		t.Fatalf("problems = %v", problems)
	}
}

// 001 holds the live capability specs, so its ledger closing does not make it
// archivable. The exemption is data rather than a special case in the check,
// and it carries its reason.
func TestTheExemptChangeMayHaveAClosedLedger(t *testing.T) {
	if problems := validate([]ledger{{ID: "001-initial-build", Open: 0, Done: 34}}); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

// The incident this command exists for: `git mv` moved 017, an editor wrote
// its buffer back to the old path, and the change was live in two places.
func TestAChangeInTwoPlacesIsReported(t *testing.T) {
	problems := validate([]ledger{
		{ID: "017-email-posture-surface", Open: 0, Done: 40},
		{ID: "017-email-posture-surface", Archived: true, Open: 0, Done: 40},
	})
	found := false
	for _, p := range problems {
		if strings.Contains(p, "both changes/ and changes/archive/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("problems = %v", problems)
	}
}

func TestAnEmptyLedgerIsReported(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "changes/016-deployment-parity", "# Tasks\n\n- [ ] Something\n")
	mustWrite(t, filepath.Join(root, "changes", "016-deployment-parity", "tasks.md"), "   \n")

	_, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "is empty") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestAChangeWithoutAProposalIsReported(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "016-deployment-parity")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "tasks.md"), "- [ ] Something\n")

	_, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "no proposal.md") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestScanSeparatesActiveFromArchived(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "changes/016-deployment-parity", "- [ ] Open\n- [x] Done\n")
	writeChange(t, root, "changes/archive/003-go-sqlite-engine", "- [x] Done\n- [x] Done\n")

	ledgers, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if len(ledgers) != 2 {
		t.Fatalf("ledgers = %v", ledgers)
	}
	if ledgers[0].ID != "003-go-sqlite-engine" || !ledgers[0].Archived {
		t.Errorf("first ledger = %+v", ledgers[0])
	}
	if ledgers[1].ID != "016-deployment-parity" || ledgers[1].Archived {
		t.Errorf("second ledger = %+v", ledgers[1])
	}
	if ledgers[1].Open != 1 || ledgers[1].Done != 1 {
		t.Errorf("counts = %d, %d", ledgers[1].Open, ledgers[1].Done)
	}
}

// The "archive" directory sits inside "changes" and must not be read as a
// change in its own right.
func TestTheArchiveDirectoryIsNotAChange(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "changes/archive/003-go-sqlite-engine", "- [x] Done\n")

	ledgers, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if len(ledgers) != 1 {
		t.Fatalf("ledgers = %v", ledgers)
	}
}

func TestReconcileRewritesBothCountColumns(t *testing.T) {
	status := "| Change | Open | Done | What is left |\n" +
		"|---|---:|---:|---|\n" +
		"| `006-vantage-integration` | 9 | 4 | Close-out. |\n"

	updated, problems := reconcile(status, []ledger{{ID: "006-vantage-integration", Open: 2, Done: 69}})
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if !strings.Contains(updated, "| `006-vantage-integration` | 2 | 69 | Close-out. |") {
		t.Fatalf("updated =\n%s", updated)
	}
}

// The "not started" table carries only an open count. The command must not
// write the done count into whatever cell happens to be next, which would be
// prose.
func TestReconcileRewritesASingleCountColumn(t *testing.T) {
	status := "| `007-contact-probability` | 3 | The keystone: supplies P(contact). |\n"

	updated, _ := reconcile(status, []ledger{{ID: "007-contact-probability", Open: 35, Done: 0}})
	if !strings.Contains(updated, "| `007-contact-probability` | 35 | The keystone: supplies P(contact). |") {
		t.Fatalf("updated =\n%s", updated)
	}
}

// The archived table has no numeric cells at all. Its prose must survive
// untouched.
func TestReconcileLeavesTheArchivedTableAlone(t *testing.T) {
	status := "| `003-go-sqlite-engine` | Complete. Store, event bus, removal of the previous datastore. |\n"

	updated, _ := reconcile(status, []ledger{{ID: "003-go-sqlite-engine", Archived: true, Open: 0, Done: 11}})
	if updated != status {
		t.Fatalf("updated =\n%s\nwant unchanged", updated)
	}
}

// Rewriting only the counts is deliberate: the prose beside them is the part a
// human wrote, and regenerating the table would destroy the reasoning in order
// to correct two integers.
func TestReconcilePreservesProse(t *testing.T) {
	const note = "Phases 0-9 complete. What remains is close-out, and a pipe-free sentence."
	status := "| `006-vantage-integration` | 0 | 0 | " + note + " |\n"

	updated, _ := reconcile(status, []ledger{{ID: "006-vantage-integration", Open: 2, Done: 69}})
	if !strings.Contains(updated, note) {
		t.Fatalf("prose lost:\n%s", updated)
	}
}

func TestReconcileReportsAChangeMissingFromStatus(t *testing.T) {
	_, problems := reconcile("# Status\n", []ledger{{ID: "019-vulnerability-feed-ingestion", Open: 12}})
	if len(problems) != 1 || !strings.Contains(problems[0], "not named in STATUS.md") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestReconcileReportsAStatusRowWithNoChange(t *testing.T) {
	status := "| `042-imaginary-change` | 1 | 2 | Notes |\n"
	_, problems := reconcile(status, nil)
	if len(problems) != 1 || !strings.Contains(problems[0], "no change folder") {
		t.Fatalf("problems = %v", problems)
	}
}

// A count shrinking a digit must not reflow the table, or the real change is
// buried in a diff of alignment. A count that grows one widens its column by a
// character, which is accepted rather than fixed by crowding the pipe.
func TestPadLikeKeepsColumnWidth(t *testing.T) {
	if got := padLike("   9 ", "10"); got != "   10 " {
		t.Errorf("padLike = %q", got)
	}
	if got := padLike(" 100 ", "2"); got != "   2 " {
		t.Errorf("padLike = %q", got)
	}
	if got := padLike(" 9 ", "9"); got != " 9 " {
		t.Errorf("padLike = %q", got)
	}
}

// An archived ledger may be prose. 002 was superseded and its checklist was
// replaced with an explanation of why not to build it, which is more use than
// nine ticked boxes.
func TestAnArchivedLedgerMayBeProse(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "changes/archive/002-susceptibility-scoring-integration",
		"# Tasks\n\n## SUPERSEDED BY CHANGE 009 — DO NOT IMPLEMENT AS WRITTEN\n")

	_, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

// An active change with no checklist is a different thing: a proposal that
// never became a plan.
func TestAnActiveLedgerWithNoChecklistIsReported(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "changes/019-vulnerability-feed-ingestion", "# Tasks\n\nSome prose.\n")

	_, problems, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "no checklist items") {
		t.Fatalf("problems = %v", problems)
	}
}

func writeChange(t *testing.T, root, rel, tasks string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "tasks.md"), tasks)
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Change\n")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
