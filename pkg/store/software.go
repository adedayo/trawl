package store

import (
	"context"
	"time"
)

// SoftwareMappingState describes whether an observed technology name matched
// an entry in the curated product map.
type SoftwareMappingState string

const (
	SoftwareMapped   SoftwareMappingState = "mapped"
	SoftwareUnmapped SoftwareMappingState = "unmapped"
)

// SoftwareVersionState distinguishes an observed version from an absent one.
// An unknown version is never guessed or defaulted.
type SoftwareVersionState string

const (
	SoftwareVersionKnown   SoftwareVersionState = "known"
	SoftwareVersionUnknown SoftwareVersionState = "unknown"
)

// SoftwareStatus tracks whether an observed software record is the asset's
// current version or has been superseded by a newer observation.
type SoftwareStatus string

const (
	SoftwareCurrent    SoftwareStatus = "current"
	SoftwareSuperseded SoftwareStatus = "superseded"
)

// SoftwareCoverageState describes what the engine knows about an asset's
// software stack. Absence of software evidence is never presented as zero
// vulnerabilities.
type SoftwareCoverageState string

const (
	SoftwareCoverageNotAssessed    SoftwareCoverageState = "not_assessed"
	SoftwareCoverageSeenUnmapped   SoftwareCoverageState = "seen_unmapped"
	SoftwareCoverageVersionUnknown SoftwareCoverageState = "version_unknown"
	SoftwareCoverageIdentified     SoftwareCoverageState = "identified"
)

// AssetSoftware is one structured software observation on an asset.
type AssetSoftware struct {
	ID             string               `json:"id"`
	AssetID        string               `json:"assetId"`
	ObservedName   string               `json:"observedName"`
	CPEPart        string               `json:"cpePart,omitempty"`
	Vendor         string               `json:"vendor,omitempty"`
	Product        string               `json:"product,omitempty"`
	MappingState   SoftwareMappingState `json:"mappingState"`
	MappingVersion string               `json:"mappingVersion,omitempty"`
	Version        string               `json:"version,omitempty"`
	VersionState   SoftwareVersionState `json:"versionState"`
	Source         string               `json:"source"`
	RawString      string               `json:"rawString"`
	Status         SoftwareStatus       `json:"status"`
	FirstSeen      time.Time            `json:"firstSeen"`
	LastSeen       time.Time            `json:"lastSeen"`
	SupersededAt   *time.Time           `json:"supersededAt,omitempty"`
}

// AssetSoftwareID returns the deterministic identity of a software observation
// on an asset.
func AssetSoftwareID(assetID, observedName, version, source string) string {
	return "sw:" + shortDigest(assetID, observedName, version, source)
}

// SoftwareCoverage summarises what is known about an asset's software.
type SoftwareCoverage struct {
	AssetID        string                `json:"assetId"`
	State          SoftwareCoverageState `json:"state"`
	Identified     int                   `json:"identified"`
	VersionUnknown int                   `json:"versionUnknown"`
	Unmapped       int                   `json:"unmapped"`
	Superseded     int                   `json:"superseded"`
	// Reason explains why software could not be assessed when State is
	// not_assessed (for example: no service probe has run yet, probed services
	// did not respond, probes timed out, or responding services disclosed no
	// software headers/banners).
	Reason string `json:"reason,omitempty"`
}

// CandidateCVE is one CVE returned by NVD for a specific (cpePart, vendor,
// product, version) lookup.
type CandidateCVE struct {
	CVE         string          `json:"cve"`
	Severity    FindingSeverity `json:"severity"`
	Description string          `json:"description,omitempty"`
	Published   string          `json:"published,omitempty"`
}

// SoftwareLookupCache caches the NVD CVE list for one (cpePart, vendor,
// product, version) tuple alongside the snapshot that produced it.
type SoftwareLookupCache struct {
	ID          string         `json:"id"`
	CPEPart     string         `json:"cpePart"`
	Vendor      string         `json:"vendor"`
	Product     string         `json:"product"`
	Version     string         `json:"version"`
	SnapshotID  string         `json:"snapshotId"`
	Fingerprint string         `json:"fingerprint"`
	QueriedAt   time.Time      `json:"queriedAt"`
	CVEs        []CandidateCVE `json:"cves"`
}

// SoftwareLookupKey returns the cache key for one (cpePart, vendor, product, version).
func SoftwareLookupKey(cpePart, vendor, product, version string) string {
	if cpePart == "" {
		cpePart = "a"
	}
	return cpePart + ":" + vendor + ":" + product + ":" + version
}

// CandidateFinding is an unconfirmed CVE matched from an asset's observed
// software rather than demonstrated by a scanner template. It is stored apart
// from confirmed findings so the two cannot be conflated.
type CandidateFinding struct {
	ID                  string              `json:"id"`
	AssetID             string              `json:"assetId"`
	SoftwareID          string              `json:"softwareId"`
	Vendor              string              `json:"vendor"`
	Product             string              `json:"product"`
	Version             string              `json:"version"`
	RawString           string              `json:"rawString"`
	CVE                 string              `json:"cve"`
	Title               string              `json:"title"`
	Description         string              `json:"description,omitempty"`
	Severity            FindingSeverity     `json:"severity"`
	SeveritySource      string              `json:"severitySource"`
	Priority            string              `json:"priority"`
	EPSS                float64             `json:"epss,omitempty"`
	KEVListed           bool                `json:"kevListed"`
	Status              FindingStatus       `json:"status"`
	NotReassessed       bool                `json:"notReassessed"`
	NotReassessedReason string              `json:"notReassessedReason,omitempty"`
	LookupSnapshotID    string              `json:"lookupSnapshotId"`
	LookupAt            time.Time           `json:"lookupAt"`
	FirstSeen           time.Time           `json:"firstSeen"`
	LastSeen            time.Time           `json:"lastSeen"`
	ResolvedAt          *time.Time          `json:"resolvedAt,omitempty"`
	Enrichments         []FindingEnrichment `json:"enrichments,omitempty"`
}

// SoftwareStore persists structured per-asset software inventory, NVD product
// lookup caches, and unconfirmed candidate findings.
type SoftwareStore interface {
	// SaveAssetSoftware upserts one software record. If a current record for
	// the same (asset, product) or (asset, observedName) exists with a
	// different version, it is marked superseded rather than overwritten.
	SaveAssetSoftware(ctx context.Context, sw *AssetSoftware) error
	// GetAssetSoftware returns software records for one asset (or all assets
	// when assetID is empty), including superseded versions.
	GetAssetSoftware(ctx context.Context, assetID string) ([]AssetSoftware, error)
	// ComputeSoftwareCoverage summarises software visibility for one asset.
	ComputeSoftwareCoverage(ctx context.Context, assetID string) (SoftwareCoverage, error)

	// NVD product/version lookup cache.
	GetSoftwareLookupCache(ctx context.Context, key string) (*SoftwareLookupCache, error)
	SaveSoftwareLookupCache(ctx context.Context, entry *SoftwareLookupCache) error

	// Candidate findings (single write path with finding_events).
	SaveCandidateFinding(ctx context.Context, cf *CandidateFinding) error
	GetCandidateFindings(ctx context.Context, assetID string) ([]CandidateFinding, error)
	GetCandidateEnrichments(ctx context.Context, candidateID string) ([]FindingEnrichment, error)
	SaveCandidateEnrichment(ctx context.Context, enrichment *FindingEnrichment) error
	MarkAssetCandidatesNotReassessed(ctx context.Context, assetID, reason string) error
}
