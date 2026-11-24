import { DestroyRef, effect, inject, Injectable, signal, untracked } from '@angular/core';
import { AssetContactPanelUI, AssetSoftwareUI, CandidateRefreshSummaryUI, ChangeSet, DomainAssessment, EmailPostureUI, FeedStatusUI, FindingDetail, FindingSummary, HistoryPoint, InstanceAggregationUI, ScanRunView, SoftwareCoverageUI } from './models/types';
import { findingHash, normalizeDetail, parseFindingHash } from './dashboard/findings';
import { normalizeChangeSet } from './dashboard/history';
import { RegressionUI } from './dashboard/estate';
import { ThemeService } from './theme.service';
import { AssessOptionsUI, CalibrationExperiment, CalibrationReportRow, createTransport, DiscoveryOptions, DiscoveryResult, ProposedDomain, Scope, TrawlTransport } from './transport';

/** The tabs of the shell. */
export type AppTab = 'executive' | 'overview' | 'assets' | 'findings' | 'email' | 'secrets' | 'scope';

/**
 * WailsIpcService is the application's single point of contact with the
 * backend.
 *
 * Despite the name it is no longer Wails-specific: it delegates to a
 * TrawlTransport chosen at runtime, so the identical bundle drives the desktop
 * build over IPC and the container dashboard over HTTP. Components depend on
 * this service and are unaware of which deployment they are running in.
 */
@Injectable({
  providedIn: 'root'
})
export class WailsIpcService {
  /** The transport in use, exposed so the UI can disclose the deployment. */
  public readonly transport: TrawlTransport = createTransport();

  private readonly themes = inject(ThemeService);

  // UI State

  /**
   * The appearance in force, read-only.
   *
   * Exposed here so that every component keeps reading the theme where it
   * always did, but resolved and persisted by `ThemeService`. It is not a
   * writable signal: a component that set it directly would change the
   * appearance without recording the operator's choice, and the next launch
   * would silently undo them.
   */
  public readonly theme = this.themes.theme;
  public activeTab = signal<AppTab>('executive');

  // Data State
  public assets = signal<any[]>([]);
  public findings = signal<any[]>([]);
  /**
   * The persisted email-authentication posture of each domain.
   *
   * Typed rather than `any[]`: this signal held a stale shape for an entire
   * phase — booleans, after the backend had widened to four-state controls —
   * and nothing complained, because nothing could.
   */
  public emailPostures = signal<EmailPostureUI[]>([]);
  public secretFindings = signal<any[]>([]);

  /**
   * Confirmed posture regressions: a tracked attribute that degraded between
   * checks.
   *
   * Held apart from findings on purpose. "This became worse" and "we found
   * this" call for different responses, and a regression folded into the
   * finding list loses the one thing that makes it urgent — that it used to
   * be fine.
   */
  public regressions = signal<RegressionUI[]>([]);

  // Change 025: findings history and drill-down.
  //
  // Each list carries a load state beside it. Three states, as with `loadState`:
  // an empty array means "nothing recorded" only once the state says `ready`,
  // and an `error` keeps whatever was last loaded rather than blanking it.

  /** The unified findings list across every source. */
  public findingIndex = signal<FindingSummary[]>([]);
  public findingIndexState = signal<'loading' | 'ready' | 'error'>('loading');
  public findingIndexError = signal<string>('');

  /** The run ledger, newest first. */
  public scanRuns = signal<ScanRunView[]>([]);
  public scanRunsState = signal<'loading' | 'ready' | 'error'>('loading');

  /** Per-run summaries, oldest first, for the trend. */
  public history = signal<HistoryPoint[]>([]);
  public historyState = signal<'loading' | 'ready' | 'error'>('loading');

  /** What changed since the previous run (the engine's default comparison). */
  public changeSet = signal<ChangeSet | null>(null);
  public changeSetState = signal<'loading' | 'ready' | 'error'>('loading');

  /**
   * The finding whose detail is open, or null. Kept in step with the address
   * `#/findings/<key>` so the detail can be shared and the back button closes
   * it. Opening a key always opens the Findings tab.
   */
  public selectedFindingKey = signal<string | null>(null);

  public calibrationReport = signal<CalibrationReportRow[]>([]);
  public calibrationReportLoading = signal(false);
  public calibrationReportError = signal('');
  public calibrationExperiments = signal<CalibrationExperiment[]>([]);
  public feedStatuses = signal<FeedStatusUI[]>([]);
  public isRefreshingFeeds = signal(false);
  /** The feed being refreshed on its own, or null; disables that feed's button only. */
  public refreshingFeed = signal<string | null>(null);
  public assetSoftware = signal<AssetSoftwareUI[]>([]);
  public softwareCoverage = signal<SoftwareCoverageUI[]>([]);
  public isRefreshingCandidates = signal(false);
  public candidateRefreshError = signal('');
  public lastCandidateRefresh = signal<CandidateRefreshSummaryUI | null>(null);
  public weaknessContactAggregations = signal<InstanceAggregationUI[]>([]);

  async loadCalibrationReport(experimentId: string): Promise<void> {
    this.calibrationReportLoading.set(true);
    this.calibrationReportError.set('');
    try {
      this.calibrationReport.set(await this.transport.getCalibrationReport(experimentId));
    } catch (error) {
      this.calibrationReportError.set(error instanceof Error ? error.message : 'Calibration report could not be loaded.');
      this.calibrationReport.set([]);
    } finally {
      this.calibrationReportLoading.set(false);
    }
  }

