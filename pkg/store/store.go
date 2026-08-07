package store

import (
	"context"
	"time"
)

// AssetType defines the category of asset tracked by Trawl.
type AssetType string

const (
	AssetTypeDomain     AssetType = "domain"
	AssetTypeSubdomain  AssetType = "subdomain"
	AssetTypeIP         AssetType = "ip"
	AssetTypeCIDR       AssetType = "cidr"
	AssetTypeRepository AssetType = "repository"
)

// AssetStatus defines the lifecycle status of an asset.
//
// There is only one status. Discovery is high-fidelity, so an asset is not
// held pending approval, and the operator's only ruling is removal, which
// deletes the record rather than archiving it. The field is kept as a named
// type so that a future state has somewhere to go, but naming states nothing
// ever sets would imply a review workflow that does not exist.
type AssetStatus string

const (
	AssetStatusActive AssetStatus = "active"
)

// Asset represents a tracked attack-surface asset.
type Asset struct {
	ID              string      `json:"id"`
	Type            AssetType   `json:"type"`
	Value           string      `json:"value"`
	Status          AssetStatus `json:"status"`
	DiscoverySource string      `json:"discoverySource"`
	Confidence      float64     `json:"confidence"`
	FirstSeen       time.Time   `json:"firstSeen"`
	LastSeen        time.Time   `json:"lastSeen"`
	Metadata        string      `json:"metadata"` // JSON string
}

// FindingSeverity defines the risk rating of a security finding.
type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityHigh     FindingSeverity = "high"
	SeverityMedium   FindingSeverity = "medium"
	SeverityLow      FindingSeverity = "low"
	SeverityInfo     FindingSeverity = "info"
)

// Rank orders severities so they can be compared and maximised. An unknown or
// empty severity ranks below info: it is the absence of a rating, and must not
// sort above one that was actually assigned.
func (s FindingSeverity) Rank() int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// Significant reports whether a severity is high enough to warrant attention
// on its own. The line sits at medium because that is where vantage's
// catalogue stops describing preferences and starts describing weaknesses.
//
// This is a presentation threshold, not a filter: nothing below it is hidden
// or discarded anywhere. It exists only so that an aggregate can distinguish
// "one control has a problem" from "one control has a preference recorded
// against it", which the posture model alone cannot express.
func (s FindingSeverity) Significant() bool { return s.Rank() >= SeverityMedium.Rank() }

// Finding represents a vulnerability or posture finding.
type Finding struct {
	ID           string          `json:"id"`
	AssetID      string          `json:"assetId"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Severity     FindingSeverity `json:"severity"`
	Priority     string          `json:"priority"`
	CVE          string          `json:"cve,omitempty"`
	EPSS         float64         `json:"epss,omitempty"`
	KEVListed    bool            `json:"kevListed"`
	Category     string          `json:"category"`
	Proof        string          `json:"proof"`
	AIAnnotation string          `json:"aiAnnotation,omitempty"`
	FirstSeen    time.Time       `json:"firstSeen"`
	LastSeen     time.Time       `json:"lastSeen"`
}

// SecretFinding represents exposed credentials or secrets scanned by CheckMate.
type SecretFinding struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"assetId"`
	RepoURL     string    `json:"repoUrl"`
	RuleID      string    `json:"ruleId"`
	SecretType  string    `json:"secretType"`
	RedactedRef string    `json:"redactedRef"` // Redacted / hashed secret reference
	FilePath    string    `json:"filePath"`
	StartLine   int       `json:"startLine"`
	Verified    bool      `json:"verified"`
	IsReused    bool      `json:"isReused"`
	FirstSeen   time.Time `json:"firstSeen"`
}

// Regression represents a confirmed posture degradation between checks.
type Regression struct {
	ID               string    `json:"id"`
	AssetID          string    `json:"assetId"`
	AttributeType    string    `json:"attributeType"` // e.g. "tls_version", "dmarc_policy"
	PreviousValue    string    `json:"previousValue"`
	CurrentValue     string    `json:"currentValue"`
	ConsecutiveFails int       `json:"consecutiveFails"`
	ConfirmedAt      time.Time `json:"confirmedAt"`
}

// EmailPosture represents the email authentication posture for a domain.
type EmailPosture struct {
	Domain      string    `json:"domain"`
	SPFValid    bool      `json:"spfValid"`
	DKIMFound   bool      `json:"dkimFound"`
	DMARCPolicy string    `json:"dmarcPolicy"`
	Priority    string    `json:"priority"`
	LastChecked time.Time `json:"lastChecked"`
	MTAStsFound bool      `json:"mtaStsFound"`
	MTAStsMode  string    `json:"mtaStsMode"`
	DNSSECValid bool      `json:"dnssecValid"`
	DANEValid   bool      `json:"daneValid"`
}

