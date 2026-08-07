package main

import (
	"context"
	"log"

	"github.com/adedayo/trawl/pkg/core"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
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

func (a *App) GetRegressions() ([]store.Regression, error) {
	return a.core.Regressions(a.ctx)
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
