package software

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	cfgsoftware "github.com/adedayo/trawl/config/software"
)

// MapEntry defines one reviewed mapping from observed technology names to a
// CPE vendor and product.
type MapEntry struct {
	Names   []string `json:"names"`
	CPEPart string   `json:"cpePart"` // "a" (application), "o" (os), "h" (hardware)
	Vendor  string   `json:"vendor"`
	Product string   `json:"product"`
}

// ProductIdentity is the resolved CPE identity for an observed name.
type ProductIdentity struct {
	CPEPart        string
	Vendor         string
	Product        string
	MappingVersion string
}

type mapFile struct {
	Version              string     `json:"version"`
	Entries              []MapEntry `json:"entries"`
	DeliberatelyUnmapped []string   `json:"deliberatelyUnmapped"`
}

// ProductMap is a validated, immutable lookup table from observed technology
// names to CPE vendor/product pairs.
type ProductMap struct {
	version              string
	byName               map[string]ProductIdentity
	deliberatelyUnmapped map[string]bool
	entries              []MapEntry
}

// LoadMap loads and validates the embedded curated product map.
func LoadMap() (*ProductMap, error) {
	return LoadMapFrom(cfgsoftware.MapJSON())
}

// LoadMapFrom decodes a product map with strict field checking and validates
// that every entry names a vendor and product and that no name collides on
// conflicting products.
func LoadMapFrom(data []byte) (*ProductMap, error) {
	var doc mapFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("software map: decode: %w", err)
	}
	if strings.TrimSpace(doc.Version) == "" {
		return nil, errors.New("software map: version is required")
	}
	if len(doc.Entries) == 0 {
		return nil, errors.New("software map: entries must not be empty")
	}

	m := &ProductMap{
		version:              strings.TrimSpace(doc.Version),
		byName:               make(map[string]ProductIdentity, len(doc.Entries)*2),
		deliberatelyUnmapped: make(map[string]bool, len(doc.DeliberatelyUnmapped)),
		entries:              doc.Entries,
	}

	for i, e := range doc.Entries {
		vendor := strings.TrimSpace(e.Vendor)
		product := strings.TrimSpace(e.Product)
		part := strings.TrimSpace(e.CPEPart)
		if part == "" {
			part = "a"
		}
		if vendor == "" || product == "" {
			return nil, fmt.Errorf("software map: entry %d must name both vendor and product", i+1)
		}
		if part != "a" && part != "o" && part != "h" {
			return nil, fmt.Errorf("software map: entry %d has invalid cpePart %q", i+1, part)
		}
		if len(e.Names) == 0 {
			return nil, fmt.Errorf("software map: entry %d (%s:%s) must list at least one observed name", i+1, vendor, product)
		}
		id := ProductIdentity{
			CPEPart:        part,
			Vendor:         vendor,
			Product:        product,
			MappingVersion: m.version,
		}
		for _, rawName := range e.Names {
			key := normalizeName(rawName)
			if key == "" {
				return nil, fmt.Errorf("software map: entry %d (%s:%s) contains an empty name", i+1, vendor, product)
			}
			if prev, exists := m.byName[key]; exists {
				if prev.Vendor != id.Vendor || prev.Product != id.Product || prev.CPEPart != id.CPEPart {
					return nil, fmt.Errorf("software map: name %q collides on %s:%s and %s:%s",
						rawName, prev.Vendor, prev.Product, id.Vendor, id.Product)
				}
			}
			m.byName[key] = id
		}
	}

	for _, rawName := range doc.DeliberatelyUnmapped {
		key := normalizeName(rawName)
		if key == "" {
			return nil, errors.New("software map: deliberatelyUnmapped contains an empty name")
		}
		if _, mapped := m.byName[key]; mapped {
			return nil, fmt.Errorf("software map: name %q is both mapped and deliberatelyUnmapped", rawName)
		}
		m.deliberatelyUnmapped[key] = true
	}

	return m, nil
}

// Version returns the curated map's version string.
func (m *ProductMap) Version() string { return m.version }

// Lookup resolves an observed technology name to its curated CPE identity.
// Returns (identity, true) if mapped, or (zero, false) if unmapped.
func (m *ProductMap) Lookup(observedName string) (ProductIdentity, bool) {
	if m == nil {
		return ProductIdentity{}, false
	}
	id, ok := m.byName[normalizeName(observedName)]
	return id, ok
}

// IsDeliberatelyUnmapped reports whether an observed name is explicitly listed
// as a non-product token (such as a header, protocol, or CDN).
func (m *ProductMap) IsDeliberatelyUnmapped(observedName string) bool {
	if m == nil {
		return false
	}
	return m.deliberatelyUnmapped[normalizeName(observedName)]
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