  async loadCalibrationExperiments(): Promise<void> {
    this.calibrationExperiments.set(await this.transport.getCalibrationExperiments());
  }

  async createCalibrationExperiment(label: string): Promise<CalibrationExperiment> {
    const experiment = await this.transport.createCalibrationExperiment(label);
    this.calibrationExperiments.update(items => [experiment, ...items]);
    return experiment;
  }

  public async refreshFeedStatuses(): Promise<void> {
    this.feedStatuses.set((await this.transport.getFeedStatuses()) || []);
  }

  public async getAssetContactPanel(assetId: string, excludedFactors: string[] = []): Promise<AssetContactPanelUI | null> {
    return this.transport.getAssetContactPanel(assetId, excludedFactors);
  }

  public async refreshWeaknessContactAggregations(): Promise<void> {
    this.weaknessContactAggregations.set((await this.transport.getWeaknessContactAggregations()) || []);
  }

  public async refreshFeeds(epssThreshold = 0): Promise<void> {
    this.isRefreshingFeeds.set(true);
    try {
      const statuses = await this.transport.refreshFeeds(epssThreshold);
      this.feedStatuses.set(statuses || []);
      await Promise.all([this.refreshFindings(''), this.refreshRegressions(), this.refreshFindingViews()]);
    } finally {
      this.isRefreshingFeeds.set(false);
    }
  }

  /**
   * Refreshes one catalogue. Findings are re-read afterwards because applying a
   * snapshot can change priority, and the list the operator is looking at
   * should not keep the old one.
   */
  public async refreshFeed(feed: string, epssThreshold = 0): Promise<void> {
    this.refreshingFeed.set(feed);
    try {
      const statuses = await this.transport.refreshFeed(feed, epssThreshold);
      this.feedStatuses.set(statuses || []);
      await Promise.all([this.refreshFindings(''), this.refreshRegressions(), this.refreshFindingViews()]);
    } finally {
      this.refreshingFeed.set(null);
    }
  }

  private softwareRefreshRunning = false;
  private softwareRefreshPending = false;

  public async refreshSoftware(assetId = ''): Promise<void> {
    if (!assetId) {
      if (this.softwareRefreshRunning) {
        this.softwareRefreshPending = true;
        return;
      }
      this.softwareRefreshRunning = true;
      try {
        do {
          this.softwareRefreshPending = false;
          const [items, cov] = await Promise.all([
            this.transport.getAssetSoftware(''),
            this.transport.getSoftwareCoverage('')
          ]);
          this.assetSoftware.set(items || []);
          this.softwareCoverage.set(cov || []);
        } while (this.softwareRefreshPending);
      } finally {
        this.softwareRefreshRunning = false;
      }
      return;
    }
    const [items, cov] = await Promise.all([
      this.transport.getAssetSoftware(assetId),
      this.transport.getSoftwareCoverage(assetId)
    ]);
    this.assetSoftware.set(items || []);
    this.softwareCoverage.set(cov || []);
  }

  /**
   * Explicitly refreshes software candidate vulnerabilities from NVD.
   * Never invoked at startup or on ingest.
   */
  public async refreshCandidates(force = false): Promise<CandidateRefreshSummaryUI | null> {
    this.isRefreshingCandidates.set(true);
    this.candidateRefreshError.set('');
    try {
      const summary = await this.transport.refreshCandidates(force);
      this.lastCandidateRefresh.set(summary);
      await Promise.all([this.refreshFindingViews(), this.refreshSoftware(), this.refreshFeedStatuses()]);
      return summary;
    } catch (error) {
      this.candidateRefreshError.set(
        error instanceof Error ? error.message : 'Candidate lookup could not be completed.'
      );
      return null;
    } finally {
      this.isRefreshingCandidates.set(false);
    }
  }

  /**
   * The build identity of the engine answering, read from it rather than
   * declared here.
   *
   * Empty until the first successful read, and the header shows nothing in
   * that case. A version badge is a claim about which build produced what is
   * on screen, and the previous hardcoded `v0.1.0-OSS` went on making that
   * claim across every release after the one it was typed in. Showing nothing
   * is worse-looking and more honest; showing a stale number is neither.
   *
   * On the HTTP transport this reports the *server's* build, which is the
   * useful answer — a browser tab may well be older or newer than the engine
   * it is pointed at.
   */
  public engineVersion = signal<string>('');

  /**
   * When the view last received data it trusts.
   *
   * A dashboard that silently stops updating is worse than one that never
   * claimed to be live: the operator's belief about freshness is now wrong and
   * nothing on screen contradicts it. Recording this is what lets the
   * interface say how old what it is showing actually is.
   */
  public lastUpdatedAt = signal<string | null>(null);

  /** False once the live stream has reported a drop it has not recovered from. */
  public streamHealthy = signal<boolean>(true);

  /**
   * Measured-state assessments from vantage: four-state coverage, derived
   * control postures and the advisories behind them.
   */
  public assessments = signal<DomainAssessment[]>([]);

  private assessmentRefreshRunning = false;
  private assessmentRefreshPending = false;

