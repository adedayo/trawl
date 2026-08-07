// Package core is Trawl's application layer: every operation the product
// offers, expressed once, independently of how it is invoked.
//
// Trawl ships in two shapes. The desktop build embeds this package behind
// Wails IPC bindings; the container build embeds it behind an HTTP API. Both
// are transports. Neither is allowed to hold behaviour, because the moment one
// does, the two deployments diverge — and the divergence is discovered by an
// operator who finds that the feature they read about in the README exists
// only in the shape of Trawl they are not running.
//
// So the rule this package exists to enforce is simple: an adapter may
// marshal, authenticate and route. It may not decide anything.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/scanner"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
)

// ScopeSettingsKey is the settings key under which the authorisation record is
// persisted. Both transports read the same key, so an operator who authorises
// a portfolio in the desktop app and later runs the container against the same
// database finds the same authorisation in force.
const ScopeSettingsKey = "scope_settings"

// Scope is the operator's declared authorisation to assess a portfolio.
//
// Nothing here is inferred. An asset is in scope because a named person said
// so on a stated date, and that record travels with every assessment as the
// answer to the only question that matters when a scan is questioned: who
// authorised this?
type Scope struct {
	SeedDomainsList []string `json:"seedDomainsList"`
	SeedCidrsList   []string `json:"seedCidrsList"`
	SeedReposList   []string `json:"seedReposList"`

	// ConsentedEndpoints are third-party service hosts the operator permits
	// the assessment to contact. It is held apart from the target scope
	// because consent to assess a domain is not consent to disclose it to a
	// certificate transparency log.
	ConsentedEndpoints []string `json:"consentedEndpoints"`

	// ProposedDomains are domains that certificate evidence suggests belong to
	// the operator, awaiting a decision. They authorise nothing: a proposal is
	// a question put to the operator, and until it is answered the domain is
	// as out of scope as one nobody has heard of.
	ProposedDomains []service.ProposedDomain `json:"proposedDomains"`

	// DismissedDomains are proposals the operator has ruled out. They are kept
	// so that a later discovery pass does not raise them again — a reviewer who
	// has said "not mine" once should not be asked every week.
	DismissedDomains []string `json:"dismissedDomains"`

	IsAuthorized      bool   `json:"isAuthorized"`
	SignerName        string `json:"signerName"`
	SignerTitle       string `json:"signerTitle"`
	AuthorizationDate string `json:"authorizationDate"`
}

// Authorised reports whether an assessment may proceed at all.
//
// An unsigned scope authorises nothing, whatever domains it lists. This is the
// fail-closed default: a scope record that exists but was never signed is a
// draft, and running against a draft is running without authorisation.
func (s Scope) Authorised() bool {
	return s.IsAuthorized && len(s.SeedDomainsList) > 0
}

// Core is the application facade.
type Core struct {
	store store.Store
	bus   event.Bus

	networkScanner *scanner.NetworkScanner
	secretScanner  *scanner.SecretScanner
	emailScanner   *service.EmailScannerService
	assessment     *service.AssessmentService
}

// New builds the application layer.
//
// registryJSON is the embedded signal registry, supplied by the caller so that
// this package makes no assumption about where the binary was started from.
func New(s store.Store, bus event.Bus, registryJSON []byte) (*Core, error) {
	assessment, err := service.NewAssessmentService(s, bus, registryJSON)
	if err != nil {
		return nil, fmt.Errorf("core: %w", err)
	}
	return &Core{
		store:          s,
		bus:            bus,
		networkScanner: scanner.NewNetworkScanner(s, bus),
		secretScanner:  scanner.NewSecretScanner(s, bus),
		emailScanner:   service.NewEmailScannerService(s),
		assessment:     assessment,
	}, nil
}

// Store exposes the underlying store for adapters that need it directly, such
// as the worker ingest path.
func (c *Core) Store() store.Store { return c.store }

