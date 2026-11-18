package contact

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSignalRegistrySingleStageConsumption enforces Change 007 Phase 5: every
// mapped signal declares exactly one valid consuming stage, and no signal
// identifier appears under two stages.
func TestSignalRegistrySingleStageConsumption(t *testing.T) {
	path := filepath.Join("..", "..", "config", "signals", "vantage-1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading signal registry: %v", err)
	}
	var reg struct {
		Entries []struct {
			SignalID string `json:"signalId"`
			Stage    string `json:"stage"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("parsing signal registry: %v", err)
	}
	if len(reg.Entries) == 0 {
		t.Fatal("signal registry has no entries")
	}

	allowedStages := map[string]bool{
		"reconnaissance": true,
		"delivery":       true,
		"exploitation":   true,
		"contact":        true,
		"attempt":        true,
		"success":        true,
	}
	seen := map[string]string{}
	for _, entry := range reg.Entries {
		stage := strings.TrimSpace(entry.Stage)
		if stage == "" {
			t.Errorf("%s declares no consuming stage", entry.SignalID)
			continue
		}
		if strings.ContainsAny(stage, ",;|/ ") {
			t.Errorf("%s declares multiple stages %q; single-stage consumption is mandatory", entry.SignalID, stage)
		}
		if !allowedStages[stage] {
			t.Errorf("%s declares unrecognised stage %q", entry.SignalID, stage)
		}
		if prev, dup := seen[entry.SignalID]; dup && prev != stage {
			t.Errorf("%s is claimed by both %q and %q", entry.SignalID, prev, stage)
		}
		seen[entry.SignalID] = stage
	}
}

// TestNoAssessmentPathCanWriteAContactPosterior enforces Guardrail 7 and
// Change 007 / Change 024: no scanner, vantage adapter, or assessment service
// path may write or define a contact posterior.
func TestNoAssessmentPathCanWriteAContactPosterior(t *testing.T) {
	dirs := []string{
		filepath.Join("..", "scanner"),
		filepath.Join("..", "scanner", "vantage"),
		filepath.Join("..", "service"),
	}
	forbidden := []string{
		"SaveContactPosterior",
		"UpdateContactPosterior",
		"WriteContactPosterior",
		"RecordContactPosterior",
		"SetContactPosterior",
		"ContactPosterior",
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		fset := token.NewFileSet()
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			full := filepath.Join(dir, entry.Name())
			file, err := parser.ParseFile(fset, full, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", full, err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				for _, bad := range forbidden {
					if ident.Name == bad {
						pos := fset.Position(ident.Pos())
						t.Errorf("%s:%d:%d references forbidden posterior symbol %q on an assessment path",
							full, pos.Line, pos.Column, ident.Name)
					}
				}
				return true
			})
		}
	}
}
