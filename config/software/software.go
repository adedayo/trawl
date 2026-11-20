// Package software carries the curated, versioned mapping from observed
// technology names (such as httpx -tech-detect output) to CPE vendor and
// product identifiers.
//
// Like config/signals, the map is data embedded directly into every binary so
// that desktop, server and worker entrypoints resolve observed software
// identically without depending on external files at runtime.
package software

import _ "embed"

//go:embed products.json
var productsV1 []byte

// MapJSON returns a fresh copy of the embedded product map.
func MapJSON() []byte {
	out := make([]byte, len(productsV1))
	copy(out, productsV1)
	return out
}
