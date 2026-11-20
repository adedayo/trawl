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
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/narrative"
	"github.com/adedayo/trawl/pkg/scanner"
	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	vprobe "github.com/adedayo/vantage/pkg/probe"
	"github.com/google/uuid"
)

// NVDAPIKeySetting is the optional settings key holding an NVD 2.0 API key.
// With a key, NVD raises the rate limit from 5 to 50 requests per 30 seconds.
const NVDAPIKeySetting = "nvd_api_key"

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
	feedIngest     *service.FeedIngestService
	feedMu         sync.Mutex
	findings       *service.FindingsService
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
	// The explanation text ships inside the binary. One that fails to load is a
	// build defect, so it stops start-up rather than leaving every finding
	// unexplained.
	explainer, err := narrative.Load()
	if err != nil {
		return nil, fmt.Errorf("core: loading the finding narratives: %w", err)
	}
	return &Core{
		store:          s,
		bus:            bus,
		networkScanner: scanner.NewNetworkScanner(s, bus),
		secretScanner:  scanner.NewSecretScanner(s, bus),
		emailScanner:   service.NewEmailScannerService(s),
		assessment:     assessment,
		feedIngest:     service.NewFeedIngestService(s),
		findings:       service.NewFindingsService(s, explainer),
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

func (c *Core) CalibrationReport(ctx context.Context, experimentID string, confidence float64) ([]contact.CalibrationReportRow, error) {
	if experimentID == "" {
		return nil, fmt.Errorf("calibration experiment is required")
	}
	observations, err := c.store.GetCalibrationObservations(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	intervals, err := c.store.GetCalibrationReachability(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	contactObservations := make([]contact.CalibrationObservation, 0, len(observations))
	for _, observation := range observations {
		contactObservations = append(contactObservations, contact.CalibrationObservation{
			ID: observation.ID, ExperimentID: observation.ExperimentID, EndpointID: observation.EndpointID,
			ServiceClass: contact.ServiceClass(observation.ServiceClass), Transport: observation.Transport,
			Port: observation.Port, ObservedAt: observation.ObservedAt,
			EventType: contact.CalibrationEventType(observation.EventType), SourceKey: contact.SourceKey(observation.SourceKey),
			Accepted: observation.Accepted, ProtocolIdentified: observation.ProtocolIdentified,
		})
	}
	contactIntervals := make([]contact.ReachabilityInterval, 0, len(intervals))
	for _, interval := range intervals {
		contactIntervals = append(contactIntervals, contact.ReachabilityInterval{
			ExperimentID: interval.ExperimentID, EndpointID: interval.EndpointID,
			ServiceClass: contact.ServiceClass(interval.ServiceClass), StartedAt: interval.StartedAt, EndedAt: interval.EndedAt,
		})
	}
	buckets, err := contact.AggregateCalibration(contactObservations, contactIntervals)
	if err != nil {
		return nil, err
	}
	rows := make([]contact.CalibrationReportRow, 0, len(buckets))
	for key, bucket := range buckets {
		rate, err := bucket.Rate()
		if err != nil {
			return nil, err
		}
		low, high, err := contact.PoissonRateInterval(bucket.EventCount, bucket.ReachableServiceDays, confidence)
		if err != nil {
			return nil, err
		}
		rows = append(rows, contact.CalibrationReportRow{
			ExperimentID: experimentID, ServiceClass: string(key.ServiceClass), EventType: string(key.EventType),
			EventCount: bucket.EventCount, ReachableServiceDays: bucket.ReachableServiceDays, EndpointCount: bucket.EndpointCount,
			UniqueSourceCount: bucket.UniqueSourceCount, RatePerServiceDay: rate, RateLowPerServiceDay: low, RateHighPerServiceDay: high,
			IntervalMethod: "garwood-poisson", Confidence: confidence,
			ObservationStart: bucket.ObservationStart.UTC().Format(time.RFC3339), ObservationEnd: bucket.ObservationEnd.UTC().Format(time.RFC3339),
			DeduplicationRule: "event count is not deduplicated; uniqueSourceCount uses keyed sourceKey",
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ServiceClass == rows[j].ServiceClass {
			return rows[i].EventType < rows[j].EventType
		}
		return rows[i].ServiceClass < rows[j].ServiceClass
	})
	return rows, nil
}

func (c *Core) CalibrationExperiments(ctx context.Context) ([]store.CalibrationExperiment, error) {
	return c.store.GetCalibrationExperiments(ctx)
}

func (c *Core) CreateCalibrationExperiment(ctx context.Context, label string) (*store.CalibrationExperiment, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, fmt.Errorf("calibration experiment label is required")
	}
	experiment := &store.CalibrationExperiment{ID: "cal-" + uuid.NewString(), Label: label, CreatedAt: time.Now().UTC()}
	if err := c.store.CreateCalibrationExperiment(ctx, experiment); err != nil {
		return nil, err
	}
	return experiment, nil
}

// ApplyFeedSnapshot applies a complete catalogue without starting an asset
// scan. The caller supplies the threshold because it is deployment policy,
// not a probability-affecting constant embedded in the engine.
func (c *Core) ApplyFeedSnapshot(ctx context.Context, snapshot *feed.Snapshot, epssThreshold float64) ([]store.Regression, error) {
	regs, err := c.feedIngest.ApplySnapshot(ctx, snapshot, epssThreshold)
	if err != nil {
		return nil, err
	}
	for _, r := range regs {
		c.bus.Publish(ctx, event.Event{Type: event.EventRegressionNew, Timestamp: r.ConfirmedAt, Payload: r})
	}
	return regs, nil
}

func (c *Core) LatestFeedSnapshot(ctx context.Context, feedName string) (*store.FeedSnapshot, error) {
	return c.store.GetLatestFeedSnapshot(ctx, feedName)
}

func (c *Core) FeedStatuses(ctx context.Context) ([]store.FeedStatus, error) {
	return c.feedIngest.FeedStatuses(ctx, time.Now().UTC(), service.DefaultFeedStalenessThreshold)
}

func (c *Core) AssetContactPanel(ctx context.Context, assetID string, excludedFactors []string) (service.AssetContactPanel, error) {
	return c.assessment.ContactPanel(ctx, assetID, excludedFactors, time.Now().UTC())
}

func (c *Core) WeaknessContactAggregations(ctx context.Context) ([]contact.InstanceAggregation, error) {
	return c.assessment.WeaknessAggregations(ctx, time.Now().UTC())
}

// ApplyFeedSnapshotBody parses and applies a raw catalogue document supplied
// over an API or file boundary without starting an asset scan.
func (c *Core) ApplyFeedSnapshotBody(ctx context.Context, feedName, sourceURL string, body []byte, epssThreshold float64) (*feed.Snapshot, []store.Regression, error) {
	snapshot, err := feed.Parse(feed.Name(feedName), sourceURL, time.Now().UTC(), body)
	if err != nil {
		return nil, nil, err
	}
	regs, err := c.ApplyFeedSnapshot(ctx, snapshot, epssThreshold)
	if err != nil {
		return nil, nil, err
	}
	return snapshot, regs, nil
}

// RefreshFeeds pulls KEV, EPSS and NVD (referenced CVEs only) from the
// configured or default public authorities and applies each snapshot that
// succeeds. Failures on one feed are recorded against that feed without
// blocking the others.
func (c *Core) RefreshFeeds(ctx context.Context, rawEndpoints map[string]string, epssThreshold float64) ([]store.FeedStatus, error) {
	return c.RefreshNamedFeeds(ctx, nil, rawEndpoints, epssThreshold)
}

// ErrUnknownFeed is returned for a feed name the engine does not ingest.
var ErrUnknownFeed = errors.New("trawl: unknown feed")

// RefreshNamedFeeds refreshes only the named feeds, or all of them when none
// are named. An unknown name is refused rather than ignored, so a mistyped
// feed is not reported as refreshed.
//
// Refreshes are serialised: the scheduler and an operator pressing the button
// at the same moment would otherwise fetch the same catalogue twice and race
// to apply it.
func (c *Core) RefreshNamedFeeds(ctx context.Context, names []string, rawEndpoints map[string]string, epssThreshold float64) ([]store.FeedStatus, error) {
	all := []feed.Name{feed.KEV, feed.EPSS, feed.NVD}
	wanted := all
	if len(names) > 0 {
		wanted = nil
		for _, n := range names {
			var match feed.Name
			for _, known := range all {
				if string(known) == n {
					match = known
				}
			}
			if match == "" {
				return nil, fmt.Errorf("%w: %q (want cisa-kev, epss or nvd)", ErrUnknownFeed, n)
			}
			wanted = append(wanted, match)
		}
	}

	c.feedMu.Lock()
	defer c.feedMu.Unlock()

	var endpoints map[feed.Name]string
	if len(rawEndpoints) > 0 {
		endpoints = make(map[feed.Name]string, len(rawEndpoints))
		for k, v := range rawEndpoints {
			endpoints[feed.Name(k)] = v
		}
	}
	if len(endpoints) == 0 {
		endpoints = feed.DefaultEndpoints()
	}
	client, err := feed.NewClient(&http.Client{Timeout: 45 * time.Second}, endpoints)
	if err != nil {
		return nil, err
	}
	if key, err := c.store.GetSetting(ctx, NVDAPIKeySetting); err == nil && strings.TrimSpace(key) != "" {
		client.SetNVDAPIKey(key)
	}
	var firstErr error
	for _, name := range wanted {
		_, ok := endpoints[name]
		if name == feed.EPSS && !ok {
			// EPSS is reachable through either its daily file or FIRST's
			// per-CVE API.
			_, ok = endpoints[feed.EPSSAPI]
		}
		if !ok {
			continue
		}
		regs, applyErr := c.feedIngest.FetchAndApply(ctx, client, name, epssThreshold)
		if applyErr != nil && firstErr == nil {
			firstErr = applyErr
		}
		for _, r := range regs {
			c.bus.Publish(ctx, event.Event{Type: event.EventRegressionNew, Timestamp: r.ConfirmedAt, Payload: r})
		}
	}
	statuses, statusErr := c.FeedStatuses(ctx)
	if statusErr != nil {
		return nil, statusErr
	}
	return statuses, firstErr
}

// AssetSoftware returns observed software records for one asset, or across the
// whole estate when assetID is empty.
func (c *Core) AssetSoftware(ctx context.Context, assetID string) ([]store.AssetSoftware, error) {
	return c.store.GetAssetSoftware(ctx, assetID)
}

// SoftwareCoverage returns software visibility for one asset, or for every
// active asset when assetID is empty.
func (c *Core) SoftwareCoverage(ctx context.Context, assetID string) ([]store.SoftwareCoverage, error) {
	if assetID != "" {
		cov, err := c.store.ComputeSoftwareCoverage(ctx, assetID)
		if err != nil {
			return nil, err
		}
		return []store.SoftwareCoverage{cov}, nil
	}
	assets, err := c.store.GetAssets(ctx, store.AssetStatusActive)
	if err != nil {
		return nil, err
	}
	out := make([]store.SoftwareCoverage, 0, len(assets))
	for _, a := range assets {
		cov, err := c.store.ComputeSoftwareCoverage(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, cov)
	}
	return out, nil
}

// RefreshCandidates queries NVD for mapped, versioned software observed on
// active assets and reconciles unconfirmed candidate findings. It is only
// invoked on explicit operator refresh or by a scheduled job, never at startup
// or on ingest.
func (c *Core) RefreshCandidates(ctx context.Context, rawEndpoints map[string]string, force bool) (*service.CandidateRefreshSummary, error) {
	c.feedMu.Lock()
	defer c.feedMu.Unlock()

	var endpoints map[feed.Name]string
	if len(rawEndpoints) > 0 {
		endpoints = make(map[feed.Name]string, len(rawEndpoints))
		for k, v := range rawEndpoints {
			endpoints[feed.Name(k)] = v
		}
	}
	if len(endpoints) == 0 {
		endpoints = feed.DefaultEndpoints()
	}
	client, err := feed.NewClient(&http.Client{Timeout: 45 * time.Second}, endpoints)
	if err != nil {
		return nil, err
	}
	if key, err := c.store.GetSetting(ctx, NVDAPIKeySetting); err == nil && strings.TrimSpace(key) != "" {
		client.SetNVDAPIKey(key)
	}
	svc := service.NewCandidateLookupService(c.store)
	summary, err := svc.RefreshCandidates(ctx, client, force)
	if err != nil {
		return nil, err
	}
	c.bus.Publish(ctx, event.Event{
		Type:      event.EventAssetUpdated,
		Timestamp: time.Now().UTC(),
		Payload:   summary,
	})
	return summary, nil
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

// ─── Email posture ───────────────────────────────────────────────────────────

func (c *Core) EmailPostures(ctx context.Context) ([]store.EmailPosture, error) {
	return c.store.GetEmailPostures(ctx)
}

// ScanEmailPosture assesses one domain's email-authentication posture within
// the operator's authorised scope.
//
// The scope is read here rather than defaulted in the service, so that the
// authorisation governing this capability is the same one governing every
// other assessment — there is no second, more permissive path to the network.
func (c *Core) ScanEmailPosture(ctx context.Context, domain string) (store.EmailPosture, error) {
	scope := c.Scope(ctx)
	return c.emailScanner.ScanAndSave(ctx, domain, scope.SeedDomainsList, scope.ConsentedEndpoints)
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
	return c.AssessDomainWithOptions(ctx, domain, service.AssessOptions{})
}

// AssessDomainWithOptions runs a scope-bounded assessment with explicit
// Vantage v1.8.0 external hostname discovery and/or opt-in dangling
// cloud-address identity probing (`dangle`).
func (c *Core) AssessDomainWithOptions(ctx context.Context, domain string, opts service.AssessOptions) (service.DomainAssessment, error) {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return service.DomainAssessment{
			AssetID: service.NormaliseDomain(domain),
			Domain:  service.NormaliseDomain(domain),
			Outcome: "refused",
			Error:   "no signed authorisation is on record; assessment refused",
		}, nil
	}
	view, err := c.assessment.AssessWithOptions(ctx, domain, scope.SeedDomainsList, scope.ConsentedEndpoints, opts)
	if err == nil {
		c.bus.Publish(ctx, event.Event{Type: event.EventAssetUpdated})
	}
	return view, err
}

// ProbeDiscoveredServices runs a bounded ranked service pass against the
// currently tracked in-scope domain and subdomain assets. It is separate from
// DNS assessment because reachability is exposure evidence, not a control
// finding or attacker-contact signal.
func (c *Core) ProbeDiscoveredServices(ctx context.Context, profile vprobe.DiscoveryProfile) error {
	return c.probeDiscoveredServices(ctx, profile, "")
}

func (c *Core) probeDiscoveredServices(ctx context.Context, profile vprobe.DiscoveryProfile, domainFilter string) error {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return fmt.Errorf("service probe: no signed authorisation is on record")
	}
	assets, err := c.store.GetAssets(ctx, store.AssetStatusActive)
	if err != nil {
		return fmt.Errorf("service probe: listing assets: %w", err)
	}
	if profile == "" {
		profile = vprobe.DiscoveryMostCommon
	}
	guard := vadapter.NewScope(scope.SeedDomainsList, scope.ConsentedEndpoints)
	prober := &vprobe.ServiceProber{Profile: vprobe.Profile{
		Name: "declared-services", MaxConcurrency: 8, MaxProbes: 100,
		Allow: func(target vprobe.Target) error {
			if guard.PermitsTarget(target.Host) {
				return nil
			}
			return fmt.Errorf("%q is outside the authorised scope", target.Host)
		},
	}}
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	normDomain := service.NormaliseDomain(domainFilter)
	targets := make([]store.Asset, 0, len(assets))
	for _, asset := range assets {
		if asset.Type != store.AssetTypeDomain && asset.Type != store.AssetTypeSubdomain {
			continue
		}
		if normDomain != "" && !hostBelongsToDomain(asset.Value, normDomain) {
			continue
		}
		if !guard.PermitsTarget(asset.Value) {
			continue
		}
		targets = append(targets, asset)
	}
	total := len(targets)
	started := time.Now().UTC()
	sweepRunID := "run-" + uuid.NewString()
	sweepCtx := store.WithRun(ctx, store.RunRef{ID: sweepRunID, Kind: store.RunServiceProbe})
	c.bus.Publish(ctx, event.Event{Type: event.EventScanProgress, Payload: ServiceProbeProgress{
		Phase: "service-probe-started", Profile: string(profile), Total: total,
	}})
	completed := 0
	for _, asset := range targets {
		asset := asset
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			observations, err := c.assessment.ProbeServices(sweepCtx, prober, vadapter.ServiceRequest{
				AssetID: asset.ID, Host: asset.Value, Profile: profile,
			})
			exposed := 0
			for _, observation := range observations {
				if observation.State == "responding" {
					exposed++
				}
			}
			mu.Lock()
			completed++
			progress := ServiceProbeProgress{
				Phase: "service-probe-progress", Profile: string(profile),
				AssetID: asset.ID, Host: asset.Value, Completed: completed,
				Total: total, Observations: len(observations), Exposed: exposed,
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", asset.Value, err))
				progress.Error = err.Error()
			}
			mu.Unlock()
			c.bus.Publish(ctx, event.Event{Type: event.EventScanProgress, Payload: progress})
		}()
	}
	wg.Wait()
	if total > 0 {
		outcome := "completed"
		errText := ""
		if len(failures) > 0 {
			outcome = "partial"
			errText = failures[0].Error()
		}
		okCount := total - len(failures)
		if okCount < 0 {
			okCount = 0
		}
		_ = c.store.RecordScanRun(ctx, &store.ScanRun{
			ID:         sweepRunID,
			Kind:       store.RunServiceProbe,
			AssetID:    normDomain,
			Profile:    string(profile),
			Outcome:    outcome,
			Error:      errText,
			StartedAt:  started,
			FinishedAt: time.Now().UTC(),
			Coverage: store.CoverageSummary{
				Total:       total,
				OK:          okCount,
				CheckFailed: len(failures),
			},
		})
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(failures) > 0 {
		return fmt.Errorf("service probe: %d assets failed (first: %w)", len(failures), failures[0])
	}
	return nil
}

func hostBelongsToDomain(host, normDomain string) bool {
	h := service.NormaliseDomain(host)
	return h == normDomain || strings.HasSuffix(h, "."+normDomain)
}

// ─── Orchestration ───────────────────────────────────────────────────────────

// ScanRequest is one run across the capabilities the operator asked for.
type ScanRequest struct {
	Domain  string `json:"domain"`
	RepoURL string `json:"repoUrl"`
}

// ServiceProbeProgress is emitted as each asset completes its parallel
// service-probe pass, so transports can show progress before the full sweep
// has finished.
type ServiceProbeProgress struct {
	Phase        string `json:"phase"`
	Profile      string `json:"profile"`
	AssetID      string `json:"assetId"`
	Host         string `json:"host"`
	Completed    int    `json:"completed"`
	Total        int    `json:"total"`
	Observations int    `json:"observations"`
	Exposed      int    `json:"exposed"`
	Error        string `json:"error,omitempty"`
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
		// Run host discovery and domain assessment first so that every
		// discovered domain and subdomain (from both subfinder and Vantage CT
		// discovery) is in the asset store before service probing runs.
		var discoverWG sync.WaitGroup
		discoverWG.Add(2)
		go func() {
			defer discoverWG.Done()
			if err := c.networkScanner.DiscoverSubdomains(ctx, req.Domain); err != nil {
				record(fmt.Errorf("discovery: %w", err))
			}
		}()
		go func() {
			defer discoverWG.Done()
			hasThirdParty := len(c.Scope(ctx).ConsentedEndpoints) > 0
			if _, err := c.AssessDomainWithOptions(ctx, req.Domain, service.AssessOptions{
				Discover: hasThirdParty,
			}); err != nil {
				record(fmt.Errorf("assessment: %w", err))
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			discoverWG.Wait()
			if err := c.probeDiscoveredServices(ctx, vprobe.DiscoveryMostCommon, req.Domain); err != nil {
				record(fmt.Errorf("service exposure: %w", err))
			}
		}()

		// Email-authentication posture, now assessed through the same
		// scope-guarded adapter as everything else.
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.ScanEmailPosture(ctx, req.Domain); err != nil {
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