  /** Per-check progress from the running assessment, for live feedback. */
  public assessmentProgress = signal<{ domain: string; check: string; done: number; total: number } | null>(null);

  // Engine State
  public isScanning = signal<boolean>(false);
  public scanStatusMessage = signal<string>('');

  /**
   * Whether the store has been read yet, and whether the read succeeded.
   *
   * Three states rather than a boolean, because "still loading", "loaded and
   * empty" and "could not load" are three different claims about the estate
   * and only one of them is good news. Collapsing them leaves an empty table
   * meaning either that nothing is wrong or that nothing was fetched, with no
   * way for the reader to tell which.
   */
  public loadState = signal<'loading' | 'ready' | 'error'>('loading');

  /** What went wrong on the last load, phrased for the operator. */
  public loadError = signal<string>('');

  /**
   * How the current status message should be read.
   *
   * Carried alongside the message rather than inferred from its wording. A
   * banner that decides its own severity by searching the text for "error"
   * starts colouring things wrongly the moment a message is reworded, and the
   * case it gets wrong is the one nobody notices until it matters.
   */
  public scanStatusTone = signal<'info' | 'success' | 'warning' | 'error'>('info');

  public lastScanDate = signal<string | null>(null);

  /** Sets the status line and how it should be presented, in one step. */
  public setScanStatus(message: string, tone: 'info' | 'success' | 'warning' | 'error' = 'info'): void {
    this.scanStatusMessage.set(message);
    this.scanStatusTone.set(tone);
  }

  // Scope State
  public seedDomainsList = signal<string[]>([]);
  public seedCidrsList = signal<string[]>([]);
  public seedReposList = signal<string[]>([]);
  public consentedEndpoints = signal<string[]>([]);
  public isAuthorized = signal<boolean>(false);
  public signerName = signal<string>('');
  public signerTitle = signal<string>('');
  public authorizationDate = signal<string>('');

  // Discovery State
  /**
   * Domains certificate evidence suggests are the operator's, awaiting a
   * decision. They are held apart from the authorised scope on purpose:
   * nothing here has been assessed, and nothing will be until it is moved
   * across deliberately.
   */
  public proposedDomains = signal<ProposedDomain[]>([]);

  /** Domains the operator has ruled out. Kept so they are not re-proposed. */
  public dismissedDomains = signal<string[]>([]);

  /** True while a discovery pass is running, for button and spinner state. */
  public isDiscovering = signal<boolean>(false);

  /** The outcome of the last discovery pass, including partial-result notes. */
  public lastDiscovery = signal<DiscoveryResult | null>(null);

  constructor() {
    this.initFindingAddress();

    this.transport.on('asset:updated', () => this.refreshAssets());

    this.transport.on('finding:new', (payload: any) => {
      void this.refreshFindingIndex();
      if (payload?.assetId) {
        this.refreshFindings(payload.assetId);
      }
    });

    this.transport.on('scan:progress', (payload: any) => {
      if (payload?.phase === 'service-probe-started') {
        this.setScanStatus(`Service sweep started (${payload.profile})…`);
      }
      if (payload?.phase === 'service-probe-progress') {
        const state = payload.error ? 'failed' : `${payload.observations} observations`;
        this.setScanStatus(`Probed ${payload.host} (${payload.completed} of ${payload.total}) · ${state}`,
          payload.error ? 'warning' : 'info');
        void this.refreshAssessments();
        void this.refreshFindingIndex();
        void this.refreshSoftware();
        if (payload.assetId) {
          void this.refreshFindings(payload.assetId);
        }
      }
      if (payload?.check) {
        this.assessmentProgress.set({
          domain: payload.domain,
          check: payload.check,
          done: payload.checksDone,
          total: payload.checksTotal
        });
      }
      if (payload?.phase === 'complete') {
        this.onScanComplete(payload);
      }
    });

    this.transport.on('scan:complete', (payload: any) => this.onScanComplete(payload));

    // A regression is confirmed by the engine, not derived here, so the view
    // learns about it the same way it learns about a finding.
    this.transport.on('regression:new', () => this.refreshRegressions());

    // The transport reports a dropped stream rather than repairing it. The
    // view must say so: a dashboard that silently stops updating leaves the
    // operator believing something about freshness that is no longer true.
    this.transport.on('stream:down', () => this.streamHealthy.set(false));
    this.transport.on('stream:up', () => {
      this.streamHealthy.set(true);
      this.refreshAll();
    });

    // Another client — or another window onto the same engine — may have
    // erased the estate. Reloading from the store keeps this view honest
    // rather than showing data the backend no longer holds.
    this.transport.on('data:erased', () => this.refreshAll());
  }

  /**
   * True while a whole-scope scan is walking its targets.
   *
   * Each target's completion arrives as its own event, so without this the
   * first domain to finish would clear the scanning state and announce that
   * the scan was complete while the remaining targets were still running.
   */
  private scopeScanInFlight = false;

  /**
   * Targets whose completion event has not yet arrived.
   *
   * Requesting a scan is not the same as finishing one. Two of the three
   * deployment paths detach the work — the desktop binding runs it in a
   * goroutine, and a long-lived container answers 202 — so the call resolves
   * at acceptance and the scan is still running. Counting outstanding targets
   * is what lets the run end when the work ends rather than when the last
   * request was accepted.
   */
  private pendingTargets = 0;