// JobStatus defines the lifecycle state of a queued worker job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job is a unit of work claimed by an external worker container.
//
// The JSON tag on ID is `_id` for wire compatibility with the worker
// entrypoints, which were written against a document-store shape.
type Job struct {
	ID          string     `json:"_id"`
	Type        string     `json:"type"` // "discovery" | "scan" | "secret_scan"
	Status      JobStatus  `json:"status"`
	Targets     []string   `json:"targets"`
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Store defines the interface for Trawl's universal storage engine.
type Store interface {
	// Assets
	GetAssets(ctx context.Context, status AssetStatus) ([]Asset, error)
	GetAssetByID(ctx context.Context, id string) (*Asset, error)
	SaveAsset(ctx context.Context, asset *Asset) error

	// DeleteAsset removes an asset and everything recorded against it.
	//
	// Discovery is high-fidelity, so an asset is not held for approval;
	// deletion is the operator's only ruling. The record is discarded
	// entirely, so a later discovery may legitimately surface it again as new.
	DeleteAsset(ctx context.Context, id string) error

	// Findings
	GetFindings(ctx context.Context, assetID string) ([]Finding, error)
	SaveFinding(ctx context.Context, finding *Finding) error

	// Secret Findings
	GetSecretFindings(ctx context.Context, repoURL string) ([]SecretFinding, error)
	SaveSecretFinding(ctx context.Context, sf *SecretFinding) error

	// Regressions
	GetRegressions(ctx context.Context) ([]Regression, error)
	RecordPostureObservation(ctx context.Context, assetID string, attributeType string, value string) (*Regression, error)

	// Email Posture
	GetEmailPostures(ctx context.Context) ([]EmailPosture, error)
	SaveEmailPosture(ctx context.Context, ep *EmailPosture) error

	// Measured-state signals and assessment coverage.
	//
	// SaveSignalObservation upserts on (assetId, signalId), preserving
	// FirstSeen. ReplaceSignalRegistry swaps the registry atomically so that
	// no observation is ever mapped against a half-loaded registry.
	SaveSignalObservation(ctx context.Context, obs *SignalObservation) error
	GetSignalObservations(ctx context.Context, assetID string) ([]SignalObservation, error)
	ReplaceSignalRegistry(ctx context.Context, entries []SignalRegistryEntry) error
	GetSignalRegistry(ctx context.Context) ([]SignalRegistryEntry, error)
	GetSignalRegistryEntry(ctx context.Context, signalID string) (*SignalRegistryEntry, error)
	RecordAssessmentCoverage(ctx context.Context, cov *AssessmentCoverage) error
	GetAssessmentCoverage(ctx context.Context, assetID string) ([]AssessmentCoverage, error)
	ComputeCoverage(ctx context.Context, assetID string) (CoverageSummary, error)

	// Assessment runs.
	//
	// RecordAssessmentRun upserts on assetId: only the latest run is kept.
	// GetAssessmentRuns follows the coverage convention, where an empty
	// assetId means every asset, so a portfolio view fetches once.
	RecordAssessmentRun(ctx context.Context, run *AssessmentRun) error
	GetAssessmentRuns(ctx context.Context, assetID string) ([]AssessmentRun, error)

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SaveSetting(ctx context.Context, key string, value string) error

	// EraseDiscoveredData removes everything the engine observed about the
	// estate — assets and everything hanging off them, email postures, and any
	// queued or completed work — while preserving what the operator
	// configured: settings, authorised scope, and the signal registry.
	//
	// It is one call rather than a sequence the caller composes, because the
	// erasure must be atomic. A partial wipe leaves findings attached to
	// assets that no longer exist, and an operator who asked to start over
	// would be left with a state neither they nor the engine can account for.
	EraseDiscoveredData(ctx context.Context) error

	// Job queue
	//
	// PopJob atomically claims the oldest pending job of the given type and
	// transitions it to running. It returns (nil, nil) when the queue is empty
	// — an empty queue is the normal steady state, not an error.
	EnqueueJob(ctx context.Context, jobType string, targets []string) (*Job, error)
	PopJob(ctx context.Context, jobType string) (*Job, error)
	CompleteJob(ctx context.Context, jobID string, status JobStatus, errMsg string) error
	GetJobs(ctx context.Context, status JobStatus) ([]Job, error)

	// Lifecycle & Health
	Close() error
}
