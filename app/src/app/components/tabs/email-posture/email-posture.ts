import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { ControlPosture, ControlView, CoverageState, DomainAssessment, ScenarioView } from '../../../models/types';

/**
 * Renders the measured-state assessment for each domain.
 *
 * The governing rule of this component is that it never collapses the
 * four-state coverage model into a tick or a cross. "Not published", "not
 * checked" and "could not tell" are distinct, and a control that was never
 * assessed is shown as unknown rather than as passing — because an operator
 * reading a green card concludes they are protected, and that conclusion must
 * be earned by an assessment that actually happened.
 */
@Component({
  selector: 'app-email-posture',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './email-posture.html',
  styleUrls: ['./email-posture.css']
})
export class EmailPostureComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  assessments = this.wailsIpc.assessments;
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
  private static readonly HEADLINE_CONTROLS = ['spf', 'dkim', 'dmarc', 'mta-sts'];

  /**
   * Controls that bear on mail authentication and transport, in the order an
   * operator reasons about them. Other controls are still rendered — under
   * "Other surface" — because hiding an assessed control would misrepresent
   * the coverage the figures are computed over.
   */
  private static readonly EMAIL_CONTROLS = [
    'spf', 'dkim', 'dmarc', 'mta-sts', 'tls-rpt', 'bimi', 'mx'
  ];

  emailControls(a: DomainAssessment): ControlView[] {
    return a.controls.filter(c => EmailPostureComponent.EMAIL_CONTROLS.includes(c.control));
  }

  otherControls(a: DomainAssessment): ControlView[] {
    return a.controls.filter(c => !EmailPostureComponent.EMAIL_CONTROLS.includes(c.control));
  }

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
    this.openDomains.set(new Set(this.assessments().map(a => a.domain)));
  }

  collapseAll(): void {
    this.openDomains.set(new Set());
  }

  /**
   * The headline controls for a collapsed row, in a fixed order.
   *
   * A control that was never assessed is emitted as unknown rather than
   * omitted. Dropping it would make a domain with four assessed controls and a
   * domain with one look alike at a glance, which is the exact confusion the
   * four-state model exists to prevent.
   */
  summaryControls(a: DomainAssessment): ControlView[] {
    return EmailPostureComponent.HEADLINE_CONTROLS.map(
      name => a.controls.find(c => c.control === name) ?? {
        control: name,
        posture: 'unknown' as ControlPosture,
        coverage: { total: 0, ok: 0, assessedOnly: 0, notFound: 0, notChecked: 0, checkFailed: 0 },
        checks: [],
        signals: []
      }
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
   * clean result over two checks is not the same claim as a clean result over
   * twenty and must not read like one.
   */
  summaryLine(a: DomainAssessment): string {
    if (a.outcome === 'refused' || a.outcome === 'failed' || a.outcome === 'cancelled') {
      return this.outcomeNote(a);
    }
    const significant = this.significantCount(a);
    const total = this.advisoryCount(a);
    const scope = `${a.coverage.assessedOnly}/${a.coverage.total} checks concluded`;
    if (total === 0) {
      return a.coverage.total === 0
        ? 'Nothing has been assessed for this domain yet.'
        : `No advisory raised across ${scope}.`;
    }
    const tail = significant > 0
      ? `${significant} at medium or above`
      : 'all low or informational';
    return `${total} advisory(s) — ${tail}, over ${scope}.`;
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

  /**
   * A small solid dot per headline control on the collapsed row. Colour alone
   * never carries the meaning: each chip is labelled with the control name and
   * titled with the posture in words, so the row is legible without colour
   * vision and to a screen reader.
   */
  summaryDotClasses(p: ControlPosture): string {
    switch (p) {
      case 'compliant': return 'bg-emerald-500';
      case 'deficient': return 'bg-rose-500';
      default: return 'bg-amber-500';
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

  /** Plain-language wording for one of the four coverage states. */
  stateLabel(s: CoverageState): string {
    switch (s) {
      case 'ok': return 'Assessed';
      case 'not_found': return 'Not published';
      case 'not_checked': return 'Not checked';
      default: return 'Could not tell';
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
