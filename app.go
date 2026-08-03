package main

import (
	"context"
	"sync"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/scanner"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx            context.Context
	store          store.Store
	eventBus       event.Bus
	networkScanner *scanner.NetworkScanner
	secretScanner  *scanner.SecretScanner
	emailScanner   *service.EmailScannerService
}

// NewApp creates a new App application struct
func NewApp(s store.Store, eb event.Bus) *App {
	return &App{
		store:          s,
		eventBus:       eb,
		networkScanner: scanner.NewNetworkScanner(s, eb),
		secretScanner:  scanner.NewSecretScanner(s, eb),
		emailScanner:   service.NewEmailScannerService(s),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire Event Bus to Wails IPC
	a.eventBus.Subscribe(event.EventAssetUpdated, a.emitToWails)
	a.eventBus.Subscribe(event.EventFindingNew, a.emitToWails)
	a.eventBus.Subscribe(event.EventScanProgress, a.emitToWails)
	a.eventBus.Subscribe(event.EventRegressionNew, a.emitToWails)
}

func (a *App) emitToWails(ctx context.Context, e event.Event) {
	// runtime.EventsEmit pushes the event to the Angular frontend over Wails IPC
	runtime.EventsEmit(a.ctx, string(e.Type), e.Payload)
}

// --- API Methods exposed to Angular ---

func (a *App) GetAssets(status store.AssetStatus) ([]store.Asset, error) {
	return a.store.GetAssets(a.ctx, status)
}

func (a *App) GetFindings(assetID string) ([]store.Finding, error) {
	return a.store.GetFindings(a.ctx, assetID)
}

func (a *App) GetSecretFindings(repoURL string) ([]store.SecretFinding, error) {
	return a.store.GetSecretFindings(a.ctx, repoURL)
}

func (a *App) GetEmailPostures() ([]store.EmailPosture, error) {
	return a.store.GetEmailPostures(a.ctx)
}

func (a *App) ScanEmailPosture(domain string) (store.EmailPosture, error) {
	return a.emailScanner.ScanAndSave(a.ctx, domain)
}

func (a *App) GetRegressions() ([]store.Regression, error) {
	return a.store.GetRegressions(a.ctx)
}

func (a *App) GetSetting(key string) (string, error) {
	return a.store.GetSetting(a.ctx, key)
}

func (a *App) SaveSetting(key string, value string) error {
	return a.store.SaveSetting(a.ctx, key, value)
}

func (a *App) TriggerScan(domain string, repoURL string) error {
	// Execute background scan using NetworkScanner, SecretScanner and EmailScanner asynchronously and in parallel
	go func() {
		var wg sync.WaitGroup

		if domain != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = a.networkScanner.DiscoverSubdomains(a.ctx, domain)
			}()

			// Automatic email posture scan
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = a.emailScanner.ScanAndSave(a.ctx, domain)
			}()
		}

		if repoURL != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = a.secretScanner.ScanRepo(a.ctx, repoURL)
			}()
		}

		// Wait for all scanners to finish
		wg.Wait()

		// Notify frontend that the scan is complete
		runtime.EventsEmit(a.ctx, "scan:complete", nil)
	}()
	return nil
}
