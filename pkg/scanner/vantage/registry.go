package vantage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/adedayo/trawl/pkg/store"
)

// SignalRegistry maps vantage finding identifiers onto what they bear on.
//
// It is versioned data rather than code because the mapping is a judgement
// about what a signal means, and that judgement changes independently of both
// the library that produces the signal and the engine that consumes it. The
// version is stamped on every observation so a later reader can tell a change
// of interpretation from a change in the domain.
type SignalRegistry struct {
	version string
	entries map[string]store.SignalRegistryEntry
}

// registryFile is the on-disk schema.
type registryFile struct {
	RegistryVersion string                      `json:"registryVersion"`
	LibraryMajor    string                      `json:"libraryMajor"`
	Entries         []store.SignalRegistryEntry `json:"entries"`
}

// LoadSignalRegistry reads a registry from a file.
func LoadSignalRegistry(path string) (*SignalRegistry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("vantage: opening the signal registry: %w", err)
	}
	defer func() { _ = f.Close() }()
	return ReadSignalRegistry(f)
}

// ReadSignalRegistry parses a registry.
func ReadSignalRegistry(r io.Reader) (*SignalRegistry, error) {
	var file registryFile
	if err := json.NewDecoder(r).Decode(&file); err != nil {
		return nil, fmt.Errorf("vantage: parsing the signal registry: %w", err)
	}
	if file.RegistryVersion == "" {
		// An unversioned registry would make every observation's provenance a
		// lie, so it is refused rather than defaulted.
		return nil, fmt.Errorf("vantage: the signal registry declares no version")
	}

	reg := &SignalRegistry{
		version: file.RegistryVersion,
		entries: make(map[string]store.SignalRegistryEntry, len(file.Entries)),
	}
	for _, e := range file.Entries {
		if e.SignalID == "" {
			return nil, fmt.Errorf("vantage: the signal registry contains an entry with no identifier")
		}
		if _, dup := reg.entries[e.SignalID]; dup {
			return nil, fmt.Errorf("vantage: the signal registry defines %q twice", e.SignalID)
		}
		e.RegistryVersion = file.RegistryVersion
		reg.entries[e.SignalID] = e
	}
	return reg, nil
}

// Version implements Registry.
func (r *SignalRegistry) Version() string {
	if r == nil {
		return ""
	}
	return r.version
}

// Lookup implements Registry.
func (r *SignalRegistry) Lookup(signalID string) (store.SignalRegistryEntry, bool) {
	if r == nil {
		return store.SignalRegistryEntry{}, false
	}
	e, ok := r.entries[signalID]
	return e, ok
}

// IDs returns every mapped identifier, sorted.
func (r *SignalRegistry) IDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.entries))
	for id := range r.entries {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Entries returns every mapping, for persisting into the store.
func (r *SignalRegistry) Entries() []store.SignalRegistryEntry {
	if r == nil {
		return nil
	}
	out := make([]store.SignalRegistryEntry, 0, len(r.entries))
	for _, id := range r.IDs() {
		out = append(out, r.entries[id])
	}
	return out
}

// MissingFrom reports identifiers the library can raise that this registry does
// not map.
//
// This is the completeness check. An unmapped identifier is not a crash — the
// adapter retains it, marked unmapped — but it is a gap in the risk model, and
// a gap that nobody is told about is one that never gets closed.
func (r *SignalRegistry) MissingFrom(catalogueIDs []string) []string {
	var missing []string
	for _, id := range catalogueIDs {
		if _, ok := r.Lookup(id); !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}
