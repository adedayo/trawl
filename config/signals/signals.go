// Package signals carries the versioned mapping from vantage finding
// identifiers to what they bear on.
//
// The registry is data rather than code because the mapping is a judgement
// about what a signal means, and that judgement changes independently of both
// the library that raises the signal and the engine that consumes it.
//
// It is exposed as an embedded byte slice rather than a file path so that
// every entrypoint — the desktop binary, the headless server, the workers —
// carries the same registry inside itself. A registry that has to be located
// on disk is one that is absent in production, and observations mapped against
// a missing registry would all be recorded as unmapped: a silent, total loss
// of meaning that looks exactly like a domain with nothing wrong with it.
package signals

import _ "embed"

//go:embed vantage-1.json
var registryV1 []byte

// RegistryJSON returns the registry for the vantage 1.x major series.
//
// It returns a copy. The registry is process-wide state, and handing out the
// backing array would let one caller's mishandling corrupt every subsequent
// load.
func RegistryJSON() []byte {
	out := make([]byte, len(registryV1))
	copy(out, registryV1)
	return out
}
