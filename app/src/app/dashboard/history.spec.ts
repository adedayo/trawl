import {
  buildTrend,
  bucketItems,
  comparedLine,
  coverageDropped,
  coverageDropText,
  coverageText,
  gapLabel,
  hasBaseline,
  median,
  normalizeChangeSet,
  pointSummary,
  seriesPaths,
  trendTableRows
} from './history';
import { change, changeSet, point, run } from './findings.fixtures';
import { ChangeSet } from '../models/types';

const HOUR = 3_600_000;
const DAY = 24 * HOUR;
const T0 = Date.parse('2026-11-01T00:00:00Z');
const at = (ms: number) => new Date(T0 + ms).toISOString();

describe('coverage wording', () => {
  it('states coverage as a fraction of attempted checks, with the counts', () => {
    expect(coverageText(run({ coverageFraction: 0.63, coverage: { total: 30, ok: 19, notFound: 0, notChecked: 11, checkFailed: 0, assessedOnly: 19 } })))
      .toBe('63% of attempted checks concluded (19 of 30)');
  });

  it('says nothing was attempted rather than showing 0%', () => {
    expect(coverageText(run({ coverageFraction: 0, coverage: { total: 0, ok: 0, notFound: 0, notChecked: 0, checkFailed: 0, assessedOnly: 0 } })))
      .toBe('no checks were attempted');
    expect(coverageText(null)).toBe('coverage not recorded');
  });

  it('flags a fall in coverage and ignores rounding noise', () => {
    expect(coverageDropped({ coverageFraction: 0.9 }, { coverageFraction: 0.6 })).toBe(true);
    expect(coverageDropped({ coverageFraction: 0.9 }, { coverageFraction: 0.89 })).toBe(false);
    expect(coverageDropped({ coverageFraction: 0.6 }, { coverageFraction: 0.9 })).toBe(false);
    expect(coverageDropped(null, { coverageFraction: 0.1 })).toBe(false);
  });
});

describe('the change panel wording', () => {
  const from = point({ runId: 'r1', at: at(0), coverageFraction: 0.9 });
  const to = point({ runId: 'r2', at: at(DAY), coverageFraction: 0.6 });

  it('states the fall in coverage beside the counts when the later run checked less', () => {
    const text = coverageDropText({ from, to });
    expect(text).toContain('Coverage fell from 90% to 60%');
    expect(text).toContain('less being checked');
  });

  it('states nothing when coverage held or rose', () => {
    expect(coverageDropText({ from: to, to: from })).toBe('');
    expect(coverageDropText({ from: null, to })).toBe('');
  });

  it('knows whether there is an earlier run to compare against', () => {
    expect(hasBaseline(changeSet({ from, to }))).toBe(true);
    expect(hasBaseline(changeSet({ to }))).toBe(false);
    expect(hasBaseline(null)).toBe(false);
  });

  it('names the runs compared', () => {
    expect(comparedLine(changeSet({ from, to }))).toBe('2026-11-01 00:00 UTC compared with 2026-11-02 00:00 UTC');
    expect(comparedLine(changeSet({ to }))).toContain('Latest recorded scan');
    expect(comparedLine(changeSet())).toBe('');
  });

  it('reads each bucket and survives the null lists Go serialises', () => {
    const c = changeSet({ new: [change()] });
    expect(bucketItems(c, 'new').length).toBe(1);
    expect(bucketItems(c, 'resolved')).toEqual([]);
    const raw = { new: null, resolved: null, worsened: null, improved: null } as unknown as ChangeSet;
    const safe = normalizeChangeSet(raw);
    expect(safe.new).toEqual([]);
    expect(safe.improved).toEqual([]);
  });
});

describe('median', () => {
  it('handles empty, odd and even lists', () => {
    expect(median([])).toBe(0);
    expect(median([5])).toBe(5);
    expect(median([3, 1, 2])).toBe(2);
    expect(median([4, 1, 3, 2])).toBe(2.5);
  });
});

