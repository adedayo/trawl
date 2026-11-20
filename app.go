package main

import (
	"context"
	"log"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/core"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/version"
	vprobe "github.com/adedayo/vantage/pkg/probe"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails transport.
//
// It holds no behaviour. Every method here forwards to pkg/core, which is the
// same application layer the container deployment serves over HTTP. Anything
// implemented in this file rather than in core would be a feature the desktop
// build has and the server build silently lacks.
type App struct {
	ctx  context.Context
	core *core.Core
}

// NewApp creates a new App application struct.
func NewApp(s store.Store, eb event.Bus, registryJSON []byte) (*App, error) {
	c, err := core.New(s, eb, registryJSON)
	if err != nil {
		return nil, err
	}
	return &App{core: c}, nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.core.Start(ctx); err != nil {
		log.Printf("trawl: could not seed the signal registry: %v", err)
	}

	// Wire Event Bus to Wails IPC
	bus := a.core.Bus()
	bus.Subscribe(event.EventAssetUpdated, a.emitToWails)
	bus.Subscribe(event.EventFindingNew, a.emitToWails)
	bus.Subscribe(event.EventScanProgress, a.emitToWails)
	bus.Subscribe(event.EventRegressionNew, a.emitToWails)
}

func (a *App) emitToWails(ctx context.Context, e event.Event) {
	// runtime.EventsEmit pushes the event to the Angular frontend over Wails IPC
	runtime.EventsEmit(a.ctx, string(e.Type), e.Payload)
}

// --- API Methods exposed to Angular ---

// GetVersion reports the build identity of the running application.
//
// This is bound rather than baked into the frontend bundle at build time
// because the two are versioned by the same tag but built by different
// toolchains, and a UI that reports a version the binary does not have is a
// support hazard rather than a convenience.
func (a *App) GetVersion() version.Info {
	return version.Get()
}

func (a *App) GetAssets(status store.AssetStatus) ([]store.Asset, error) {
	return a.core.Assets(a.ctx, status)
}

// RemoveAsset deletes an asset and everything recorded against it.
func (a *App) RemoveAsset(id string) error {
	return a.core.RemoveAsset(a.ctx, id)
}

func (a *App) GetFindings(assetID string) ([]store.Finding, error) {
	return a.core.Findings(a.ctx, assetID)
}

// GetFindingIndex lists findings across every source (Change 025).
func (a *App) GetFindingIndex(filter service.FindingFilter) ([]service.FindingSummary, error) {
	return a.core.FindingIndex(a.ctx, filter)
}

// GetFindingDetail returns the drill-down for one finding.
func (a *App) GetFindingDetail(key string) (service.FindingDetail, error) {
	return a.core.FindingDetail(a.ctx, key)
}

// GetFindingChanges reports what moved between two runs; empty ids mean the
// latest run and the run before it.
func (a *App) GetFindingChanges(from string, to string) (service.ChangeSet, error) {
	return a.core.FindingChanges(a.ctx, from, to)
}

// GetScanRuns lists the run ledger, newest first.
func (a *App) GetScanRuns() ([]service.ScanRunView, error) {
	return a.core.ScanRuns(a.ctx)
}

// GetHistorySummary returns one point per recorded run from since (RFC 3339).
func (a *App) GetHistorySummary(since string) ([]service.HistoryPoint, error) {
	return a.core.HistorySummary(a.ctx, since)
}

func (a *App) GetSecretFindings(repoURL string) ([]store.SecretFinding, error) {
	return a.core.SecretFindings(a.ctx, repoURL)
}

func (a *App) GetEmailPostures() ([]store.EmailPosture, error) {
	return a.core.EmailPostures(a.ctx)
}

func (a *App) ScanEmailPosture(domain string) (store.EmailPosture, error) {
	return a.core.ScanEmailPosture(a.ctx, domain)
}

// --- Measured-state assessment (vantage) ---

// GetDomainAssessments returns the stored assessment for every assessed domain.
func (a *App) GetDomainAssessments() ([]service.DomainAssessment, error) {
	return a.core.Assessments(a.ctx)
}

// GetDomainAssessment returns the stored assessment for one domain.
func (a *App) GetDomainAssessment(domain string) (service.DomainAssessment, error) {
	return a.core.Assessment(a.ctx, domain)
}

// AssessDomain runs a scope-bounded assessment and returns the fresh view.
func (a *App) AssessDomain(domain string) (service.DomainAssessment, error) {
	return a.core.AssessDomain(a.ctx, domain)
}

// AssessDomainWithOptions runs a scope-bounded assessment with explicit
// Vantage external discovery and/or opt-in dangling cloud-address identity
// probing (`dangle`).
func (a *App) AssessDomainWithOptions(domain string, opts service.AssessOptions) (service.DomainAssessment, error) {
	return a.core.AssessDomainWithOptions(a.ctx, domain, opts)
}

// ProbeDiscoveredServices runs a bounded Vantage service discovery pass over
// tracked, authorised assets.
func (a *App) ProbeDiscoveredServices(profile string) error {
	if profile == "" {
		profile = string(vprobe.DiscoveryMostCommon)
	}
	return a.core.ProbeDiscoveredServices(a.ctx, vprobe.DiscoveryProfile(profile))
}

func (a *App) GetRegressions() ([]store.Regression, error) {
	return a.core.Regressions(a.ctx)
}

func (a *App) GetCalibrationReport(experimentID string, confidence float64) ([]contact.CalibrationReportRow, error) {
	return a.core.CalibrationReport(a.ctx, experimentID, confidence)
}

func (a *App) GetCalibrationExperiments() ([]store.CalibrationExperiment, error) {
	return a.core.CalibrationExperiments(a.ctx)
}

func (a *App) CreateCalibrationExperiment(label string) (*store.CalibrationExperiment, error) {
	return a.core.CreateCalibrationExperiment(a.ctx, label)
}

func (a *App) GetFeedStatuses() ([]store.FeedStatus, error) {
	return a.core.FeedStatuses(a.ctx)
}

func (a *App) RefreshFeeds(epssThreshold float64) ([]store.FeedStatus, error) {
	return a.core.RefreshFeeds(a.ctx, nil, epssThreshold)
}

// RefreshFeed refreshes one catalogue (cisa-kev, epss or nvd) without
// fetching the others.
func (a *App) RefreshFeed(feedName string, epssThreshold float64) ([]store.FeedStatus, error) {
	return a.core.RefreshNamedFeeds(a.ctx, []string{feedName}, nil, epssThreshold)
}

func (a *App) GetAssetSoftware(assetID string) ([]store.AssetSoftware, error) {
	return a.core.AssetSoftware(a.ctx, assetID)
}

func (a *App) GetSoftwareCoverage(assetID string) ([]store.SoftwareCoverage, error) {
	return a.core.SoftwareCoverage(a.ctx, assetID)
}

func (a *App) RefreshCandidates(force bool) (*service.CandidateRefreshSummary, error) {
	return a.core.RefreshCandidates(a.ctx, nil, force)
}

func (a *App) GetAssetContactPanel(assetID string, excludedFactors []string) (service.AssetContactPanel, error) {
	return a.core.AssetContactPanel(a.ctx, assetID, excludedFactors)
}

func (a *App) GetWeaknessContactAggregations() ([]contact.InstanceAggregation, error) {
	return a.core.WeaknessContactAggregations(a.ctx)
}

func (a *App) GetSetting(key string) (string, error) {
	return a.core.Setting(a.ctx, key)
}

func (a *App) SaveSetting(key string, value string) error {
	return a.core.SaveSetting(a.ctx, key, value)
}

// EraseDiscoveredData clears everything the engine discovered, preserving the
// operator's configuration and authorised scope.
//
// It returns the error rather than swallowing it. This is a destructive action
// the operator is told succeeded, and reporting success for a wipe that failed
// would leave them believing the estate was cleared while every finding
// remained.
func (a *App) EraseDiscoveredData() error {
	return a.core.EraseDiscoveredData(a.ctx)
}

func (a *App) DiscoverRelatedDomains(opts service.DiscoveryOptions) (service.DiscoveryResult, error) {
	return a.core.DiscoverRelatedDomains(a.ctx, opts)
}

func (a *App) AuthoriseProposedDomains(domains []string) error {
	return a.core.AuthoriseProposedDomains(a.ctx, domains)
}

func (a *App) DismissProposedDomains(domains []string) error {
	return a.core.DismissProposedDomains(a.ctx, domains)
}

func (a *App) RestoreDismissedDomain(domain string) error {
	return a.core.RestoreDismissedDomain(a.ctx, domain)
}

func (a *App) GetDismissedDomains() []string {
	return a.core.DismissedDomains(a.ctx)
}

// TriggerScan starts a scan in the background and returns immediately, so the
// desktop window stays responsive. Completion is announced on the event bus,
// carrying how the scan ended.
//
// The outcome travels with the event rather than being logged and dropped. A
// desktop user has no terminal to read, so an error swallowed into a log line
// leaves the UI unable to tell a scan that failed from one that succeeded —
// and it settles on the reassuring reading.
func (a *App) TriggerScan(domain string, repoURL string) error {
	go func() {
		req := core.ScanRequest{Domain: domain, RepoURL: repoURL}
		err := a.core.RunScan(a.ctx, req)
		if err != nil {
			log.Printf("trawl: %v", err)
		}
		runtime.EventsEmit(a.ctx, "scan:complete", core.NewScanOutcome(req, err))
	}()
	return nil
}