  /** Resolves once every outstanding target has reported completion. */
  private allTargetsSettled: (() => void) | null = null;

  /** Targets finished so far, for the progress line. */
  private targetsDone = 0;
  private targetsTotal = 0;

  /**
   * Targets whose scan ran but did not conclude cleanly, named by the backend
   * in its completion event.
   *
   * Kept separately from the request failures collected in
   * `scanAuthorisedScope`, because the two are different facts: one is a scan
   * that could not be started, the other a scan that started and came back
   * incomplete. Both must be reported; neither may be presented as success.
   */
  private partialTargets: string[] = [];

  /**
   * Targets the last run reported as incomplete, for callers that write their
   * own closing message. Exposed so a component cannot overwrite a partial
   * result with a success line simply because no request was rejected.
   */
  public lastPartialTargets = signal<string[]>([]);

  /**
   * Records that one target has finished, and releases the whole-scope scan
   * once none are outstanding.
   */
  private noteTargetFinished(): void {
    if (this.pendingTargets > 0) {
      this.pendingTargets--;
      this.targetsDone++;
      if (this.pendingTargets > 0) {
        this.setScanStatus(
          `Scanned ${this.targetsDone} of ${this.targetsTotal} - ${this.pendingTargets} still running…`
        );
      }
    }
    if (this.pendingTargets === 0) {
      const settled = this.allTargetsSettled;
      this.allTargetsSettled = null;
      settled?.();
    }
  }

  /**
   * Notes a target the backend reported as incomplete.
   *
   * A completion event with no status is treated as clean, because that is
   * what an older backend emits and inventing a failure would be worse than
   * missing one. Anything that explicitly is not "completed" is recorded.
   */
  private noteOutcome(payload: any): void {
    if (!payload || !payload.status || payload.status === 'completed') {
      return;
    }
    const target = payload.domain || payload.repoUrl || 'a target';
    if (!this.partialTargets.includes(target)) {
      this.partialTargets.push(target);
    }
    console.warn(`Trawl: ${target} completed with failures:`, payload.error);
  }

  private async onScanComplete(payload?: any): Promise<void> {
    this.noteOutcome(payload);

    await Promise.all([
      this.refreshAssets(),
      this.refreshFindings(''),
      this.refreshSecretFindings(),
      this.refreshEmailPostures(),
      this.refreshAssessments(),
      this.refreshFindingViews()
    ]);

    // Mid-portfolio, refresh the views but leave the status alone: the run is
    // not over, and saying so would be wrong in the direction that matters.
    if (this.scopeScanInFlight) {
      this.noteTargetFinished();
      return;
    }

    this.assessmentProgress.set(null);
    this.isScanning.set(false);

    if (this.partialTargets.length > 0) {
      // Named rather than counted. "1 target incomplete" leaves the operator
      // to work out which, and the whole point is that they should not read
      // this estate as fully assessed.
      const named = this.partialTargets.join(', ');
      this.lastPartialTargets.set([...this.partialTargets]);
      this.partialTargets = [];
      this.setScanStatus(
        `Scan finished, but ${named} did not complete. Results are partial.`,
        'warning'
      );
      return;
    }

    this.lastPartialTargets.set([]);
    this.setScanStatus('Scan complete. Findings have been ingested.', 'success');
    setTimeout(() => this.scanStatusMessage.set(''), 8000);
  }

  public async loadSettings(): Promise<void> {
    const scope = await this.transport.getScope();
    if (!scope) {
      return;
    }
    this.seedDomainsList.set(scope.seedDomainsList || []);
    this.seedCidrsList.set(scope.seedCidrsList || []);
    this.seedReposList.set(scope.seedReposList || []);
    this.consentedEndpoints.set(scope.consentedEndpoints || []);
    this.isAuthorized.set(scope.isAuthorized || false);
    this.signerName.set(scope.signerName || '');
    this.signerTitle.set(scope.signerTitle || '');
    this.authorizationDate.set(scope.authorizationDate || '');
    this.proposedDomains.set(scope.proposedDomains || []);
    this.dismissedDomains.set(scope.dismissedDomains || []);
  }

  public async saveSettings(): Promise<void> {
    const scope: Scope = {
      seedDomainsList: this.seedDomainsList(),
      seedCidrsList: this.seedCidrsList(),
      seedReposList: this.seedReposList(),
      consentedEndpoints: this.consentedEndpoints(),
      isAuthorized: this.isAuthorized(),
      signerName: this.signerName(),
      signerTitle: this.signerTitle(),
      authorizationDate: this.authorizationDate()
    };
    await this.transport.saveScope(scope);
  }

  public async refreshAssets(status: string = ''): Promise<void> {
    const nextAssets = (await this.transport.getAssets(status)) || [];
    this.assets.set(nextAssets);
    void this.refreshSoftware();
    if (this.findings().length > 0 && nextAssets.length > 0) {
      const valueById = new Map<string, string>();
      for (const a of nextAssets) {
        if (a?.id && a?.value) {
          valueById.set(a.id, a.value);
        }
      }
      this.findings.update(current =>
        current.map(f => ({
          ...f,
          assetValue: valueById.get(f.assetId) || f.assetValue || f.assetId || ''
        }))
      );
    }
  }

