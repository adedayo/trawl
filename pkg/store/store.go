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
type AssetStatus string

const (
	AssetStatusActive   AssetStatus = "active"
	AssetStatusPending  AssetStatus = "pending"
	AssetStatusArchived AssetStatus = "archived"
)

// Asset represents a tracked attack-surface asset.
type Asset struct {
	ID             string      `json:"id"`
	Type           AssetType   `json:"type"`
	Value          string      `json:"value"`
	Status         AssetStatus `json:"status"`
	DiscoverySource string     `json:"discoverySource"`
	Confidence     float64     `json:"confidence"`
	FirstSeen      time.Time   `json:"firstSeen"`
	LastSeen       time.Time   `json:"lastSeen"`
	Metadata       string      `json:"metadata"` // JSON string
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

// Finding represents a vulnerability or posture finding.
type Finding struct {
	ID             string          `json:"id"`
	AssetID        string          `json:"assetId"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Severity       FindingSeverity `json:"severity"`
	Priority       string          `json:"priority"`
	CVE            string          `json:"cve,omitempty"`
	EPSS           float64         `json:"epss,omitempty"`
	KEVListed      bool            `json:"kevListed"`
	Category       string          `json:"category"`
	Proof          string          `json:"proof"`
	AIAnnotation   string          `json:"aiAnnotation,omitempty"`
	FirstSeen      time.Time       `json:"firstSeen"`
	LastSeen       time.Time       `json:"lastSeen"`
}

// SecretFinding represents exposed credentials or secrets scanned by CheckMate.
type SecretFinding struct {
	ID           string    `json:"id"`
	AssetID      string    `json:"assetId"`
	RepoURL      string    `json:"repoUrl"`
	RuleID       string    `json:"ruleId"`
	SecretType   string    `json:"secretType"`
	RedactedRef  string    `json:"redactedRef"` // Redacted / hashed secret reference
	FilePath     string    `json:"filePath"`
	StartLine    int       `json:"startLine"`
	Verified     bool      `json:"verified"`
	IsReused     bool      `json:"isReused"`
	FirstSeen    time.Time `json:"firstSeen"`
}

// Regression represents a confirmed posture degradation between checks.
type Regression struct {
	ID             string    `json:"id"`
	AssetID        string    `json:"assetId"`
	AttributeType  string    `json:"attributeType"` // e.g. "tls_version", "dmarc_policy"
	PreviousValue  string    `json:"previousValue"`
	CurrentValue   string    `json:"currentValue"`
	ConsecutiveFails int     `json:"consecutiveFails"`
	ConfirmedAt    time.Time `json:"confirmedAt"`
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

// Store defines the interface for Trawl's universal storage engine.
type Store interface {
	// Assets
	GetAssets(ctx context.Context, status AssetStatus) ([]Asset, error)
	GetAssetByID(ctx context.Context, id string) (*Asset, error)
	SaveAsset(ctx context.Context, asset *Asset) error

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

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SaveSetting(ctx context.Context, key string, value string) error

	// Lifecycle & Health
	Close() error
}
