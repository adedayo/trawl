package store

import (
	"strings"
	"testing"
)

// TestSchemeOf pins the parsing rules, because every mistake here is silent:
// a misread DSN opens the wrong database rather than failing, and the process
// then runs happily against state nobody expected.
func TestSchemeOf(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"empty selects the default", "", DefaultScheme},
		{"whitespace is not a scheme", "   ", DefaultScheme},
		{"bare absolute path", "/var/lib/trawl/trawl.db", DefaultScheme},
		{"bare relative path", "data/trawl.db", DefaultScheme},
		{"windows drive letter is a path, not a scheme", `C:\ProgramData\trawl.db`, DefaultScheme},
		{"explicit sqlite", "sqlite:/var/lib/trawl.db", "sqlite"},
		{"explicit sqlite with authority", "sqlite:///var/lib/trawl.db", "sqlite"},
		{"scheme case is normalised", "SQLite:/var/lib/trawl.db", "sqlite"},
		{"file", "file:/var/lib/trawl.db", "file"},
		{"libsql", "libsql://db.example.com?authToken=abc", "libsql"},
		{"postgres", "postgres://user:pw@host:5432/trawl", "postgres"},
		{"a colon inside a path is not a scheme", "/var/lib/my db:1/trawl.db", DefaultScheme},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SchemeOf(tc.dsn)
			if got != tc.want {
				t.Fatalf("SchemeOf(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}

// TestOpenUnknownSchemeIsAnError guards the decision not to fall back.
//
// Falling back to a local file when an operator asked for a shared database
// yields a process that starts, passes its health check, and reports an empty
// estate. An error at startup is the only outcome that cannot be mistaken for
// a clean bill of health.
func TestOpenUnknownSchemeIsAnError(t *testing.T) {
	withOpeners(t, map[string]Opener{
		"sqlite": func(string) (Store, error) { return nil, nil },
	})

	_, err := Open("postgres://user@host/trawl")
	if err == nil {
		t.Fatal("expected an error for an unregistered scheme, got nil")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Errorf("the error should name the requested scheme, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sqlite") {
		t.Errorf("the error should list what is available, got: %v", err)
	}
}

// TestOpenPassesTheDSNThrough confirms the factory does not rewrite the DSN.
// Backends parse their own connection strings; a factory that edited them
// would need to understand all of them.
func TestOpenPassesTheDSNThrough(t *testing.T) {
	var seen string
	withOpeners(t, map[string]Opener{
		"libsql": func(dsn string) (Store, error) {
			seen = dsn
			return nil, nil
		},
	})

	const dsn = "libsql://db.example.com?authToken=secret"
	if _, err := Open(dsn); err != nil {
		t.Fatalf("Open returned an unexpected error: %v", err)
	}
	if seen != dsn {
		t.Errorf("the opener received %q, want the DSN unchanged: %q", seen, dsn)
	}
}

// TestRegisterRejectsDuplicates protects backend selection from link order.
func TestRegisterRejectsDuplicates(t *testing.T) {
	withOpeners(t, nil)

	Register("duplicate", func(string) (Store, error) { return nil, nil })

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic when registering a scheme twice")
		}
	}()
	Register("duplicate", func(string) (Store, error) { return nil, nil })
}

func TestRegisterRejectsNilOpener(t *testing.T) {
	withOpeners(t, nil)

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic when registering a nil opener")
		}
	}()
	Register("broken", nil)
}

// withOpeners swaps the registry for the duration of a test, so that tests do
// not depend on which backends happen to be linked into the test binary.
func withOpeners(t *testing.T, replacement map[string]Opener) {
	t.Helper()

	openersMu.Lock()
	previous := openers
	openers = map[string]Opener{}
	for scheme, open := range replacement {
		openers[scheme] = open
	}
	openersMu.Unlock()

	t.Cleanup(func() {
		openersMu.Lock()
		openers = previous
		openersMu.Unlock()
	})
}