  /**
   * Removes an asset and reloads the inventory.
   *
   * The list is reloaded from the store rather than patched in place, so what
   * the operator sees afterwards is what the engine actually holds. This
   * rethrows: the caller has a person waiting on the outcome of something they
   * clicked.
   */
  public async removeAsset(id: string): Promise<void> {
    await this.transport.removeAsset(id);
    await Promise.all([this.refreshAssets(), this.refreshFindings(''), this.refreshFindingViews()]);
  }

  private mapFinding(finding: any, assetNameById: Map<string, string>): any {
    const resolvedValue =
      finding.assetValue ||
      assetNameById.get(finding.assetId) ||
      finding.assetId ||
      '';
    return {
      ...finding,
      assetValue: resolvedValue,
      cveId: finding.cveId || finding.cve || '',
      kev: finding.kev ?? finding.kevListed ?? false,
      epssScore: finding.epssScore ?? finding.epss ?? 0,
      cvssScore: finding.cvssScore ?? 0,
      status: finding.status || 'open',
      detectedAt: finding.detectedAt || finding.firstSeen || finding.lastSeen || ''
    };
  }

  /**
   * Loads findings from the store.
   *
   * When `assetId` is empty, replaces the estate-wide list. When `assetId`
   * names a single asset (during incremental `finding:new` or
   * `service-probe-progress` events), merges that asset's rows into the
   * existing signal rather than replacing the whole list — otherwise a
   * clean/unresponsive host finishing right after an exposed one wipes every
   * other asset's findings off the screen.
   */
  public async refreshFindings(assetId: string = ''): Promise<void> {
    const rows = (await this.transport.getFindings(assetId)) || [];
    const assetNameById = new Map<string, string>();
    for (const a of this.assets()) {
      if (a?.id && a?.value) {
        assetNameById.set(a.id, a.value);
      }
    }
    const mapped = rows.map((f: any) => this.mapFinding(f, assetNameById));

    if (!assetId) {
      this.findings.set(mapped);
      return;
    }

    this.findings.update(current => {
      const others = current.filter((f: any) => f.assetId !== assetId);
      return [...mapped, ...others];
    });
  }

  public async triggerScan(domain: string, repoUrl: string): Promise<void> {
    this.isScanning.set(true);
    try {
      await this.transport.triggerScan(domain, repoUrl);
    } finally {
      // The call returns either when the scan was accepted (long-lived
      // deployments, completion arrives as an event) or when it has actually
      // finished (autoscaled deployments, which run it inline). Refreshing
      // here covers the second case, and covers the first when the event
      // stream is served by a different instance from the one that scanned.
      await this.onScanComplete();
    }
  }

  /**
   * Scans every authorised target, not merely the first one.
   *
   * The backend's scan request names a single domain, so covering a portfolio
   * means issuing one request per target. Doing that here rather than in a
   * component keeps both deployments and every caller consistent: a scan
   * started from anywhere covers the whole authorised scope.
   *
   * Targets are scanned sequentially. Each individual scan already runs its
   * own checks concurrently, so firing every domain at once would multiply
   * that into a burst against the targets' nameservers without finishing
   * meaningfully sooner.
   *
   * One target failing does not stop the rest. A portfolio scan that abandons
   * everything because the third domain has a broken delegation is far less
   * useful than one that reports what it found and names what it could not.
   *
   * Requests are issued and then the run waits for the work to report back.
   * On the desktop binding and on a long-lived container the request resolves
   * as soon as the scan is accepted, so treating that as completion announced
   * the run finished while every target was still being assessed.
   */
  public async scanAuthorisedScope(): Promise<string[]> {
    const domains = this.seedDomainsList();
    const repos = this.seedReposList();

    if (domains.length === 0 && repos.length === 0) {
      return [];
    }

    const failures: string[] = [];
    let issued = 0;
    const total = domains.length + repos.length;

    this.isScanning.set(true);
    this.scopeScanInFlight = true;
    this.pendingTargets = 0;
    this.targetsDone = 0;
    this.targetsTotal = total;
    // Cleared per run, so a partial result from an earlier scan cannot be
    // reported against this one.
    this.partialTargets = [];

    const settled = new Promise<void>(resolve => { this.allTargetsSettled = resolve; });

    try {
      for (const domain of domains) {
        this.setScanStatus(`Requesting ${domain} (${++issued} of ${total})…`);
        // Counted before the request is issued. An inline deployment publishes
        // completion before the response is written, so a target counted
        // afterwards could have its event arrive with nothing outstanding.
        this.pendingTargets++;
        try {
          await this.transport.triggerScan(domain, '');
        } catch (e) {
          console.error(`Trawl: scanning ${domain} failed:`, e);
          // A rejected request will never produce a completion event, so it
          // must be taken back off the count or the run would wait forever.
          this.pendingTargets--;
          failures.push(domain);
        }
      }

      for (const repoUrl of repos) {
        this.setScanStatus(`Requesting ${repoUrl} (${++issued} of ${total})…`);
        this.pendingTargets++;
        try {
          await this.transport.triggerScan('', repoUrl);
        } catch (e) {
          console.error(`Trawl: scanning ${repoUrl} failed:`, e);
          this.pendingTargets--;
          failures.push(repoUrl);
        }
      }

      if (this.pendingTargets > 0) {
        this.setScanStatus(
          `Scanning ${this.pendingTargets} target(s) - waiting for results…`
        );
        await this.awaitTargets(settled);
      }
    } finally {
      // Cleared before the final refresh so that onScanComplete treats this as
      // the end of the run and settles the scanning state.
      this.scopeScanInFlight = false;
      this.allTargetsSettled = null;
      this.pendingTargets = 0;
      await this.onScanComplete();
    }

    return failures;
  }