// Bus exposes the event bus so a transport can bridge it to its own clients.
func (c *Core) Bus() event.Bus { return c.bus }

// Start performs the one-time work every deployment needs before serving.
func (c *Core) Start(ctx context.Context) error {
	return c.assessment.SyncRegistry(ctx)
}

// ─── Inventory ───────────────────────────────────────────────────────────────

func (c *Core) Assets(ctx context.Context, status store.AssetStatus) ([]store.Asset, error) {
	return c.store.GetAssets(ctx, status)
}

func (c *Core) Findings(ctx context.Context, assetID string) ([]store.Finding, error) {
	return c.store.GetFindings(ctx, assetID)
}

func (c *Core) SecretFindings(ctx context.Context, repoURL string) ([]store.SecretFinding, error) {
	return c.store.GetSecretFindings(ctx, repoURL)
}

func (c *Core) Regressions(ctx context.Context) ([]store.Regression, error) {
	return c.store.GetRegressions(ctx)
}

// RemoveAsset deletes an asset and everything recorded against it.
//
// Discovery is high-fidelity, so an asset does not wait for approval; the only
// operator ruling is removal. Nothing is retained, so a later discovery may
// legitimately present the asset again as new.
func (c *Core) RemoveAsset(ctx context.Context, id string) error {
	if err := c.store.DeleteAsset(ctx, id); err != nil {
		return fmt.Errorf("trawl: removing asset %s: %w", id, err)
	}
	c.bus.Publish(ctx, event.Event{Type: event.EventAssetUpdated})
	return nil
}

// EraseDiscoveredData clears everything the engine observed about the estate,
// preserving the operator's configuration and authorised scope.
//
// The completion is announced on the bus so that every open view refreshes
// from the store. A UI that cleared its own state instead would be asserting
// the erasure succeeded rather than reflecting that it did, and the two differ
// precisely when the erasure failed.
func (c *Core) EraseDiscoveredData(ctx context.Context) error {
	if err := c.store.EraseDiscoveredData(ctx); err != nil {
		return fmt.Errorf("trawl: erasing discovered data: %w", err)
	}
	c.bus.Publish(ctx, event.Event{Type: event.EventDataErased})
	return nil
}

// ─── Settings and scope ──────────────────────────────────────────────────────

func (c *Core) Setting(ctx context.Context, key string) (string, error) {
	return c.store.GetSetting(ctx, key)
}

func (c *Core) SaveSetting(ctx context.Context, key, value string) error {
	return c.store.SaveSetting(ctx, key, value)
}

// Scope reads the authorisation record, failing closed.
//
// An absent, unparseable or unsigned record yields the zero Scope, which
// authorises nothing. The alternative — treating a malformed record as
// permissive — would turn a corrupted settings row into an unbounded scan.
func (c *Core) Scope(ctx context.Context) Scope {
	raw, err := c.store.GetSetting(ctx, ScopeSettingsKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return Scope{}
	}
	var s Scope
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Scope{}
	}
	if !s.IsAuthorized {
		return Scope{}
	}
	return s
}

// SaveScope persists the authorisation record.
func (c *Core) SaveScope(ctx context.Context, s Scope) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("core: encoding the scope: %w", err)
	}
	return c.store.SaveSetting(ctx, ScopeSettingsKey, string(payload))
}

// ─── Email posture (legacy path) ─────────────────────────────────────────────

func (c *Core) EmailPostures(ctx context.Context) ([]store.EmailPosture, error) {
	return c.store.GetEmailPostures(ctx)
}

func (c *Core) ScanEmailPosture(ctx context.Context, domain string) (store.EmailPosture, error) {
	return c.emailScanner.ScanAndSave(ctx, domain)
}

// ─── Measured-state assessment ───────────────────────────────────────────────

func (c *Core) Assessments(ctx context.Context) ([]service.DomainAssessment, error) {
	return c.assessment.Views(ctx)
}

