package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// wailsMethodRoutes maps every method the desktop transport binds over Wails
// IPC to the HTTP route that serves the same operation.
//
// This table is the parity contract made checkable. Deployment parity is an
// invariant about the whole codebase, and an invariant that only prose defends
// holds until the next person adds a handler — two capabilities, GetVersion
// and ScanEmailPosture, reached the desktop build and never reached HTTP
// precisely because nothing failed when they didn't.
//
// A method mapped to the empty string is deliberately desktop-only and must
// carry its reason here. There are none today, and that is the point: the
// empty string is a decision someone has to write down, not a default.
var wailsMethodRoutes = map[string]string{
	"GetVersion":              "GET /api/v1/version",
	"GetAssets":               "GET /api/v1/assets",
	"RemoveAsset":             "DELETE /api/v1/assets/{id}",
	"GetFindings":             "GET /api/v1/findings",
	"GetSecretFindings":       "GET /api/v1/secret-findings",
	"GetEmailPostures":        "GET /api/v1/email-postures",
	"ScanEmailPosture":        "POST /api/v1/email-postures/{domain}",
	"GetDomainAssessment":     "GET /api/v1/assessments/{domain}",
	"GetDomainAssessments":    "GET /api/v1/assessments",
	"AssessDomain":            "POST /api/v1/assessments/{domain}",
	"ProbeDiscoveredServices": "POST /api/v1/service-probes",
	"GetRegressions":          "GET /api/v1/regressions",
	"GetSetting":              "GET /api/v1/settings/{key}",
	"SaveSetting":             "PUT /api/v1/settings/{key}",
	"TriggerScan":             "POST /api/v1/scans",
	"EraseDiscoveredData":     "DELETE /api/v1/discovered-data",
}

// TestEveryWailsMethodHasAnHTTPRoute enforces the central claim of the
// deployment-parity capability: a capability cannot exist in one deployment
// and not the other.
//
// It reads app.go as source rather than reflecting over the App type, because
// the desktop transport lives in a different main package and cannot be
// imported from here. Parsing is the price of checking the thing that matters.
func TestEveryWailsMethodHasAnHTTPRoute(t *testing.T) {
	methods := wailsBoundMethods(t)
	if len(methods) == 0 {
		t.Fatal("no Wails-bound methods found in app.go; the parser has drifted from the source and this test is silently passing")
	}

	routes := registeredRoutes(t)

	for _, m := range methods {
		route, mapped := wailsMethodRoutes[m]
		if !mapped {
			t.Errorf("App.%s is bound over Wails IPC but is absent from the parity table.\n"+
				"Add its HTTP route, or map it to \"\" with a written reason for why this\n"+
				"operation is desktop-only.", m)
			continue
		}
		if route == "" {
			continue // deliberately desktop-only; the reason is recorded in the table
		}
		if !routes[route] {
			t.Errorf("App.%s maps to %q, which no HTTP route serves.\n"+
				"The container deployment cannot perform this operation.", m, route)
		}
	}

	// The table must not outlive the methods it describes, or it becomes a
	// record of a parity that used to exist.
	known := make(map[string]bool, len(methods))
	for _, m := range methods {
		known[m] = true
	}
	for m := range wailsMethodRoutes {
		if !known[m] {
			t.Errorf("the parity table maps App.%s, which app.go no longer binds; remove the stale entry", m)
		}
	}
}

// wailsBoundMethods returns the exported methods on *App, which is exactly the
// set Wails exposes to the frontend.
func wailsBoundMethods(t *testing.T) []string {
	t.Helper()

	path := filepath.Join("..", "..", "app.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	var methods []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if !ok || ident.Name != "App" {
			continue
		}
		if !fn.Name.IsExported() {
			continue // lifecycle hooks such as startup are not bound
		}
		methods = append(methods, fn.Name.Name)
	}
	sort.Strings(methods)
	return methods
}

// registeredRoutes returns the pattern of every route registered on the mux.
//
// It reads the source rather than starting a server, so the check runs without
// a store, a bus or a port.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()

	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("reading server.go: %v", err)
	}

	routes := map[string]bool{}
	for _, line := range strings.Split(string(src), "\n") {
		const marker = `mux.HandleFunc("`
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		rest := line[i+len(marker):]
		end := strings.Index(rest, `"`)
		if end < 0 {
			continue
		}
		routes[rest[:end]] = true
	}
	return routes
}