  /**
   * How long to wait for outstanding targets before settling anyway.
   *
   * A scan that never reports back would otherwise leave the banner spinning
   * for the rest of the session. Giving up eventually and saying so is more
   * useful than a spinner that means nothing — but the window is generous,
   * because settling early is the fault being fixed here and re-introducing it
   * under a different name would be no improvement.
   */
  private static readonly TARGET_WAIT_MS = 15 * 60 * 1000;

  private async awaitTargets(settled: Promise<void>): Promise<void> {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const expired = new Promise<'timeout'>(resolve => {
      timer = setTimeout(() => resolve('timeout'), WailsIpcService.TARGET_WAIT_MS);
    });

    try {
      if (await Promise.race([settled.then(() => 'done' as const), expired]) === 'timeout') {
        // Reported rather than swallowed. The results already ingested are
        // real, but the operator must not read this as a complete assessment.
        this.setScanStatus(
          `${this.pendingTargets} target(s) did not report back. ` +
          `Results shown are incomplete.`,
          'warning'
        );
      }
    } finally {
      clearTimeout(timer);
    }
  }

  public async refreshSecretFindings(repoUrl: string = ''): Promise<void> {
    const data = (await this.transport.getSecretFindings(repoUrl)) || [];
    this.secretFindings.set(
      data.map((s: any) => ({
        ...s,
        _id: s.id,
        provider: s.secretType,
        lineNumber: s.startLine,
        priority: 'high',
        status: 'open',
        commitSha: 'unknown'
      }))
    );
  }

  public async refreshEmailPostures(): Promise<void> {
    this.emailPostures.set((await this.transport.getEmailPostures()) || []);
  }

  /** Loads the stored measured-state assessments for every assessed domain. */
  public async refreshAssessments(): Promise<void> {
    if (this.assessmentRefreshRunning) {
      this.assessmentRefreshPending = true;
      return;
    }

    this.assessmentRefreshRunning = true;
    try {
      do {
        this.assessmentRefreshPending = false;
        this.assessments.set((await this.transport.getAssessments()) || []);
        this.markFresh();
      } while (this.assessmentRefreshPending);
    } finally {
      this.assessmentRefreshRunning = false;
    }
  }

  /**
   * Loads confirmed posture regressions.
   *
   * Failure leaves the previous list in place and marks the view stale rather
   * than emptying it. An empty regression list is a strong claim — nothing has
   * degraded — and it must never be the accidental result of a failed request.
   */
  public async refreshRegressions(): Promise<void> {
    try {
      this.regressions.set((await this.transport.getRegressions()) || []);
      this.markFresh();
    } catch (err) {
      console.error('Trawl: could not load regressions', err);
      this.streamHealthy.set(false);
    }
  }

  /** Records that the view has just been brought up to date with the store. */
  private markFresh(): void {
    this.lastUpdatedAt.set(new Date().toISOString());
    this.streamHealthy.set(true);
  }

  /**
   * Runs a fresh assessment of one domain and merges the result in place, so
   * the card the operator is looking at updates without the whole list
   * flickering.
   */
  public async assessDomain(domain: string, opts?: AssessOptionsUI): Promise<void> {
    const result = await this.transport.assessDomain(domain, opts);
    if (!result) {
      return;
    }
    const current = this.assessments();
    const index = current.findIndex(a => a.assetId === result.assetId);
    if (index >= 0) {
      const next = [...current];
      next[index] = result;
      this.assessments.set(next);
    } else {
      this.assessments.set([...current, result]);
    }
    if (opts?.discover || opts?.probeAddresses) {
      await this.refreshAssets();
    }
  }

  public async probeDiscoveredServices(profile: 'most-common' | 'extended'): Promise<void> {
    await this.transport.probeDiscoveredServices(profile);
    await Promise.all([this.refreshAssessments(), this.refreshFindings(''), this.refreshFindingViews()]);
  }

  /**
   * Erases everything the engine discovered, then reloads every view from the
   * store.
   *
   * It reloads rather than clearing the signals locally. Emptying them here
   * would show the operator an erased estate whether or not anything was
   * erased, and the difference matters most in exactly the case where the
   * erasure failed. It rethrows for the same reason: the caller has a person
   * to tell.
   */
  public async eraseDiscoveredData(): Promise<void> {
    await this.transport.eraseDiscoveredData();
    await this.refreshAll();
  }

