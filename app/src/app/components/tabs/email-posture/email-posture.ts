import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { RegressionUI } from '../../../dashboard/estate';
import {
  ControlPosture,
  ControlView,
  CoverageState,
  DomainAssessment,
  EMAIL_POSTURE_CONTROLS,
  EmailPostureControl,
  EmailPostureUI,
  ScenarioView,
  SignalView
} from '../../../models/types';

/**
 * One email-authentication control as this view renders it.
 *
 * There is exactly one of these per control per domain, and it is the only
 * place that control's state is stated. The posture record supplies the state;
 * the assessment pipeline supplies the advisories raised against the same
 * control. Rendering the assessment's own derived posture beside the record
 * would give the operator two accounts of one control and no way to choose
 * between them.
 */
export interface PostureControlView {
  key: EmailPostureControl;
  label: string;
  state: CoverageState;
  /** The salient observed value, where the control has one. */
  detail: string;
  /** Why the state is not an assessment. Never invented. */
  reason: string;
  /**
   * Whether the state may be read as a fact about the domain rather than
   * about the limits of the search. False only for a probed DKIM.
   */
  conclusive: boolean;
  /** False when no posture record exists, so the state is our ignorance. */
  recorded: boolean;
  /** Advisories the assessment raised against this control. */
  signals: SignalView[];
  /** Observed values behind the state, verbatim from the record. */
  evidence: string[];
  /** Qualifications the evidence carries, such as partial enforcement. */
  notes: string[];
}

/**
 * A domain as the view assembles it: the posture record, the assessment, or
 * both.
 *
 * A domain with a posture and no assessment is still shown. Dropping it would
 * hide an assessed domain because a second, unrelated pipeline had not run.
 */
export interface DomainRow {
  domain: string;
  assetId: string;
  assessment: DomainAssessment | null;
  posture: EmailPostureUI | null;
  controls: PostureControlView[];
}

/**
 * Renders the measured-state assessment for each domain.
 *
 * The governing rule of this component is that it never collapses the
 * four-state coverage model into a tick or a cross. "Not published", "not
 * checked" and "could not tell" are distinct, and a control that was never
 * assessed is shown as unassessed rather than as passing — because an operator
 * reading a green card concludes they are protected, and that conclusion must
 * be earned by an assessment that actually happened.
 *
 * The second rule is that nothing here derives a severity. `priority` is
 * computed in Go from the recorded tags and rendered verbatim beside them, so
 * a rating can always be checked against the record that produced it. A client
 * that adjusted a severity would be a second, unreviewable risk model.
 */
