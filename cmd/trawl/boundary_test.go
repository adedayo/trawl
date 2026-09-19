package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"testing"
)

// domainPackages are the packages a transport must not reach into directly.
//
// pkg/core is absent deliberately: forwarding to core is what a transport is
// for. Everything here sits behind core, and a transport that calls it is a
// transport that has grown behaviour of its own.
var domainPackages = map[string]bool{
	"scanner": true,
	"service": true,
	"sqlite":  true,
	"vantage": true,
	"store":   true,
}

// storeBypassExemptions name the transport methods that talk to the store
// directly, each with the reason it has not been moved behind core.
//
// These are debt, not design. They are listed rather than silently permitted
// so the count can only go down: a new method reaching past core fails this
// test, and removing one of these means deleting a line here.
//
// The job queue and raw-ingest paths are server-only, so the parity check
// never forced them through core. That made them invisible to every other
// guard in the repository.
var storeBypassExemptions = map[string]string{
	"handleEnqueueJob":         "job queue is server-only; no core method exists yet",
	"handlePopJob":             "job queue is server-only; no core method exists yet",
	"handleCompleteJob":        "job queue is server-only; no core method exists yet",
	"ingestRaw":                "persists the raw payload before correlation; that durability guarantee is not yet expressed in core",
	"handleIngestEmailPosture": "writes the posture directly; core has no ingest-side posture writer",
}

// TestTransportsHoldNoBehaviour enforces the first requirement of the
// deployment-parity capability: every operation lives in the application
// layer, and transports only forward to it.
//
// Import position alone is too blunt a rule here. A transport legitimately
// names domain types in its signatures, and the composition root must
// construct a store to wire anything at all. What matters is whether a method
// calls into a domain package, because that is where behaviour starts to
// accumulate on one transport and not the other.
func TestTransportsHoldNoBehaviour(t *testing.T) {
	cases := []struct {
		file     string
		receiver string
	}{
		// Unexported methods on App are checked too, not just the bound ones:
		// a lifecycle hook reaching past core is the same defect arriving by a
		// quieter route.
		{file: filepath.Join("..", "..", "app.go"), receiver: "App"},
		// Every method on server, not only handle*. ingestRaw is a helper that
		// reaches past core, and a rule inspecting handlers alone would have
		// let it through. Behaviour does not become acceptable by sitting one
		// call away from the route.
		{file: "server.go", receiver: "server"},
	}

	for _, tc := range cases {
		t.Run(filepath.Base(tc.file), func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tc.file, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", tc.file, err)
			}

			var checked int
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !hasReceiver(fn, tc.receiver) {
					continue
				}
				checked++

				for _, pkg := range domainCallsIn(fn) {
					if reason, exempt := storeBypassExemptions[fn.Name.Name]; exempt {
						t.Logf("known bypass: %s calls %s.* - %s", fn.Name.Name, pkg, reason)
						continue
					}
					t.Errorf("%s calls %s.* directly.\n"+
						"Transports forward to pkg/core and hold no behaviour of their own.\n"+
						"Move the operation into core, or record an exemption with its reason.",
						fn.Name.Name, pkg)
				}
			}

			if checked == 0 {
				t.Fatalf("no methods found on *%s in %s; the parser has drifted and this test is silently passing",
					tc.receiver, tc.file)
			}
		})
	}
}

// TestStoreBypassExemptionsAreLive stops the exemption list outliving the
// methods it excuses. A stale exemption reads as a known problem that is still
// there, which is worse than no note at all.
func TestStoreBypassExemptionsAreLive(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing server.go: %v", err)
	}

	live := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !hasReceiver(fn, "server") {
			continue
		}
		if len(domainCallsIn(fn)) > 0 {
			live[fn.Name.Name] = true
		}
	}

	var stale []string
	for name := range storeBypassExemptions {
		if !live[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		t.Errorf("%s is exempted from the transport boundary but no longer reaches past core; remove the exemption", name)
	}
}

func hasReceiver(fn *ast.FuncDecl, typeName string) bool {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	return ok && ident.Name == typeName
}

// domainCallsIn reports the domain packages a function calls into, whether
// directly as store.Open or through a field as s.store.SaveSetting.
//
// A type conversion such as store.AssetStatus(x) is not behaviour and is not
// reported: the transport is still only forwarding, it is merely naming the
// type it forwards.
func domainCallsIn(fn *ast.FuncDecl) []string {
	if fn.Body == nil {
		return nil
	}
	found := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if inner, ok := sel.X.(*ast.SelectorExpr); ok {
			if domainPackages[inner.Sel.Name] {
				found[inner.Sel.Name] = true
			}
			return true
		}

		if ident, ok := sel.X.(*ast.Ident); ok && domainPackages[ident.Name] {
			if !isLikelyTypeConversion(call) {
				found[ident.Name] = true
			}
		}
		return true
	})

	var pkgs []string
	for p := range found {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

// isLikelyTypeConversion distinguishes store.AssetStatus(x) from store.Open(x).
// This cannot be decided from syntax alone without type information, so the
// heuristic is narrow: only single-argument calls count as conversions, which
// is the shape every conversion in this codebase has.
func isLikelyTypeConversion(call *ast.CallExpr) bool {
	return len(call.Args) == 1
}
