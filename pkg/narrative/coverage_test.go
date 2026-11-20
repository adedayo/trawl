package narrative

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adedayo/trawl/config/signals"
	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// These tests make a library upgrade that adds an identifier fail the build,
// naming it, instead of letting new findings render with the generic wording.

func loadEmbedded(t *testing.T) *Registry {
	t.Helper()
	r, err := Load()
	if err != nil {
		t.Fatalf("loading embedded narratives: %v", err)
	}
	return r
}

// registryEntry is the part of a signal-registry entry this package needs.
type registryEntry struct {
	SignalID      string `json:"signalId"`
	WeaknessClass string `json:"weaknessClass"`
}

func signalRegistry(t *testing.T) []registryEntry {
	t.Helper()
	var doc struct {
		Entries []registryEntry `json:"entries"`
	}
	if err := json.Unmarshal(signals.RegistryJSON(), &doc); err != nil {
		t.Fatalf("parsing the signal registry: %v", err)
	}
	if len(doc.Entries) == 0 {
		t.Fatal("signal registry has no entries")
	}
	return doc.Entries
}

func TestEveryCatalogueIdentifierHasANarrative(t *testing.T) {
	r := loadEmbedded(t)
	classOf := map[string]string{}
	for _, e := range signalRegistry(t) {
		classOf[e.SignalID] = e.WeaknessClass
	}
	catalogue := vfinding.Catalogue()
	if len(catalogue) == 0 {
		t.Fatal("vantage catalogue is empty")
	}
	for _, entry := range catalogue {
		in := Input{SignalID: entry.ID, WeaknessClass: classOf[entry.ID], Check: entry.Check}
		got := r.Explain(in)
		if got.Level == LevelGeneric {
			t.Errorf("catalogue identifier %s (check %q, weakness class %q) has no narrative or fallback: add a %s entry for the class or check, or a %s entry for the identifier",
				entry.ID, entry.Check, classOf[entry.ID], LevelWeaknessClass, LevelSignal)
		}
		// Even with the registry mapping missing, the check family must catch it.
		if byCheck := r.Explain(Input{SignalID: entry.ID, Check: entry.Check}); byCheck.Level == LevelGeneric {
			t.Errorf("catalogue identifier %s: check family %q has no narrative", entry.ID, entry.Check)
		}
	}
}

func TestEverySignalRegistryClassAndSignalIsCovered(t *testing.T) {
	r := loadEmbedded(t)
	classes := map[string]bool{}
	for _, e := range signalRegistry(t) {
		classes[e.WeaknessClass] = true
	}
	for class := range classes {
		if _, ok := r.Entry(LevelWeaknessClass, class); !ok {
			t.Errorf("signal registry weakness class %q has no %s narrative", class, LevelWeaknessClass)
		}
	}
	// Narratives must not name classes or signals that do not exist: a typo in
	// a key would silently leave the intended entry unused.
	known := map[string]bool{}
	for _, e := range signalRegistry(t) {
		known[e.SignalID] = true
	}
	for _, key := range r.Keys(LevelWeaknessClass) {
		if !classes[key] {
			t.Errorf("%s narrative %q matches no weakness class in the signal registry", LevelWeaknessClass, key)
		}
	}
	for _, key := range r.Keys(LevelSignal) {
		if !known[key] {
			t.Errorf("%s narrative %q matches no signal in the registry", LevelSignal, key)
		}
	}
}

func TestEveryCatalogueCheckFamilyHasANarrative(t *testing.T) {
	r := loadEmbedded(t)
	checks := map[string]bool{}
	for _, entry := range vfinding.Catalogue() {
		checks[entry.Check] = true
	}
	for check := range checks {
		if _, ok := r.Entry(LevelCheck, check); !ok {
			t.Errorf("vantage check family %q has no %s narrative", check, LevelCheck)
		}
	}
}

// Service posture advisories are raised by pkg/scanner/vantage with a short
// code. The caller passes the code as Input.Check, so each needs a check entry.
func TestEveryServiceAdvisoryCodeHasANarrative(t *testing.T) {
	r := loadEmbedded(t)
	src, err := filepath.Abs(filepath.Join("..", "scanner", "vantage", "reachability.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", src, err)
	}
	var codes []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "saveAdvisory" || len(call.Args) < 3 {
			return true
		}
		if lit, ok := call.Args[2].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			code, _ := strconv.Unquote(lit.Value)
			codes = append(codes, code)
		}
		return true
	})
	if len(codes) == 0 {
		t.Fatal("found no service advisory codes in reachability.go; the extraction needs updating")
	}
	for _, code := range codes {
		if got := r.Explain(Input{Check: code}); got.Level != LevelCheck || got.LevelKey != code {
			t.Errorf("service advisory %q has no %s narrative (resolved to %s/%s)", code, LevelCheck, got.Level, got.LevelKey)
		}
	}
}