  /**
   * Runs a discovery pass and replaces the proposal queue with the result.
   *
   * The proposals replace rather than accumulate: a stale proposal whose
   * certificate has since lapsed should leave the queue, and the operator's own
   * decisions live in the authorised and dismissed lists, so nothing they have
   * said is lost by rebuilding it.
   *
   * It rethrows. A failed search and an empty estate produce the same empty
   * list, and only an exception distinguishes "nothing more is out there" from
   * "we could not find out".
   */
  public async discoverRelatedDomains(options: DiscoveryOptions): Promise<DiscoveryResult> {
    this.isDiscovering.set(true);
    try {
      const result = await this.transport.discoverRelatedDomains(options);
      this.proposedDomains.set(result.proposals || []);
      this.lastDiscovery.set(result);
      return result;
    } finally {
      // Cleared in a finally so a failed pass does not leave the UI stuck
      // showing a spinner that will never resolve.
      this.isDiscovering.set(false);
    }
  }

  /**
   * Authorises proposals, bringing them into scope so they will be assessed.
   *
   * This is the moment authority is asserted, so the scope is reloaded from the
   * backend afterwards rather than patched locally: what the operator sees as
   * authorised must be what the engine will actually act upon.
   */
  public async authoriseProposedDomains(domains: string[]): Promise<void> {
    await this.transport.authoriseProposedDomains(domains);
    await this.loadSettings();
  }

  /** Rules proposals out and remembers it, so they are not proposed again. */
  public async dismissProposedDomains(domains: string[]): Promise<void> {
    await this.transport.dismissProposedDomains(domains);
    await this.loadSettings();
  }

  /** Undoes a dismissal, returning the domain to consideration. */
  public async restoreDismissedDomain(domain: string): Promise<void> {
    await this.transport.restoreDismissedDomain(domain);
    await this.loadSettings();
  }

  /**
   * Reads the engine's build identity.
   *
   * A failure leaves the previous value in place rather than blanking it: the
   * version does not change while the process runs, so a dropped request is
   * no reason to stop reporting what was already established.
   */
  public async refreshVersion(): Promise<void> {
    const info = await this.transport.getVersion();
    if (info?.version) {
      this.engineVersion.set(info.version);
    }
  }

  // -------------------------------------------------------------------------
  // Change 025: unified findings, history and change views
  // -------------------------------------------------------------------------

  private findingIndexRunning = false;
  private findingIndexPending = false;

  /**
   * Loads the unified findings list.
   *
   * Failure keeps the previous list and records the state as `error`: an empty
   * list is a claim about the estate, and it must not be the accidental result
   * of a request that failed. Overlapping calls coalesce, because progress
   * events can arrive faster than the list can be read.
   */
  public async refreshFindingIndex(): Promise<void> {
    if (this.findingIndexRunning) {
      this.findingIndexPending = true;
      return;
    }
    this.findingIndexRunning = true;
    try {
      do {
        this.findingIndexPending = false;
        try {
          this.findingIndex.set((await this.transport.getFindingIndex()) || []);
          this.findingIndexError.set('');
          this.findingIndexState.set('ready');
          this.markFresh();
        } catch (error) {
          console.error('Trawl: could not load the findings index', error);
          this.findingIndexError.set(error instanceof Error ? error.message : 'The findings index could not be loaded.');
          this.findingIndexState.set('error');
          this.streamHealthy.set(false);
        }
      } while (this.findingIndexPending);
    } finally {
      this.findingIndexRunning = false;
    }
  }

  public async refreshScanRuns(): Promise<void> {
    try {
      this.scanRuns.set((await this.transport.getScanRuns()) || []);
      this.scanRunsState.set('ready');
    } catch (error) {
      console.error('Trawl: could not load the scan ledger', error);
      this.scanRunsState.set('error');
    }
  }

  /**
   * Loads the run summaries the trend is drawn from. `since` is RFC 3339, or
   * empty for everything recorded.
   */
  public async refreshHistory(since: string = ''): Promise<void> {
    try {
      this.history.set((await this.transport.getHistorySummary(since)) || []);
      this.historyState.set('ready');
    } catch (error) {
      console.error('Trawl: could not load the history summary', error);
      this.historyState.set('error');
    }
  }

  /** Loads the default comparison: the latest run against the one before it. */
  public async refreshChanges(): Promise<void> {
    try {
      this.changeSet.set(normalizeChangeSet(await this.transport.getFindingChanges()));
      this.changeSetState.set('ready');
    } catch (error) {
      console.error('Trawl: could not load the change set', error);
      this.changeSetState.set('error');
    }
  }

  /** Everything the findings and change views read, refreshed together. */
  public async refreshFindingViews(): Promise<void> {
    await Promise.allSettled([
      this.refreshFindingIndex(),
      this.refreshScanRuns(),
      this.refreshHistory(this.historySince),
      this.refreshChanges()
    ]);
  }

  /** The `since` the trend was last loaded with, so a refresh keeps the range. */
  private historySince = '';

  /** Loads the trend for a chosen range and remembers it for later refreshes. */
  public async loadHistorySince(since: string): Promise<void> {
    this.historySince = since;
    await this.refreshHistory(since);
  }

  /** One finding's detail. Rejects on failure; null means the key is unknown. */
  public async getFindingDetail(key: string): Promise<FindingDetail | null> {
    return normalizeDetail(await this.transport.getFindingDetail(key));
  }

  /** An arbitrary comparison between two runs. Rejects on failure. */
  public async compareScans(from: string, to: string): Promise<ChangeSet> {
    return normalizeChangeSet(await this.transport.getFindingChanges(from, to));
  }

