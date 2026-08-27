import { CoverageSummary, DomainAssessment, FindingUI } from '../models/types';

/**
 * Estate-level derivation for the executive area.
 *
 * Everything here is a pure function over rows the detail views already read.
 * That is a deliberate constraint rather than a stylistic one: the executive
 * area must never become a second computation. The moment a summary figure is
 * derived its own way — by an analytics endpoint, or by arithmetic embedded in
 * a template — the board slide and the engineer's table can disagree by four,
 * nobody can say which is right, and the tool's credibility goes with it.
 *
 * Keeping the derivation pure also makes it testable without mounting a
 * component, which is what allows the coverage floor and the ranking to be
 * asserted directly rather than inferred from rendered markup.
 */

/** A regression as the backend records it: a tracked attribute that worsened. */
export interface RegressionUI {
  id: string;
  assetId: string;
  attributeType: string;
  previousValue: string;
  currentValue: string;
  consecutiveFails: number;
  confirmedAt: string;
}

/** A check the deployment's egress policy withheld, and why. */
export interface WithheldCheck {
  domain: string;
  checkId: string;
  reason: string;
}

/**
 * The headline posture figure, or the reason there isn't one.
 *
 * `available: false` is a first-class outcome, not an error. Below the
 * coverage floor the honest answer is that the estate has not been assessed
 * enough to summarise, and saying so is more actionable than a hedged number.
 */
export type HeadlinePosture =
  | {
      available: true;
      /** Percentage of assessed controls found sound, 0-100. */
      score: number;
      band: 'strong' | 'adequate' | 'weak';
      coverage: CoverageFigure;
    }
  | {
      available: false;
      reason: 'below-floor' | 'nothing-assessed';
      coverage: CoverageFigure;
      /** Checks that would have to conclude to reach the floor. */
      shortfall: number;
    };

export interface CoverageFigure {
  /** Checks that reached a conclusion. */
  concluded: number;
  /** Checks that were applicable. */
  applicable: number;
  /** concluded / applicable, 0-1. Zero when nothing was applicable. */
  fraction: number;
  /** Attempted but inconclusive — an assessment gap, not a clean result. */
  checkFailed: number;
  /** Never attempted, including everything withheld by policy. */
  notChecked: number;
}

/**
 * The default floor beneath which no headline figure is offered.
 *
 * Configuration rather than a constant in a component, because the right value
 * depends on the estate and because a figure-affecting threshold buried in
 * view code is unauditable. The value in force is displayed alongside the
 * refusal so a deployment cannot quietly lower it to make a number appear.
 */
export const DEFAULT_COVERAGE_FLOOR = 0.6;

const EMPTY_COVERAGE: CoverageFigure = {
  concluded: 0,
  applicable: 0,
  fraction: 0,
  checkFailed: 0,
  notChecked: 0
};

/** Sums per-domain coverage into one estate figure. */
export function estateCoverage(assessments: DomainAssessment[]): CoverageFigure {
  const totals = assessments.reduce(
    (acc, a) => {
      const c: CoverageSummary | undefined = a.coverage;
      if (!c) {
        return acc;
      }
      return {
        applicable: acc.applicable + (c.total ?? 0),
        concluded: acc.concluded + (c.ok ?? 0) + (c.notFound ?? 0),
        checkFailed: acc.checkFailed + (c.checkFailed ?? 0),
        notChecked: acc.notChecked + (c.notChecked ?? 0)
      };
    },
    { applicable: 0, concluded: 0, checkFailed: 0, notChecked: 0 }
  );

  return {
    ...totals,
    fraction: totals.applicable === 0 ? 0 : totals.concluded / totals.applicable
  };
}

/**
 * Derives the headline figure, refusing below the floor.
 *
 * The score is the proportion of *assessed* controls that were found sound.
 * Computing it over assessed controls only, and then gating the whole figure
 * on coverage, keeps the two questions separate: "how healthy is what we
 * looked at" and "how much did we look at". Folding unassessed checks into the
 * denominator would blend them into a single number that answers neither.
 */