func TestEveryServiceClassHasANarrative(t *testing.T) {
	r := loadEmbedded(t)
	src, err := filepath.Abs(filepath.Join("..", "contact", "service_class.go"))
	if err != nil {
		t.Fatal(err)
	}
	classes := stringConstants(t, src, "ServiceClass")
	if len(classes) < 8 {
		t.Fatalf("expected at least the eight known service classes, found %v", classes)
	}
	for _, class := range classes {
		if got := r.Explain(Input{ServiceClass: class}); got.Level != LevelServiceClass {
			t.Errorf("service class %q has no %s narrative (resolved to %s)", class, LevelServiceClass, got.Level)
		}
	}
}

func TestEverySecretTypeHasANarrative(t *testing.T) {
	r := loadEmbedded(t)
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/adedayo/checkmate").Output()
	if err != nil {
		t.Fatalf("locating the checkmate module: %v", err)
	}
	src := filepath.Join(strings.TrimSpace(string(out)), "pkg", "sdk", "types.go")
	types := stringConstants(t, src, "SecretType")
	if len(types) < 20 {
		t.Fatalf("expected the checkmate secret taxonomy, found only %v", types)
	}
	for _, secret := range types {
		if got := r.Explain(Input{SecretType: secret}); got.Level != LevelSecretType {
			t.Errorf("secret type %q has no %s narrative, not even a family entry (resolved to %s)", secret, LevelSecretType, got.Level)
		}
	}
	// A type this taxonomy does not know yet still gets an explanation.
	if got := r.Explain(Input{SecretType: "brand.new_kind"}); got.Level != LevelGeneric {
		t.Errorf("an unknown secret type should fall back to the generic narrative, got %s", got.Level)
	}
}

func TestFallbackTemplatesExist(t *testing.T) {
	r := loadEmbedded(t)
	if got := r.Explain(Input{Source: "candidate", CVE: "CVE-2024-0001"}); got.Level != LevelCandidate || got.LevelKey != KeyCandidate {
		t.Errorf("a candidate finding resolved to %s/%s, want %s/%s", got.Level, got.LevelKey, LevelCandidate, KeyCandidate)
	}
	if got := r.Explain(Input{CVE: "CVE-2024-0001"}); got.Level != LevelCVE {
		t.Errorf("a CVE finding resolved to %s, want %s", got.Level, LevelCVE)
	}
	if got := r.Explain(Input{}); got.Level != LevelGeneric || got.LevelKey != KeyGeneric {
		t.Errorf("an empty input resolved to %s/%s", got.Level, got.LevelKey)
	}
}

// Every embedded entry must be reachable through some input; an entry nothing
// can select is dead content that reviewers would still have to maintain.
func TestNoUnreachableNarratives(t *testing.T) {
	r := loadEmbedded(t)
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck, LevelServiceClass, LevelSecretType} {
		for _, key := range r.Keys(level) {
			in := Input{}
			switch level {
			case LevelSignal:
				in.SignalID = key
			case LevelWeaknessClass:
				in.WeaknessClass = key
			case LevelCheck:
				in.Check = key
			case LevelServiceClass:
				in.ServiceClass = key
			case LevelSecretType:
				in.SecretType = strings.Replace(key, ".*", ".anything", 1)
			}
			if got := r.Explain(in); got.Level != level || got.LevelKey != key {
				t.Errorf("%s entry %q is unreachable: input resolves to %s/%s", level, key, got.Level, got.LevelKey)
			}
		}
	}
}

func TestExplainIsDeterministicForEveryCatalogueIdentifier(t *testing.T) {
	r := loadEmbedded(t)
	classOf := map[string]string{}
	for _, e := range signalRegistry(t) {
		classOf[e.SignalID] = e.WeaknessClass
	}
	for _, entry := range vfinding.Catalogue() {
		in := catalogueInput(entry, classOf[entry.ID])
		first, _ := json.Marshal(r.Explain(in))
		for i := 0; i < 5; i++ {
			again, _ := json.Marshal(r.Explain(in))
			if string(first) != string(again) {
				t.Fatalf("%s rendered differently on repeat:\n%s\n%s", entry.ID, first, again)
			}
		}
	}
}

// catalogueInput builds the input a caller would pass for a catalogue finding.
func catalogueInput(entry vfinding.Entry, class string) Input {
	return Input{
		Source: "signal", SignalID: entry.ID, WeaknessClass: class, Check: entry.Check,
		Title:    entry.Title,
		Detail:   "Observed on the host under test",
		Evidence: "evidence-for-" + entry.ID,
		Catalogue: &Catalogue{
			Title: entry.Title, Description: entry.Description, Remediation: entry.Remediation,
			References: entry.References, Tags: entry.Tags,
		},
	}
}

// stringConstants returns the values of the string constants of the named type
// declared in a Go source file, sorted. Parsing the source, rather than listing
// the constants here, is what lets a new constant fail the build.
func stringConstants(t *testing.T, path, typeName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var values []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != typeName {
				continue
			}
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					s, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("%s: %v", path, err)
					}
					values = append(values, s)
				}
			}
		}
	}
	sort.Strings(values)
	return values
}