  // -------------------------------------------------------------------------
  // The finding address: #/findings/<key>
  //
  // The tab shell stays; the Angular Router is not used. A hash is enough to
  // make a detail linkable and to let the back button close it, and it needs
  // no server cooperation, which matters because the same bundle is served by
  // Wails and by nginx.
  // -------------------------------------------------------------------------

  /** The tab to return to when a detail opened from elsewhere is closed. */
  private findingReturnTab: AppTab | null = null;

  /** True while the current history entry was pushed by `openFinding`. */
  private findingEntryPushed = false;

  private initFindingAddress(): void {
    if (typeof window === 'undefined') {
      return;
    }

    // A shared address opens straight onto its finding.
    const initial = parseFindingHash(window.location.hash);
    if (initial) {
      this.selectedFindingKey.set(initial);
      this.activeTab.set('findings');
    }

    // Back, forward and a hand-edited address all arrive here.
    const onHashChange = () => this.syncFromAddress();
    window.addEventListener('hashchange', onHashChange);
    inject(DestroyRef).onDestroy(() => window.removeEventListener('hashchange', onHashChange));

    // Leaving the Findings tab by any route closes an open detail, so the
    // address never names a finding the screen is not showing.
    effect(() => {
      if (this.activeTab() !== 'findings' && this.selectedFindingKey()) {
        untracked(() => this.closeFinding({ restoreTab: false }));
      }
    });
  }

  private syncFromAddress(): void {
    const key = parseFindingHash(window.location.hash);
    if (key === this.selectedFindingKey()) {
      return;
    }
    if (key) {
      this.findingReturnTab = null;
      this.findingEntryPushed = false;
      this.selectedFindingKey.set(key);
      this.activeTab.set('findings');
      return;
    }
    this.findingEntryPushed = false;
    this.finishClose(true);
  }

  /**
   * Opens a finding's detail and gives it a stable address.
   *
   * The first open pushes a history entry, so the back button closes the
   * detail; opening another finding while one is open replaces that entry
   * instead of stacking them.
   */
  public openFinding(key: string): void {
    if (!key) {
      return;
    }
    const alreadyOpen = this.selectedFindingKey() !== null;
    if (!alreadyOpen) {
      this.findingReturnTab = this.activeTab() === 'findings' ? null : this.activeTab();
    }
    this.selectedFindingKey.set(key);
    this.activeTab.set('findings');

    if (typeof window === 'undefined') {
      return;
    }
    const hash = findingHash(key);
    if (window.location.hash === hash) {
      return;
    }
    if (alreadyOpen && this.findingEntryPushed) {
      window.history.replaceState(null, '', hash);
    } else {
      window.location.hash = hash;
      this.findingEntryPushed = true;
    }
  }

  /**
   * Closes the open detail.
   *
   * Where `openFinding` pushed an entry, closing walks it back, so the history
   * stack ends as it began. A detail that was opened by arriving at its
   * address has nothing behind it to return to, so the hash is cleared in
   * place instead.
   */
  public closeFinding(options: { restoreTab?: boolean } = {}): void {
    if (this.selectedFindingKey() === null) {
      return;
    }
    const restoreTab = options.restoreTab ?? true;
    const pushed = this.findingEntryPushed;
    this.findingEntryPushed = false;
    this.finishClose(restoreTab);

    if (typeof window === 'undefined' || parseFindingHash(window.location.hash) === null) {
      return;
    }
    if (pushed) {
      window.history.back();
    } else {
      window.history.replaceState(null, '', window.location.pathname + window.location.search);
    }
  }

  private finishClose(restoreTab: boolean): void {
    const back = this.findingReturnTab;
    this.findingReturnTab = null;
    this.selectedFindingKey.set(null);
    if (restoreTab && back) {
      this.activeTab.set(back);
    }
  }

  /** Reloads every view from the store. */
  public async refreshAll(): Promise<void> {
    if (this.loadState() === 'loading' && !this.lastUpdatedAt()) {
      // First load: leave the state as `loading` so views show skeletons
      // rather than an empty estate they have no basis for claiming.
    } else {
      this.loadState.set('loading');
    }

    // `allSettled`, not `all`. A rejected member of `all` abandons the results
    // of everything that succeeded, so one failing endpoint would blank views
    // that had perfectly good data — and an empty view is a claim, not a
    // neutral state. Here each loader keeps whatever it managed to fetch and
    // the failures are reported alongside it.
    const results = await Promise.allSettled([
      this.refreshVersion(),
      this.refreshAssets(),
      this.refreshFindings(''),
      this.refreshFeedStatuses(),
      this.refreshSecretFindings(),
      this.refreshEmailPostures(),
      this.refreshAssessments(),
      this.refreshRegressions(),
      this.refreshFindingViews()
    ]);
    const failed = results.filter(r => r.status === 'rejected') as PromiseRejectedResult[];
    if (failed.length === 0) {
      this.loadError.set('');
      this.loadState.set('ready');
      return;
    }

    this.loadError.set(
      failed.length === results.length
        ? 'Could not reach the engine. Nothing below has been loaded.'
        : `${failed.length} of ${results.length} views could not be loaded. What is shown is incomplete.`
    );
    this.loadState.set('error');
    this.streamHealthy.set(false);
  }
}