// Assessment returns the stored assessment for one domain.
//
// A domain that has never been assessed yields an empty view rather than an
// error: "nothing has been assessed here" is a legitimate answer, and it is
// the one the UI needs in order to say so.
func (c *Core) Assessment(ctx context.Context, domain string) (service.DomainAssessment, error) {
	assetID := c.assessment.ResolveAssetID(ctx, domain)
	if assetID == "" {
		return service.DomainAssessment{
			Domain:  service.NormaliseDomain(domain),
			Outcome: "not_assessed",
		}, nil
	}
	return c.assessment.View(ctx, assetID)
}

// AssessDomain runs a scope-bounded assessment of one domain.
//
// The scope is read here rather than accepted from the caller. A transport
// that could supply its own scope would be a transport that could widen it,
// and the HTTP API is reachable by anything on the network.
func (c *Core) AssessDomain(ctx context.Context, domain string) (service.DomainAssessment, error) {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return service.DomainAssessment{
			AssetID: service.NormaliseDomain(domain),
			Domain:  service.NormaliseDomain(domain),
			Outcome: "refused",
			Error:   "no signed authorisation is on record; assessment refused",
		}, nil
	}
	return c.assessment.Assess(ctx, domain, scope.SeedDomainsList, scope.ConsentedEndpoints)
}

// ─── Orchestration ───────────────────────────────────────────────────────────

// ScanRequest is one run across the capabilities the operator asked for.
type ScanRequest struct {
	Domain  string `json:"domain"`
	RepoURL string `json:"repoUrl"`
}

// RunScan performs discovery, assessment and secret scanning concurrently and
// blocks until all of them finish.
//
// It is synchronous because a caller that wants it in the background can say
// so; a function that is always asynchronous cannot be made to wait, and the
// HTTP transport needs to wait in order to report a result.
//
// One capability failing never cancels the others. A domain whose nameservers
// are down must not deny the operator their repository findings.
func (c *Core) RunScan(ctx context.Context, req ScanRequest) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error

	record := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		failures = append(failures, err)
		mu.Unlock()
	}

	if req.Domain != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.networkScanner.DiscoverSubdomains(ctx, req.Domain); err != nil {
				record(fmt.Errorf("discovery: %w", err))
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.AssessDomain(ctx, req.Domain); err != nil {
				record(fmt.Errorf("assessment: %w", err))
			}
		}()

		// The legacy email path is retained until the Change 006 Phase 9 gap
		// analysis closes. Its failure is recorded rather than ignored.
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.emailScanner.ScanAndSave(ctx, req.Domain); err != nil {
				record(fmt.Errorf("email posture: %w", err))
			}
		}()
	}

	if req.RepoURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.secretScanner.ScanRepo(ctx, req.RepoURL); err != nil {
				record(fmt.Errorf("secret scan: %w", err))
			}
		}()
	}

	wg.Wait()

	if len(failures) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(failures))
	for _, e := range failures {
		msgs = append(msgs, e.Error())
	}
	// Every failure is named. A summary saying only that "the scan failed"
	// leaves the operator unable to tell a partial result from no result.
	return fmt.Errorf("scan completed with failures: %s", strings.Join(msgs, "; "))
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

func (c *Core) Jobs(ctx context.Context, status store.JobStatus) ([]store.Job, error) {
	return c.store.GetJobs(ctx, status)
}

func (c *Core) EnqueueJob(ctx context.Context, jobType string, targets []string) (*store.Job, error) {
	return c.store.EnqueueJob(ctx, jobType, targets)
}

func (c *Core) PopJob(ctx context.Context, jobType string) (*store.Job, error) {
	return c.store.PopJob(ctx, jobType)
}

func (c *Core) CompleteJob(ctx context.Context, jobID string, status store.JobStatus, errMsg string) error {
	return c.store.CompleteJob(ctx, jobID, status, errMsg)
}
