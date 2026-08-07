package store

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Opener builds a Store from a data source name.
//
// The DSN is passed through unchanged. Each backend understands its own
// connection string, and a factory that tried to normalise them would have to
// know something about every backend — which is the coupling this indirection
// exists to remove.
type Opener func(dsn string) (Store, error)

var (
	openersMu sync.RWMutex
	openers   = map[string]Opener{}
)

// Register associates a DSN scheme with the backend that serves it.
//
// Backends register themselves from an init function, so linking a backend in
// is what makes it available. A deployment that never imports the Postgres
// package cannot accidentally be configured into it, and the binary does not
// carry a driver nobody asked for.
//
// Registering the same scheme twice panics. Two backends answering to one
// scheme would make the selected implementation depend on package
// initialisation order, and a store chosen by link order is a store chosen at
// random.
func Register(scheme string, open Opener) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" {
		panic("store: a backend cannot register an empty scheme")
	}
	if open == nil {
		panic("store: a backend cannot register a nil opener")
	}

	openersMu.Lock()
	defer openersMu.Unlock()

	if _, exists := openers[scheme]; exists {
		panic(fmt.Sprintf("store: two backends registered for the scheme %q", scheme))
	}
	openers[scheme] = open
}

// Schemes lists the registered backends, sorted, for diagnostics.
func Schemes() []string {
	openersMu.RLock()
	defer openersMu.RUnlock()

	out := make([]string, 0, len(openers))
	for scheme := range openers {
		out = append(out, scheme)
	}
	sort.Strings(out)
	return out
}

// DefaultScheme is assumed when a DSN names no scheme.
//
// SQLite remains the default because it is what a desktop install and a
// single-instance container both want, and neither should have to be
// configured to get it.
const DefaultScheme = "sqlite"

// Open builds the Store described by a DSN.
//
// Recognised forms:
//
//	""                          the default backend's own default location
//	/var/lib/trawl/trawl.db     a bare path, taken as the default backend
//	sqlite:/var/lib/trawl.db    an explicit scheme
//	libsql://host?authToken=…   a networked SQLite-compatible backend
//	postgres://user@host/trawl  a networked relational backend
//
// An unrecognised scheme is an error rather than a fallback to the default.
// Silently opening a local file when an operator asked for a shared database
// would produce a working process holding a private, empty copy of the estate
// — a deployment that looks healthy and knows nothing.
func Open(dsn string) (Store, error) {
	scheme, err := schemeOf(dsn)
	if err != nil {
		return nil, err
	}

	openersMu.RLock()
	open, ok := openers[scheme]
	openersMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf(
			"store: no backend is linked for the scheme %q (available: %s); "+
				"a backend is enabled by importing its package",
			scheme, strings.Join(Schemes(), ", "))
	}
	return open(dsn)
}

// SchemeOf reports which backend a DSN selects, without opening it.
//
// Callers use this to describe the deployment accurately — a warning about
// SQLite's single-writer semantics is misleading when the DSN points at a
// networked database, and a warning nobody should have read is a warning
// everybody learns to ignore.
func SchemeOf(dsn string) string {
	scheme, err := schemeOf(dsn)
	if err != nil {
		return DefaultScheme
	}
	return scheme
}

// schemeOf extracts the backend scheme from a DSN.
//
// A bare filesystem path is the case that needs care: Windows paths carry a
// drive letter, and treating "C:\trawl\trawl.db" as the scheme "c" would
// reject a perfectly ordinary configuration. A scheme is therefore recognised
// only when it is followed by a separator and is longer than a single
// character.
func schemeOf(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return DefaultScheme, nil
	}

	idx := strings.Index(dsn, ":")
	if idx <= 1 {
		// No colon, or a single-character prefix that is a drive letter.
		return DefaultScheme, nil
	}

	scheme := strings.ToLower(dsn[:idx])
	for _, r := range scheme {
		// A scheme is alphanumeric with '+', '-' and '.'; anything else means
		// the colon belonged to the path, not to a scheme.
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') &&
			r != '+' && r != '-' && r != '.' {
			return DefaultScheme, nil
		}
	}
	return scheme, nil
}
