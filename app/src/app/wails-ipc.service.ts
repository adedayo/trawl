import { Injectable, signal } from '@angular/core';
import { DomainAssessment } from './models/types';
import { createTransport, DiscoveryOptions, DiscoveryResult, ProposedDomain, Scope, TrawlTransport } from './transport';

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

  // UI State
  public theme = signal<'light' | 'dark'>('light');
  public activeTab = signal<'overview' | 'assets' | 'findings' | 'email' | 'secrets' | 'scope'>('overview');

  // Data State
  public assets = signal<any[]>([]);
  public findings = signal<any[]>([]);
  public emailPostures = signal<any[]>([]);
  public secretFindings = signal<any[]>([]);

  /**
   * Measured-state assessments from vantage: four-state coverage, derived
   * control postures and the advisories behind them.
   */
  public assessments = signal<DomainAssessment[]>([]);

  /** Per-check progress from the running assessment, for live feedback. */
  public assessmentProgress = signal<{ domain: string; check: string; done: number; total: number } | null>(null);

  // Engine State
  public isScanning = signal<boolean>(false);
  public scanStatusMessage = signal<string>('');

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
    this.transport.on('asset:updated', () => this.refreshAssets());

    this.transport.on('finding:new', (payload: any) => {
      if (payload?.assetId) {
        this.refreshFindings(payload.assetId);
      }
    });

    this.transport.on('scan:progress', (payload: any) => {
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
          `Scanned ${this.targetsDone} of ${this.targetsTotal} — ${this.pendingTargets} still running…`
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
      this.refreshSecretFindings(),
      this.refreshEmailPostures(),
      this.refreshAssessments(),
      this.refreshAssets()
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
    this.assets.set((await this.transport.getAssets(status)) || []);
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
    await this.refreshAssets();
  }

  public async refreshFindings(assetId: string): Promise<void> {
    this.findings.set((await this.transport.getFindings(assetId)) || []);
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
          `Scanning ${this.pendingTargets} target(s) — waiting for results…`
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
    this.assessments.set((await this.transport.getAssessments()) || []);
  }

  /**
   * Runs a fresh assessment of one domain and merges the result in place, so
   * the card the operator is looking at updates without the whole list
   * flickering.
   */
  public async assessDomain(domain: string): Promise<void> {
    const result = await this.transport.assessDomain(domain);
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

  /** Reloads every view from the store. */
  public async refreshAll(): Promise<void> {
    await Promise.all([
      this.refreshAssets(),
      this.refreshSecretFindings(),
      this.refreshEmailPostures(),
      this.refreshAssessments()
    ]);
    this.findings.set([]);
  }
}
