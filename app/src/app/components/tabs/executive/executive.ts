import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import {
  coverageLabel,
  DEFAULT_COVERAGE_FLOOR,
  estateCoverage,
  headlinePosture,
  isSnapshotStale,
  kevEnrichmentSummary,
  rankFindings,
  unresolvedAssessments,
  withheldChecks
} from '../../../dashboard/estate';

/**
 * The executive area: what is exposed, how much of it we have actually looked
 * at, what became worse, and what to do first.
 *
 * Three rules govern this component, and each exists because the obvious
 * alternative is wrong in a way that is hard to detect afterwards.
 *
 * It computes nothing of its own. Every figure comes from the pure functions
 * in dashboard/estate.ts, over the same rows the detail tabs read, so an
 * executive number and its detail view cannot drift apart.
 *
 * Coverage is rendered inside the figure it qualifies, never beside it. A
 * number travels — into a board pack, an email, a screenshot — and a caveat
 * placed next to it does not travel with it. Any design where the reassurance
 * is separable from its qualification will, in practice, separate.
 *
 * It states no monetary value and no probability of compromise. Those require
 * the model packs and loss model of Changes 008-011, and a risk score invented
 * here would be a number with no calibration behind it — which is precisely
 * what the project's sixth guardrail forbids.
 */