describe('buildTrend', () => {
  /** Daily scans for five days, then ten days of silence, then two more. */
  const history = [
    ...[0, 1, 2, 3, 4].map(d => point({ runId: `r${d}`, at: at(d * DAY), high: d })),
    ...[14, 15].map(d => point({ runId: `r${d}`, at: at(d * DAY), high: d }))
  ];

  it('places points at their real times, sorted, whatever order they arrive in', () => {
    const trend = buildTrend([...history].reverse());
    expect(trend.points.map(p => p.t)).toEqual(history.map(h => Date.parse(h.at)));
    expect(trend.tMin).toBe(T0);
    expect(trend.tMax).toBe(T0 + 15 * DAY);
  });

  it('draws a ten-day silence as a gap and joins nothing across it', () => {
    const trend = buildTrend(history);
    expect(trend.gaps).toHaveLength(1);
    expect(trend.gaps[0]).toMatchObject({ fromIndex: 4, toIndex: 5, days: 10 });
    expect(trend.segments).toEqual([[0, 1], [1, 2], [2, 3], [3, 4], [5, 6]]);
    expect(trend.segments.some(([a, b]) => a === 4 && b === 5)).toBe(false);
  });

  it('flags a gap only beyond three times the median interval', () => {
    const steady = [0, 1, 2, 3].map(d => point({ at: at(d * DAY) }));
    expect(buildTrend(steady).gaps).toEqual([]);
    const threeDays = [...steady, point({ at: at(6 * DAY) })];
    // A three-day silence is exactly 3x the one-day median, so it is not a gap.
    expect(buildTrend(threeDays).gaps).toEqual([]);
    const fourDays = [...steady, point({ at: at(7 * DAY) })];
    expect(buildTrend(fourDays).gaps).toHaveLength(1);
  });

  it('does not flag ordinary silences between bursts of runs', () => {
    // Runs minutes apart make the median tiny; the floor keeps a half-hour pause an ordinary one.
    const burst = [0, 1, 2, 3, 4].map(m => point({ at: at(m * 60_000) }));
    const trend = buildTrend([...burst, point({ at: at(34 * 60_000) })]);
    expect(trend.gaps).toEqual([]);
  });

  it('marks a run that concluded fewer checks than the one before as thin', () => {
    const trend = buildTrend([
      point({ at: at(0), coverageFraction: 0.9 }),
      point({ at: at(DAY), coverageFraction: 0.4, high: 0, medium: 0, low: 0, open: 0 }),
      point({ at: at(2 * DAY), coverageFraction: 0.9 })
    ]);
    expect(trend.points.map(p => p.thin)).toEqual([false, true, false]);
  });

  it('carries each run coverage onto its point and into the tooltip', () => {
    const trend = buildTrend([point({ at: at(0), coverageFraction: 0.5 })]);
    expect(trend.points[0].coverageFraction).toBe(0.5);
    const summary = pointSummary(trend.points[0]);
    expect(summary).toContain('Coverage:');
    expect(summary).toContain('50%');
  });

  it('says in the tooltip that a thin run may reflect less being checked', () => {
    const trend = buildTrend([point({ at: at(0), coverageFraction: 0.9 }), point({ at: at(DAY), coverageFraction: 0.3 })]);
    expect(pointSummary(trend.points[1])).toContain('less being checked');
    expect(pointSummary(trend.points[0])).not.toContain('less being checked');
  });

  it('drops points whose time cannot be read rather than placing them', () => {
    const trend = buildTrend([point({ at: 'nonsense' }), point({ at: at(0) })]);
    expect(trend.points).toHaveLength(1);
  });

  it('handles no history and a single run', () => {
    expect(buildTrend([])).toMatchObject({ points: [], gaps: [], segments: [], maxCount: 0 });
    const one = buildTrend([point({ at: at(0) })]);
    expect(one.points).toHaveLength(1);
    expect(one.gaps).toEqual([]);
    expect(one.segments).toEqual([]);
  });

  it('reports the largest count so the axis can be scaled', () => {
    expect(buildTrend([point({ at: at(0), low: 9 }), point({ at: at(DAY), high: 4 })]).maxCount).toBe(9);
  });

  it('handles 2,000 scan summaries swiftly in O(N log N) time', () => {
    const many = Array.from({ length: 2000 }, (_, i) =>
      point({ runId: `r-${i}`, at: at(i * HOUR), high: i % 5, open: i % 5 })
    );
    const start = performance.now();
    const trend = buildTrend(many);
    const elapsedMs = performance.now() - start;

    expect(trend.points).toHaveLength(2000);
    expect(trend.gaps).toHaveLength(0);
    expect(trend.segments).toHaveLength(1999);
    expect(elapsedMs).toBeLessThan(250);
  });
});

describe('seriesPaths', () => {
  const x = (t: number) => (t - T0) / HOUR;
  const y = (c: number) => 100 - c;
  const history = [
    point({ at: at(0), high: 1 }),
    point({ at: at(DAY), high: 3 }),
    // a long silence
    point({ at: at(20 * DAY), high: 5 }),
    point({ at: at(21 * DAY), high: 2 }),
    // then a lone run
    point({ at: at(60 * DAY), high: 7 })
  ];

  it('starts a separate stroke after each gap, so nothing spans a silence', () => {
    const paths = seriesPaths(buildTrend(history), 'high', x, y);
    expect(paths).toHaveLength(2);
    expect(paths.every(d => (d.match(/M/g) ?? []).length === 1)).toBe(true);
  });

  it('holds the value to the next run and then steps, with no slope', () => {
    const [first] = seriesPaths(buildTrend(history), 'high', x, y);
    // From (0, 99) hold to x=24, then step to y=97.
    expect(first).toBe('M0 99 H24 V97');
    expect(first).not.toMatch(/L/);
  });

  it('gives a lone point no stroke, leaving its marker to carry it', () => {
    const paths = seriesPaths(buildTrend(history), 'high', x, y);
    expect(paths.some(d => d.includes('M1440'))).toBe(false);
  });

  it('draws nothing for one run', () => {
    expect(seriesPaths(buildTrend([point({ at: at(0) })]), 'high', x, y)).toEqual([]);
  });
});

describe('trendTableRows', () => {
  it('puts a gap row between the runs either side of the silence', () => {
    const trend = buildTrend([point({ at: at(0) }), point({ at: at(DAY) }), point({ at: at(30 * DAY) })]);
    const rows = trendTableRows(trend);
    expect(rows.map(r => r.kind)).toEqual(['run', 'run', 'gap', 'run']);
    expect(gapLabel(rows[2].gap!)).toBe('No scan recorded for 29 days');
  });
});