export function headlinePosture(
  assessments: DomainAssessment[],
  floor: number = DEFAULT_COVERAGE_FLOOR
): HeadlinePosture {
  const coverage = estateCoverage(assessments);

  if (coverage.applicable === 0 || coverage.concluded === 0) {
    return { available: false, reason: 'nothing-assessed', coverage, shortfall: 0 };
  }

  if (coverage.fraction < floor) {
    const needed = Math.ceil(floor * coverage.applicable) - coverage.concluded;
    return {
      available: false,
      reason: 'below-floor',
      coverage,
      shortfall: Math.max(needed, 0)
    };
  }

  const ok = assessments.reduce((n, a) => n + (a.coverage?.ok ?? 0), 0);
  const score = Math.round((ok / coverage.concluded) * 100);

  return {
    available: true,
    score,
    band: score >= 85 ? 'strong' : score >= 60 ? 'adequate' : 'weak',
    coverage
  };
}

/**
 * Collects checks withheld by the deployment's egress policy.
 *
 * These are surfaced rather than filtered out. They are the operator's own
 * policy decisions made visible, and omitting them would reproduce in the
 * interface exactly the failure the fail-closed path exists to prevent: a
 * check that did not run and nobody was told.
 */
export function withheldChecks(assessments: DomainAssessment[]): WithheldCheck[] {
  const out: WithheldCheck[] = [];
  for (const a of assessments) {
    for (const control of a.controls ?? []) {
      for (const check of control.checks ?? []) {
        if (check.state === 'not_checked' && check.reason) {
          out.push({ domain: a.domain, checkId: check.checkId, reason: check.reason });
        }
      }
    }
  }
  return out.sort((x, y) => x.domain.localeCompare(y.domain) || x.checkId.localeCompare(y.checkId));
}

/** A finding with the deterministic inputs that placed it in the order. */
export interface RankedFinding {
  finding: FindingUI;
  /** Exploited in the wild, per CISA KEV. The strongest single input. */
  kev: boolean;
  /** Probability of exploitation in the next 30 days, per EPSS, 0-1. */
  epss: number;
  severity: FindingUI['severity'];
  /** Position in the order, 1-based, for display and for testing. */
  position: number;
}

const SEVERITY_RANK: Record<string, number> = {
  critical: 5,
  high: 4,
  medium: 3,
  low: 2,
  info: 1
};

/**
 * Orders open findings by what should be done first.
 *
 * The ordering is deterministic and depends on nothing outside the rows: two
 * operators looking at the same data see the same order. That property is
 * worth more than any particular weighting, because an order that varies
 * between readers cannot be argued with, only trusted or dismissed.
 *
 * Known exploitation dominates, because it is the one input that reports what
 * is actually happening rather than what could. Everything else is a tie-break
 * beneath it.
 */
export function rankFindings(findings: FindingUI[]): RankedFinding[] {
  return findings
    .filter(f => f.status !== 'resolved')
    .slice()
    .sort((a, b) => {
      if (a.kev !== b.kev) {
        return a.kev ? -1 : 1;
      }
      const epssDelta = (b.epssScore ?? 0) - (a.epssScore ?? 0);
      if (Math.abs(epssDelta) > 1e-9) {
        return epssDelta;
      }
      const sevDelta = (SEVERITY_RANK[b.severity] ?? 0) - (SEVERITY_RANK[a.severity] ?? 0);
      if (sevDelta !== 0) {
        return sevDelta;
      }
      // A total order, so the list cannot reshuffle between renders of
      // identical data.
      return (a.id ?? '').localeCompare(b.id ?? '');
    })
    .map((finding, i) => ({
      finding,
      kev: finding.kev,
      epss: finding.epssScore ?? 0,
      severity: finding.severity,
      position: i + 1
    }));
}

/** Domains whose last assessment did not complete, with the reason. */
export function unresolvedAssessments(
  assessments: DomainAssessment[]
): { domain: string; outcome: string; error?: string }[] {
  return assessments
    .filter(a => a.outcome && a.outcome !== 'completed')
    .map(a => ({ domain: a.domain, outcome: a.outcome, error: a.error }));
}

/** Formats a coverage fraction for display, always with its denominator. */
export function coverageLabel(c: CoverageFigure): string {
  if (c.applicable === 0) {
    return 'nothing assessed';
  }
  return `${Math.round(c.fraction * 100)}% assessed (${c.concluded}/${c.applicable} checks)`;
}