@Component({
  selector: 'app-executive',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './executive.html'
})
export class ExecutiveComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;

  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;
  assessments = this.wailsIpc.assessments;
  regressions = this.wailsIpc.regressions;
  secretFindings = this.wailsIpc.secretFindings;
  isAuthorized = this.wailsIpc.isAuthorized;
  lastUpdatedAt = this.wailsIpc.lastUpdatedAt;
  streamHealthy = this.wailsIpc.streamHealthy;
  loadError = this.wailsIpc.loadError;

  /**
   * True only before the store has ever been read.
   *
   * Skeletons are shown on the first load and never afterwards. On a refresh
   * the previous figures stay on screen, because replacing a real number with
   * a shimmering placeholder tells the operator less than the slightly stale
   * number it replaced.
   */
  readonly isLoading = computed(
    () => this.wailsIpc.loadState() === 'loading' && this.lastUpdatedAt() === null
  );

  /** True when the last load failed, in whole or in part. */
  readonly hasLoadError = computed(() => this.wailsIpc.loadState() === 'error');

  /** The floor beneath which no headline figure is offered. */
  readonly coverageFloor = signal(DEFAULT_COVERAGE_FLOOR);

  /** How many ranked findings to show before the operator asks for more. */
  private readonly rankedLimit = signal(5);

  readonly floorPercent = computed(() => Math.round(this.coverageFloor() * 100));

  readonly posture = computed(() => headlinePosture(this.assessments(), this.coverageFloor()));

  readonly coverage = computed(() => estateCoverage(this.assessments()));

  readonly coverageText = computed(() => coverageLabel(this.coverage()));

  readonly ranked = computed(() => rankFindings(this.findings()));

  readonly topRanked = computed(() => this.ranked().slice(0, this.rankedLimit()));

  readonly withheld = computed(() => withheldChecks(this.assessments()));

  readonly unresolved = computed(() => unresolvedAssessments(this.assessments()));

  readonly activeAssets = computed(() => this.assets().filter(a => a.status === 'active').length);

  readonly kevSummary = computed(() =>
    kevEnrichmentSummary(this.findings(), this.wailsIpc.feedStatuses())
  );

  /**
   * One row per external catalogue, for the "Vulnerability catalogues" list.
   * "Last refreshed" is the last successful consultation of the source;
   * "catalogue retrieved" is when the content in force arrived, and the two
   * differ whenever a refresh found nothing new.
   */
  readonly feedRows = computed(() => {
    const labels: Record<string, string> = {
      'cisa-kev': 'CISA Known Exploited Vulnerabilities',
      epss: 'EPSS exploit-likelihood scores',
      nvd: 'NVD vulnerability details'
    };
    const statuses = this.wailsIpc.feedStatuses();
    return ['cisa-kev', 'epss', 'nvd'].map(feed => {
      const status = statuses.find(s => s.feed === feed);
      return {
        feed,
        label: labels[feed],
        hasSnapshot: !!status?.snapshot,
        checkedAt: status?.lastCheckedAt ?? status?.snapshot?.retrievedAt ?? null,
        retrievedAt: status?.snapshot?.retrievedAt ?? null,
        entries: status?.snapshot?.recordCount ?? null,
        entriesStored: status?.recordsStored ?? false,
        stale: status?.stale ?? false,
        error: status?.lastError ?? null
      };
    });
  });

  readonly verifiedSecrets = computed(() => this.secretFindings().filter(s => s.verified).length);

  /**
   * Unconfirmed software-matched candidates, counted separately from confirmed
   * exposure so version-based matches never inflate confirmed headline figures.
   */
  readonly unconfirmedCandidates = computed(() =>
    this.wailsIpc.findingIndex().filter(f => f.source === 'candidate' && f.status !== 'resolved')
  );

  readonly unconfirmedKevCandidates = computed(() =>
    this.unconfirmedCandidates().filter(f => f.kev).length
  );

  /**
   * Regressions ordered by confirmation recency.
   *
   * Presented apart from new findings throughout, because "this became worse"
   * and "we found this" call for different responses: one is a change to
   * investigate, the other is a backlog item to schedule.
   */
  readonly recentRegressions = computed(() =>
    this.regressions()
      .slice()
      .sort((a, b) => (b.confirmedAt ?? '').localeCompare(a.confirmedAt ?? ''))
  );

  /**
   * True when the estate has been assessed at all.
   *
   * Distinguishes "we found nothing" from "we have not looked", which must
   * never share a rendering. Both present as an empty table, and only one of
   * them is good news.
   */
  readonly hasAssessed = computed(() => this.coverage().concluded > 0);

  /** Whether anything at all has been discovered yet. */
  readonly hasEstate = computed(() => this.assets().length > 0);

  readonly calibrationExperimentId = signal('');
  readonly calibrationExperimentLabel = signal('');

  constructor() {
    void this.wailsIpc.loadCalibrationExperiments();
  }

  loadCalibrationReport(): void {
    const experimentId = this.calibrationExperimentId().trim();
    if (!experimentId) {
      this.wailsIpc.calibrationReportError.set('Enter an experiment identifier first.');
      return;
    }
    void this.wailsIpc.loadCalibrationReport(experimentId);
  }

  async createCalibrationExperiment(): Promise<void> {
    const label = this.calibrationExperimentLabel().trim();
    if (!label) {
      this.wailsIpc.calibrationReportError.set('Enter a label for the new experiment.');
      return;
    }
    try {
      const experiment = await this.wailsIpc.createCalibrationExperiment(label);
      this.calibrationExperimentId.set(experiment.id);
      this.calibrationExperimentLabel.set('');
      await this.wailsIpc.loadCalibrationReport(experiment.id);
    } catch (error) {
      this.wailsIpc.calibrationReportError.set(error instanceof Error ? error.message : 'Experiment could not be created.');
    }
  }

  showAllRanked(): void {
    this.rankedLimit.set(Number.MAX_SAFE_INTEGER);
  }

  goToScope(): void {
    this.wailsIpc.activeTab.set('scope');
  }

  goToFindings(): void {
    this.wailsIpc.activeTab.set('findings');
  }

  goToAssets(): void {
    this.wailsIpc.activeTab.set('assets');
  }

  goToEmail(): void {
    this.wailsIpc.activeTab.set('email');
  }

  goToSecrets(): void {
    this.wailsIpc.activeTab.set('secrets');
  }

  /** A relative time for the freshness line, so staleness is legible at a glance. */
  relativeTime(iso: string | null): string {
    if (!iso) {
      return 'never';
    }
    const then = new Date(iso).getTime();
    if (Number.isNaN(then)) {
      return 'unknown';
    }
    const seconds = Math.floor((Date.now() - then) / 1000);
    if (seconds < 60) {
      return 'just now';
    }
    if (seconds < 3600) {
      const m = Math.floor(seconds / 60);
      return `${m} minute${m === 1 ? '' : 's'} ago`;
    }
    if (seconds < 86400) {
      const h = Math.floor(seconds / 3600);
      return `${h} hour${h === 1 ? '' : 's'} ago`;
    }
    const d = Math.floor(seconds / 86400);
    return `${d} day${d === 1 ? '' : 's'} ago`;
  }

  /** Human phrasing for a regression's attribute, avoiding raw column names. */
  attributeLabel(attribute: string): string {
    const labels: Record<string, string> = {
      dmarc_policy: 'DMARC policy',
      tls_version: 'TLS version',
      open_ports: 'Open ports',
      http_scheme: 'HTTP scheme',
      spf_policy: 'SPF policy'
    };
    return labels[attribute] ?? attribute.replace(/_/g, ' ');
  }

  epssPercent(epss: number): string {
    return `${(epss * 100).toFixed(1)}%`;
  }

  enrichmentAgeLabel(finding: any, feed: string): { text: string; stale: boolean } {
    const enrichment = finding.enrichments?.find((e: any) => e.feed === feed);
    const retrievedAt =
      enrichment?.snapshot?.retrievedAt ??
      this.wailsIpc.feedStatuses().find(s => s.feed === feed)?.snapshot?.retrievedAt ??
      null;
    if (!retrievedAt) {
      if (enrichment?.state === 'check_failed') {
        return { text: 'check failed', stale: false };
      }
      return { text: 'not checked', stale: false };
    }
    return {
      text: `retrieved ${this.relativeTime(retrievedAt)}`,
      stale: isSnapshotStale(retrievedAt)
    };
  }

  refreshFeeds(): void {
    void this.wailsIpc.refreshFeeds();
  }

  refreshFeed(feed: string): void {
    void this.wailsIpc.refreshFeed(feed);
  }
}
