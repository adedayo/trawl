import { DomainAssessment, FindingUI } from '../models/types';
import {
  DEFAULT_COVERAGE_FLOOR,
  HeadlinePosture,
  coverageLabel,
  estateCoverage,
  headlinePosture,
  rankFindings,
  unresolvedAssessments,
  withheldChecks
} from './estate';

/**
 * These assertions are deliberately about figures rather than markup.
 *
 * The properties that matter here — that a number is withheld below the
 * coverage floor, that two readers see the same order — are properties of the
 * derivation, and testing them through rendered HTML would tie them to layout
 * decisions that are expected to change.
 */

function assessment(over: Partial<DomainAssessment> = {}): DomainAssessment {
  return {
    domain: 'example.com',
    outcome: 'completed',
    coverage: { total: 10, ok: 8, notFound: 0, notChecked: 0, checkFailed: 0 },
    controls: [],
    ...over
  } as unknown as DomainAssessment;
}

function finding(over: Partial<FindingUI> = {}): FindingUI {
  return {
    id: 'f-1',
    severity: 'medium',
    kev: false,
    epssScore: 0,
    status: 'open',
    ...over
  } as unknown as FindingUI;
}

describe('estateCoverage', () => {
  it('counts ok and notFound as concluded, and nothing else', () => {
    const c = estateCoverage([
      assessment({
        coverage: { total: 10, ok: 4, notFound: 2, notChecked: 3, checkFailed: 1 }
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(c.applicable).toBe(10);
    expect(c.concluded).toBe(6);
    expect(c.checkFailed).toBe(1);
    expect(c.notChecked).toBe(3);
    expect(c.fraction).toBeCloseTo(0.6);
  });

  it('does not divide by zero when nothing is applicable', () => {
    expect(estateCoverage([]).fraction).toBe(0);
  });
});

describe('headlinePosture', () => {
  it('withholds the figure below the coverage floor', () => {
    const posture = headlinePosture([
      assessment({
        coverage: { total: 100, ok: 10, notFound: 0, notChecked: 90, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(posture.available).toBe(false);
    const withheldFigure = posture as Extract<HeadlinePosture, { available: false }>;
    expect(withheldFigure.reason).toBe('below-floor');
    // 60 checks must conclude to reach the floor; 10 have.
    expect(withheldFigure.shortfall).toBe(50);
    expect((withheldFigure as { score?: number }).score).toBeUndefined();
  });

  it('distinguishes nothing-assessed from below-floor', () => {
    const posture = headlinePosture([
      assessment({
        coverage: { total: 20, ok: 0, notFound: 0, notChecked: 20, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(posture.available).toBe(false);
    const withheldFigure = posture as Extract<HeadlinePosture, { available: false }>;
    expect(withheldFigure.reason).toBe('nothing-assessed');
  });

  it('scores over assessed controls only, once the floor is met', () => {
    const posture = headlinePosture([
      assessment({
        coverage: { total: 10, ok: 6, notFound: 2, notChecked: 2, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(posture.available).toBe(true);
    const figure = posture as Extract<HeadlinePosture, { available: true }>;
    // 6 sound of 8 concluded — the 2 unchecked lower coverage, not the score.
    expect(figure.score).toBe(75);
    expect(figure.band).toBe('adequate');
  });

  it('treats check_failed as an assessment gap, not a clean result', () => {
    const posture = headlinePosture([
      assessment({
        coverage: { total: 10, ok: 5, notFound: 0, notChecked: 0, checkFailed: 5 }
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(posture.available).toBe(false);
    const withheldFigure = posture as Extract<HeadlinePosture, { available: false }>;
    // Had the 5 failed checks counted as concluded the estate would read as
    // fully assessed and scored 100. Attempted-but-inconclusive is a gap.
    expect(withheldFigure.reason).toBe('below-floor');
    expect(withheldFigure.coverage.concluded).toBe(5);
  });

  it('honours an explicitly supplied floor', () => {
    const rows = [
      assessment({
        coverage: { total: 10, ok: 5, notFound: 0, notChecked: 5, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ];

    expect(headlinePosture(rows, DEFAULT_COVERAGE_FLOOR).available).toBe(false);
    expect(headlinePosture(rows, 0.5).available).toBe(true);
  });
});

describe('rankFindings', () => {
  it('lets known exploitation dominate EPSS and severity', () => {
    const ranked = rankFindings([
      finding({ id: 'a', severity: 'critical', epssScore: 0.9 }),
      finding({ id: 'b', severity: 'low', epssScore: 0.01, kev: true })
    ]);

    expect(ranked[0].finding.id).toBe('b');
    expect(ranked[0].position).toBe(1);
  });

  it('breaks EPSS ties by severity', () => {
    const ranked = rankFindings([
      finding({ id: 'a', severity: 'low', epssScore: 0.4 }),
      finding({ id: 'b', severity: 'critical', epssScore: 0.4 })
    ]);

    expect(ranked.map(r => r.finding.id)).toEqual(['b', 'a']);
  });

  it('produces a total order, so two readers see the same list', () => {
    const rows = [
      finding({ id: 'z', severity: 'high', epssScore: 0.5 }),
      finding({ id: 'a', severity: 'high', epssScore: 0.5 }),
      finding({ id: 'm', severity: 'high', epssScore: 0.5 })
    ];

    const first = rankFindings(rows).map(r => r.finding.id);
    const second = rankFindings(rows.slice().reverse()).map(r => r.finding.id);

    expect(first).toEqual(['a', 'm', 'z']);
    expect(second).toEqual(first);
  });

  it('omits resolved findings', () => {
    const ranked = rankFindings([
      finding({ id: 'a' }),
      finding({ id: 'b', status: 'resolved' } as unknown as Partial<FindingUI>)
    ]);

    expect(ranked.map(r => r.finding.id)).toEqual(['a']);
  });
});

describe('withheldChecks', () => {
  it('reports not_checked entries that carry a reason', () => {
    const withheld = withheldChecks([
      assessment({
        domain: 'b.example',
        controls: [
          {
            checks: [
              { checkId: 'dnssec', state: 'not_checked', reason: 'egress policy' },
              { checkId: 'spf', state: 'not_checked' },
              { checkId: 'dmarc', state: 'ok' }
            ]
          }
        ]
      } as unknown as Partial<DomainAssessment>)
    ]);

    expect(withheld.length).toBe(1);
    expect(withheld[0].checkId).toBe('dnssec');
    expect(withheld[0].reason).toBe('egress policy');
  });
});

describe('unresolvedAssessments', () => {
  it('surfaces every outcome that is not completed', () => {
    const rows = unresolvedAssessments([
      assessment({ domain: 'ok.example' }),
      assessment({ domain: 'refused.example', outcome: 'refused', error: 'not authorised' } as unknown as Partial<DomainAssessment>)
    ]);

    expect(rows.length).toBe(1);
    expect(rows[0].domain).toBe('refused.example');
    expect(rows[0].error).toBe('not authorised');
  });
});

describe('coverageLabel', () => {
  it('always carries its denominator', () => {
    expect(
      coverageLabel({ concluded: 6, applicable: 10, fraction: 0.6, checkFailed: 0, notChecked: 4 })
    ).toBe('60% assessed (6/10 checks)');
  });

  it('says so plainly when nothing was assessed', () => {
    expect(
      coverageLabel({ concluded: 0, applicable: 0, fraction: 0, checkFailed: 0, notChecked: 0 })
    ).toBe('nothing assessed');
  });
});