@Component({
  selector: 'app-email-posture',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './email-posture.html'
})
export class EmailPostureComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  assessments = this.wailsIpc.assessments;
  postures = this.wailsIpc.emailPostures;
  regressions = this.wailsIpc.regressions;
  progress = this.wailsIpc.assessmentProgress;

  /** Which control panels the operator has opened, keyed "domain::control". */
  private expanded = signal<Set<string>>(new Set());

  /**
   * Which domain rows the operator has opened.
   *
   * A portfolio is read by scanning for the domain that needs attention, not
   * by reading every domain in full. So the list collapses to one row each,
   * and the row must carry enough — posture of the headline controls, how much
   * concluded, how many advisories — to decide whether to open it. Detail is
   * one click away and unchanged when it arrives.
   */
  private openDomains = signal<Set<string>>(new Set());

  /** Domains currently being re-assessed, so their button can show progress. */
  private running = signal<Set<string>>(new Set());

  /**
   * The controls summarised on a collapsed row. These are the ones that decide
   * whether mail from the domain can be spoofed or intercepted, so they are
   * the ones worth showing before the operator has asked for detail.
   */
  private static readonly HEADLINE_CONTROLS: EmailPostureControl[] = ['spf', 'dkim', 'dmarc', 'mtaSts'];

  /** How a posture control is named in the assessment pipeline's vocabulary. */
  private static readonly ASSESSMENT_CONTROL: Record<EmailPostureControl, string> = {
    spf: 'spf', dkim: 'dkim', dmarc: 'dmarc', mtaSts: 'mta-sts',
    tlsRpt: 'tls-rpt', bimi: 'bimi', caa: 'caa'
  };

  private static readonly CONTROL_LABEL: Record<EmailPostureControl, string> = {
    spf: 'SPF', dkim: 'DKIM', dmarc: 'DMARC', mtaSts: 'MTA-STS',
    tlsRpt: 'TLS-RPT', bimi: 'BIMI', caa: 'CAA'
  };

  /**
   * Every domain either pipeline knows about, each with one account of each
   * control.
   */
  rows = computed<DomainRow[]>(() => {
    const remaining = new Map((this.postures() ?? []).map(p => [p.domain, p]));
    const rows: DomainRow[] = [];

    for (const a of this.assessments()) {
      const posture = remaining.get(a.domain) ?? null;
      remaining.delete(a.domain);
      rows.push(this.buildRow(a.domain, a.assetId, a, posture));
    }
    for (const p of remaining.values()) {
      rows.push(this.buildRow(p.domain, p.domain, null, p));
    }

    return rows.sort((x, y) => x.domain.localeCompare(y.domain));
  });

  private buildRow(
    domain: string,
    assetId: string,
    assessment: DomainAssessment | null,
    posture: EmailPostureUI | null
  ): DomainRow {
    return {
      domain,
      assetId,
      assessment,
      posture,
      controls: EMAIL_POSTURE_CONTROLS.map(key => this.buildControl(key, assessment, posture))
    };
  }

  private buildControl(
    key: EmailPostureControl,
    assessment: DomainAssessment | null,
    posture: EmailPostureUI | null
  ): PostureControlView {
    const recorded = posture ? posture[key] : undefined;
    const assessmentName = EmailPostureComponent.ASSESSMENT_CONTROL[key];
    const signals = assessment?.controls.find(c => c.control === assessmentName)?.signals ?? [];

    return {
      key,
      label: EmailPostureComponent.CONTROL_LABEL[key],
      // No posture record means no assessment of this control, and that is
      // said in the same vocabulary as any other unassessed control rather
      // than through an absence the reader has to notice.
      state: recorded?.state ?? 'not_checked',
      detail: recorded?.detail ?? '',
      reason: recorded
        ? (recorded.reason ?? '')
        : 'No posture record has been stored for this domain.',
      conclusive: recorded?.conclusive ?? false,
      recorded: !!recorded,
      signals,
      evidence: posture ? this.evidenceFor(key, posture) : [],
      notes: posture ? this.notesFor(key, posture) : []
    };
  }

  // ─── Evidence ──────────────────────────────────────────────────────────────

  /**
   * The observed values a rating was derived from.
   *
   * Shown beside the rating rather than behind a further request, because a
   * severity a reader cannot check against its evidence is a severity they
   * must take on trust.
   */
  private evidenceFor(key: EmailPostureControl, p: EmailPostureUI): string[] {
    switch (key) {
      case 'dmarc': return this.dmarcEvidence(p);
      case 'spf': return this.spfEvidence(p);
      case 'dkim': return this.dkimEvidence(p);
      default: {
        const detail = p[key]?.detail;
        return detail ? [detail] : [];
      }
    }
  }

  dmarcEvidence(p: EmailPostureUI): string[] {
    if (!this.isAssessed(p.dmarc.state) || p.dmarc.state === 'not_found') {
      return [];
    }
    const evidence: string[] = [];
    evidence.push(p.dmarcPolicy ? `p=${p.dmarcPolicy}` : 'no policy tag published');
    if (p.dmarcPolicy) {
      evidence.push(`pct=${p.dmarcPercent}`);
    }
    if (p.dmarcSubdomainPolicy) {
      evidence.push(`sp=${p.dmarcSubdomainPolicy}`);
    }
    if (p.dmarcAlignmentSpf) {
      evidence.push(`aspf=${p.dmarcAlignmentSpf}`);
    }
    if (p.dmarcAlignmentDkim) {
      evidence.push(`adkim=${p.dmarcAlignmentDkim}`);
    }
    evidence.push(p.dmarcReporting ? 'aggregate reporting published' : 'no aggregate reporting');
    return evidence;
  }

  spfEvidence(p: EmailPostureUI): string[] {
    if (!this.isAssessed(p.spf.state) || p.spf.state === 'not_found') {
      return [];
    }
    const evidence: string[] = [];
    if (p.spfAllMechanism) {
      evidence.push(`${p.spfAllMechanism}all`);
    }
    evidence.push(`${p.spfLookups} DNS-querying mechanism(s)`);
    return evidence;
  }

  dkimEvidence(p: EmailPostureUI): string[] {
    const examined = p.dkimSelectorsExamined ?? [];
    const found = p.dkimSelectorsFound ?? [];
    if (examined.length === 0 && found.length === 0) {
      return [];
    }
    // Both figures, always. "A key exists" and "one key from thirteen
    // selectors tried" are different statements about the same domain.
    const evidence = [`${found.length} of ${examined.length} selector(s) examined yielded a key`];
    if (found.length > 0) {
      evidence.push(`found: ${found.join(', ')}`);
    }
    if (examined.length > 0) {
      evidence.push(`examined: ${examined.join(', ')}`);
    }
    return evidence;
  }

  /** Qualifications without which the evidence would be read too strongly. */
  private notesFor(key: EmailPostureControl, p: EmailPostureUI): string[] {
    const notes: string[] = [];

    if (key === 'dmarc' && this.isAssessed(p.dmarc.state) && p.dmarc.state !== 'not_found') {
      if ((p.dmarcPolicy === 'reject' || p.dmarcPolicy === 'quarantine') && p.dmarcPercent < 100) {
        // A badge reading "reject" would say the route is closed. At pct=40
        // it is open three times in five.
        notes.push(
          `Partial enforcement: the policy is applied to ${p.dmarcPercent}% of mail. ` +
          `The remaining ${100 - p.dmarcPercent}% is handled as though no policy were published.`
        );
      }
      if (p.dmarcPolicy === 'none') {
        notes.push('Monitoring only: spoofed mail is reported where reporting is configured, and delivered either way.');
      }
      if ((p.dmarcPolicy === 'reject' || p.dmarcPolicy === 'quarantine') && p.dmarcSubdomainPolicy === 'none') {
        notes.push('The apex is enforced and subdomains are not, which is the route an attacker will take.');
      }
      if (p.dmarcPolicy === 'quarantine' && p.dmarcPercent === 100) {
        notes.push('Quarantine delivers to junk rather than refusing.');
      }
    }

    if (key === 'spf' && this.isAssessed(p.spf.state) && p.spf.state !== 'not_found') {
      if (p.spfAllMechanism === '+') {
        notes.push('+all authorises every host on the internet to send as this domain.');
      }
      if (p.spfAllMechanism === '?') {
        notes.push('?all authorises nothing either way, so a receiver learns nothing from the record.');
      }
      if (p.spfLookups > 10) {
        notes.push(
          'Above the limit of ten DNS-querying mechanisms. Receivers may return permerror and ' +
          'stop evaluating, so this policy may be enforced nowhere.'
        );
      }
    }

    if (key === 'dkim' && p.dkim.state === 'not_found' && !p.dkim.conclusive) {
      notes.push(
        'Selectors cannot be enumerated from DNS. Nothing was found at the selectors examined, ' +
        'which does not establish that the domain has no DKIM key.'
      );
    }

    return notes;
  }

  // ─── Coverage ──────────────────────────────────────────────────────────────

  /**
   * Whether a state reached a conclusion.
   *
   * This decides only whether evidence is worth rendering. The coverage figure
   * itself is never derived here — it arrives computed, from the one
   * definition that also gates severity.
   */
  isAssessed(state: CoverageState): boolean {
    return state === 'ok' || state === 'not_found';
  }

  /**
   * How many controls were assessed, in the engine's own words.
   *
   * Read from the record, never recomputed. It is computed in Go by
   * `EmailPosture.Assessed`, the same function severity is gated on, so the
   * coverage shown and the coverage reasoned over cannot diverge.
   */
  coverageLine(row: DomainRow): string {
    const p = row.posture;
    if (!p) {
      return 'no posture record';
    }
    if (p.assessedControls === undefined || p.totalControls === undefined) {
      return 'control coverage not reported by this build';
    }
    return `${p.assessedControls}/${p.totalControls} controls assessed`;
  }

  /** True when the record carries the engine's coverage figure. */
  hasCoverage(row: DomainRow): boolean {
    return row.posture?.assessedControls !== undefined && row.posture?.totalControls !== undefined;
  }

  // ─── Severity ──────────────────────────────────────────────────────────────

  /**
   * The engine's rating, verbatim.
   *
   * An empty priority is not "info" and not "low": it means the controls the
   * rating would be derived from were not assessed. Substituting a floor here
   * would let a resolver outage manufacture a finding.
   */
  priority(row: DomainRow): string {
    return row.posture?.priority ?? '';
  }

  hasPriority(row: DomainRow): boolean {
    return !!this.priority(row);
  }

  /**
   * How many domains carry a rating at all.
   *
   * Unrated domains are excluded rather than counted as clean, so the figure
   * states what was rated rather than implying the rest were found healthy.
   */
  ratedCount = computed(() => this.rows().filter(r => this.hasPriority(r)).length);

  // ─── Drift ─────────────────────────────────────────────────────────────────

  /**
   * Recorded DMARC policy changes for a domain.
   *
   * Only confirmed regressions appear, because the engine writes no
   * fingerprint for an assessment that did not conclude — so an outage cannot
   * present itself here as a weakened policy.
   */
  driftFor(row: DomainRow): RegressionUI[] {
    return (this.regressions() ?? []).filter(
      r => r.attributeType === 'dmarc_policy' && (r.assetId === row.assetId || r.assetId === row.domain)
    );
  }

  driftLine(r: RegressionUI): string {
    const from = r.previousValue || 'an unrecorded policy';
    const to = r.currentValue || 'an unrecorded policy';
    return `DMARC policy changed from ${from} to ${to}.`;
  }

  // ─── Presentation of the four states ───────────────────────────────────────

  /**
   * Plain-language wording for one of the four coverage states.
   *
   * No state shares wording with `ok`, and neither `not_checked` nor
   * `check_failed` asserts anything about the domain — they are statements
   * about our knowledge, and they read that way.
   */
  stateLabel(s: CoverageState): string {
    switch (s) {
      case 'ok': return 'Assessed';
      case 'not_found': return 'Not published';
      case 'not_checked': return 'Not checked';
      default: return 'Could not tell';
    }
  }

  /**
   * The wording for one control, which is the state's wording except where
   * the state cannot bear it.
   *
   * A DKIM `not_found` from a probed selector list is not absence, so it is
   * never worded as absence.
   */
  controlHeadline(c: PostureControlView): string {
    if (c.key === 'dkim' && c.state === 'not_found' && !c.conclusive) {
      return 'No key at the selectors examined';
    }
    return this.stateLabel(c.state);
  }

  /**
   * A shape per state, so the four remain distinguishable without colour.
   *
   * Deliberately not a tick or a cross: two of the four states are about what
   * we failed to learn, and a cross would read as a finding about the domain.
   */
  stateGlyph(s: CoverageState): string {
    switch (s) {
      case 'ok': return '●';
      case 'not_found': return '○';
      case 'not_checked': return '–';
      default: return '?';
    }
  }

  stateClasses(s: CoverageState): string {
    switch (s) {
      case 'ok': return 'text-emerald-500';
      case 'not_found': return 'text-rose-500';
      case 'not_checked': return 'text-slate-400';
      default: return 'text-amber-500';
    }
  }

  /** Four visually separable badges, so a row is legible before it is opened. */
  stateBadgeClasses(s: CoverageState, dark: boolean): string {
    switch (s) {
      case 'ok':
        return dark
          ? 'bg-emerald-950 border-emerald-800 text-emerald-400'
          : 'bg-emerald-50 border-emerald-300 text-emerald-800';
      case 'not_found':
        return dark
          ? 'bg-rose-950 border-rose-800 text-rose-400'
          : 'bg-rose-50 border-rose-300 text-rose-800';
      case 'not_checked':
        // Slate, not grey-as-good: it is a gap in knowledge, and the wording
        // beside it says so rather than relying on the shade.
        return dark
          ? 'bg-slate-900 border-slate-700 text-slate-300'
          : 'bg-slate-100 border-slate-300 text-slate-700';
      default:
        return dark
          ? 'bg-amber-950 border-amber-800 text-amber-400'
          : 'bg-amber-50 border-amber-300 text-amber-800';
    }
  }

  /**
   * The title text on a collapsed row's chip. Carries the control name, the
   * state in words and the reason, so the row survives without colour vision
   * and reads correctly to a screen reader.
   */
  controlTitle(c: PostureControlView): string {
    const parts = [`${c.label}: ${this.controlHeadline(c)}`];
    if (c.reason) {
      parts.push(c.reason);
    }
    return parts.join(' — ');
  }

  /** True where the state owes the reader an explanation. */
  needsReason(c: PostureControlView): boolean {
    return c.state === 'not_checked' || c.state === 'check_failed';
  }

  /**
   * What to say when an inconclusive control recorded no reason.
   *
   * The gap is disclosed rather than filled. Inferring a reason from the state
   * would manufacture evidence, and a reader acting on it would be acting on
   * something nobody observed.
   */
  reasonText(c: PostureControlView): string {
    if (!this.needsReason(c)) {
      return c.reason;
    }
    return c.reason || 'No reason was recorded for this outcome.';
  }

  // ─── Rows ──────────────────────────────────────────────────────────────────

  isExpanded(domain: string, control: string): boolean {
    return this.expanded().has(`${domain}::${control}`);
  }

  isOpen(domain: string): boolean {
    return this.openDomains().has(domain);
  }

  toggleDomain(domain: string): void {
    const next = new Set(this.openDomains());
    if (next.has(domain)) {
      next.delete(domain);
    } else {
      next.add(domain);
    }
    this.openDomains.set(next);
  }

  expandAll(): void {
    this.openDomains.set(new Set(this.rows().map(r => r.domain)));
  }

  collapseAll(): void {
    this.openDomains.set(new Set());
  }

  /**
   * The headline controls for a collapsed row, in a fixed order.
   *
   * A control that was never assessed is emitted as unassessed rather than
   * omitted. Dropping it would make a domain with four assessed controls and a
   * domain with one look alike at a glance, which is the exact confusion the
   * four-state model exists to prevent.
   */
  summaryControls(row: DomainRow): PostureControlView[] {
    return EmailPostureComponent.HEADLINE_CONTROLS.map(
      key => row.controls.find(c => c.key === key)!
    );
  }

  /** Total advisories raised across every control on a domain. */
  advisoryCount(a: DomainAssessment): number {
    return a.controls.reduce((n, c) => n + c.signals.length, 0);
  }

  /** Advisories at medium severity or above, which drive the row's urgency. */
  significantCount(a: DomainAssessment): number {
    return a.controls.reduce(
      (n, c) => n + c.signals.filter(
        s => s.severity === 'critical' || s.severity === 'high' || s.severity === 'medium'
      ).length,
      0
    );
  }

  /**
   * One line describing the domain's standing, so a collapsed row can be read
   * without opening it. Coverage is stated before any judgement, because a
   * clean result over two controls is not the same claim as a clean result
   * over seven and must not read like one.
   */
  summaryLine(row: DomainRow): string {
    const a = row.assessment;
    if (a && (a.outcome === 'refused' || a.outcome === 'failed' || a.outcome === 'cancelled')) {
      return this.outcomeNote(a);
    }

    const parts = [this.coverageLine(row)];
    if (!a) {
      return parts[0];
    }

    const significant = this.significantCount(a);
    const total = this.advisoryCount(a);
    const scope = `${a.coverage.assessedOnly}/${a.coverage.total} checks concluded`;
    if (total === 0) {
      parts.push(
        a.coverage.total === 0
          ? 'nothing has been assessed for this domain yet'
          : `no advisory raised across ${scope}`
      );
    } else {
      const tail = significant > 0
        ? `${significant} at medium or above`
        : 'all low or informational';
      parts.push(`${total} advisory(s) — ${tail}, over ${scope}`);
    }
    return parts.join(' · ');
  }

  toggle(domain: string, control: string): void {
    const key = `${domain}::${control}`;
    const next = new Set(this.expanded());
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    this.expanded.set(next);
  }

  isRunning(domain: string): boolean {
    return this.running().has(domain);
  }

  async reassess(domain: string): Promise<void> {
    this.running.set(new Set(this.running()).add(domain));
    try {
      await this.wailsIpc.assessDomain(domain);
    } finally {
      const next = new Set(this.running());
      next.delete(domain);
      this.running.set(next);
    }
  }

  /** Coverage as a whole percentage, for the header figure. */
  coveragePercent(a: DomainAssessment): number {
    return Math.round((a.coverageFraction ?? 0) * 100);
  }

  /**
   * True when some part of the assessment did not conclude. The UI says so
   * explicitly rather than letting a partial result read as a complete one.
   */
  hasGaps(a: DomainAssessment): boolean {
    return a.coverage.notChecked > 0 || a.coverage.checkFailed > 0;
  }

  /** Human wording for a posture. Deliberately not a tick or a cross. */
  postureLabel(p: ControlPosture): string {
    switch (p) {
      case 'compliant': return 'Assessed clean';
      case 'deficient': return 'Advisories raised';
      default: return 'Not established';
    }
  }

  postureClasses(p: ControlPosture, dark: boolean): string {
    switch (p) {
      case 'compliant':
        return dark
          ? 'bg-emerald-950 border-emerald-800 text-emerald-400'
          : 'bg-emerald-50 border-emerald-300 text-emerald-800';
      case 'deficient':
        return dark
          ? 'bg-rose-950 border-rose-800 text-rose-400'
          : 'bg-rose-50 border-rose-300 text-rose-800';
      default:
        // Unknown is amber, never grey-as-good. It is a gap in knowledge, and
        // it should look like something needing attention.
        return dark
          ? 'bg-amber-950 border-amber-800 text-amber-400'
          : 'bg-amber-50 border-amber-300 text-amber-800';
    }
  }

  severityClasses(severity: string, dark: boolean): string {
    switch (severity) {
      case 'critical':
      case 'high':
        return dark ? 'text-rose-400' : 'text-rose-700';
      case 'medium':
        return dark ? 'text-amber-400' : 'text-amber-700';
      case 'low':
        return dark ? 'text-sky-400' : 'text-sky-700';
      default:
        return dark ? 'text-slate-400' : 'text-slate-600';
    }
  }

  /**
   * The rest of the assessed surface: everything the posture record does not
   * describe.
   *
   * It is still rendered, because hiding an assessed control would
   * misrepresent the coverage the assessment figures are computed over. The
   * seven posture controls are excluded here precisely so that no control is
   * described twice.
   */
  otherControls(a: DomainAssessment): ControlView[] {
    const claimed: string[] = Object.values(EmailPostureComponent.ASSESSMENT_CONTROL);
    return a.controls.filter(c => !claimed.includes(c.control));
  }

  /**
   * Spells out what a scenario chip's numbers mean, since two counts on one
   * chip are ambiguous without it. Coverage is included because a scenario
   * with no aggravating groups over two checks is not the same claim as the
   * same result over nine.
   */
  scenarioHint(s: ScenarioView): string {
    if (!s.supported) {
      return 'No assessment bears on this scenario yet';
    }
    const minor = s.aggravating - s.significant;
    const parts = [
      `${s.significant} weakness group(s) at medium severity or above`
    ];
    if (minor > 0) {
      parts.push(`${minor} raising only low or informational advisories`);
    }
    parts.push(`${s.mitigating} assessed clean`);
    parts.push(`over ${s.coverage.assessedOnly}/${s.coverage.total} checks concluded`);
    return parts.join(', ');
  }

  outcomeNote(a: DomainAssessment): string {
    switch (a.outcome) {
      case 'refused':
        return a.error || 'This domain is outside the authorised scope; nothing was queried.';
      case 'failed':
        return a.error || 'The assessment could not be performed.';
      case 'cancelled':
        return 'The assessment was withdrawn before it finished.';
      case 'partial':
        return 'Some checks did not reach a conclusion; coverage below is reduced accordingly.';
      default:
        return '';
    }
  }

  /** Formats a control identifier for display: "mta-sts" becomes "MTA-STS". */
  controlLabel(control: string): string {
    const overrides: Record<string, string> = {
      'spf': 'SPF', 'dkim': 'DKIM', 'dmarc': 'DMARC', 'mta-sts': 'MTA-STS',
      'tls-rpt': 'TLS-RPT', 'bimi': 'BIMI', 'mx': 'MX', 'caa': 'CAA',
      'dnssec': 'DNSSEC', 'zone-transfer': 'Zone transfer',
      'certificate-transparency': 'Certificate transparency'
    };
    return overrides[control] ?? control.replace(/-/g, ' ');
  }

  totalUnmapped = computed(() =>
    this.assessments().reduce((n, a) => n + (a.unmapped?.length ?? 0), 0)
  );
}
