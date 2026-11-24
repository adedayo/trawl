import { ChangeSet, FindingChange, HistoryPoint, ScanRunView } from '../models/types';
import { formatTime, plural } from './findings';

/**
 * Derivation for the change and trend views (Change 025, Phase 5).
 *
 * Pure functions again, and for a sharper reason than testability. A trend
 * line is the place where a tool most easily says something it does not know:
 * joining two points with a straight line asserts a value for every day
 * between them, and a run that checked less than the one before it draws as an
 * improvement. This module is where those two mistakes are made impossible.
 */

// ---------------------------------------------------------------------------
// Coverage
// ---------------------------------------------------------------------------

/** `63% of attempted checks concluded (19 of 30)`. */
export function coverageText(run: Pick<ScanRunView, 'coverage' | 'coverageFraction'> | undefined | null): string {
  if (!run) return 'coverage not recorded';
  const total = run.coverage?.total ?? 0;
  if (total === 0) return 'no checks were attempted';
  const concluded = run.coverage?.assessedOnly ?? 0;
  return `${Math.round(run.coverageFraction * 100)}% of attempted checks concluded (${concluded} of ${total})`;
}

/**
 * Whether the later run concluded materially fewer checks than the earlier.
 *
 * A fall below `tolerance` (default two percentage points) is noise from
 * rounding and differing run kinds; anything larger is stated, because a count
 * that fell while coverage fell may mean "less was looked at" rather than
 * "less is wrong".
 */
export function coverageDropped(
  from: Pick<ScanRunView, 'coverageFraction'> | undefined | null,
  to: Pick<ScanRunView, 'coverageFraction'> | undefined | null,
  tolerance = 0.02
): boolean {
  if (!from || !to) return false;
  return from.coverageFraction - to.coverageFraction > tolerance;
}

/** The sentence stated beside a count when coverage fell between two runs. */
export function coverageDropText(change: Pick<ChangeSet, 'from' | 'to'>): string {
  const from = change.from?.run;
  const to = change.to?.run;
  if (!from || !to || !coverageDropped(from, to)) return '';
  return `Coverage fell from ${Math.round(from.coverageFraction * 100)}% to ${Math.round(to.coverageFraction * 100)}%, so a lower count may reflect less being checked.`;
}

// ---------------------------------------------------------------------------
// Change set
// ---------------------------------------------------------------------------

/** Replaces the `null` lists the engine serialises for empty slices. */
export function normalizeChangeSet(change: ChangeSet): ChangeSet {
  return {
    ...change,
    new: change.new ?? [],
    resolved: change.resolved ?? [],
    worsened: change.worsened ?? [],
    improved: change.improved ?? []
  };
}

export type ChangeBucket = 'new' | 'resolved' | 'worsened' | 'improved';

export const CHANGE_BUCKETS: readonly { key: ChangeBucket; label: string }[] = [
  { key: 'new', label: 'New' },
  { key: 'resolved', label: 'Resolved' },
  { key: 'worsened', label: 'Worsened' },
  { key: 'improved', label: 'Improved' }
];

export function bucketItems(change: ChangeSet | null | undefined, bucket: ChangeBucket): FindingChange[] {
  return change?.[bucket] ?? [];
}

/**
 * Whether a comparison has an earlier run to compare against. Without one the
 * lists describe everything the engine first saw, not a change.
 */
export function hasBaseline(change: ChangeSet | null | undefined): boolean {
  return !!change?.from;
}

/** `compared 2026-11-01 10:00 UTC with 2026-11-08 10:00 UTC` */
export function comparedLine(change: ChangeSet | null | undefined): string {
  if (!change?.to) return '';
  const to = formatTime(change.to.at);
  return change.from ? `${formatTime(change.from.at)} compared with ${to}` : `Latest recorded scan: ${to}`;
}

// ---------------------------------------------------------------------------
// Trend
// ---------------------------------------------------------------------------

export const TREND_SERIES = [
  { key: 'critical', label: 'Critical' },
  { key: 'high', label: 'High' },
  { key: 'medium', label: 'Medium' },
  { key: 'low', label: 'Low' },
  { key: 'info', label: 'Info' }
] as const;

export type TrendSeriesKey = (typeof TREND_SERIES)[number]['key'];

export interface TrendPoint {
  index: number;
  /** Epoch milliseconds of the run. The x coordinate: actual time, not rank. */
  t: number;
  at: string;
  counts: Record<TrendSeriesKey, number>;
  open: number;
  newInRun: number;
  resolvedInRun: number;
  coverageFraction: number;
  coverage: string;
  /** True when this run concluded materially fewer checks than the one before. */
  thin: boolean;
  kind: string;
}

export interface TrendGap {
  /** The points either side of the silence. */
  fromIndex: number;
  toIndex: number;
  fromT: number;
  toT: number;
  days: number;
}

export interface Trend {
  points: TrendPoint[];
  /** Silences long enough to be drawn as gaps, in order. */
  gaps: TrendGap[];
  /** Index pairs that are close enough to be joined. Never spans a gap. */
  segments: [number, number][];
  medianIntervalMs: number;
  tMin: number;
  tMax: number;
  maxCount: number;
}

export interface TrendOptions {
  /** A silence longer than this many typical intervals is a gap. Default 3. */
  gapFactor?: number;
  /**
   * A floor under the gap threshold. Runs arrive in bursts (one per asset), so
   * the median interval can be minutes, and without a floor every ordinary
   * overnight silence would be flagged. Default one hour.
   */
  minGapMs?: number;
}

export function median(values: readonly number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 1 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

/**
 * Median of `sorted` with one occurrence of `excluded` removed, in O(1).
 *
 * Pre-sorting once and removing in constant time turns `buildTrend`'s gap
 * classification from O(N^2 log N) into O(N log N) while keeping the exact
 * same leave-one-out median semantics.
 */
function medianWithout(sorted: readonly number[], excluded: number): number {
  const m = sorted.length;
  if (m <= 1) return 0;
  const k = Math.floor(m / 2);
  if ((m - 1) % 2 === 1) {
    // m = 2k, remaining length is odd (2k - 1), median is at index k - 1 of remaining.
    return excluded <= sorted[k - 1] ? sorted[k] : sorted[k - 1];
  }
  // m = 2k + 1, remaining length is even (2k), median averages indices k - 1 and k.
  if (excluded < sorted[k]) return (sorted[k] + sorted[k + 1]) / 2;
  if (excluded === sorted[k]) return (sorted[k - 1] + sorted[k + 1]) / 2;
  return (sorted[k - 1] + sorted[k]) / 2;
}

/**
 * Turns run summaries into a trend, sorted by time, with silences marked.
 *
 * Points are placed at the run's real time. Two consecutive points are joined
 * only when the silence between them is within `gapFactor` times the median of
 * the other intervals (and above the floor); a longer silence becomes a gap and
 * nothing is drawn across it. Points with unusable times are dropped rather than placed
 * somewhere plausible.
 */
export function buildTrend(history: readonly HistoryPoint[], options: TrendOptions = {}): Trend {
  const gapFactor = options.gapFactor ?? 3;
  const minGapMs = options.minGapMs ?? 3_600_000;

  const sorted = history
    .map(h => ({ h, t: Date.parse(h.at) }))
    .filter(x => !Number.isNaN(x.t))
    .sort((a, b) => a.t - b.t);

  const points: TrendPoint[] = sorted.map(({ h, t }, index) => ({
    index,
    t,
    at: h.at,
    counts: {
      critical: h.critical,
      high: h.high,
      medium: h.medium,
      low: h.low,
      info: h.info
    },
    open: h.open,
    newInRun: h.new,
    resolvedInRun: h.resolvedInRun,
    coverageFraction: h.run?.coverageFraction ?? 0,
    coverage: coverageText(h.run),
    thin: index > 0 && coverageDropped(sorted[index - 1].h.run, h.run),
    kind: h.run?.kind ?? ''
  }));

  const intervals: number[] = [];
  for (let i = 1; i < points.length; i++) {
    intervals.push(points[i].t - points[i - 1].t);
  }
  const sortedIntervals = [...intervals].sort((a, b) => a - b);
  const medianIntervalMs =
    sortedIntervals.length === 0
      ? 0
      : sortedIntervals.length % 2 === 1
        ? sortedIntervals[Math.floor(sortedIntervals.length / 2)]
        : (sortedIntervals[sortedIntervals.length / 2 - 1] + sortedIntervals[sortedIntervals.length / 2]) / 2;

  // Each interval is judged against the typical interval of the *others*. A
  // single long silence otherwise drags the median up and excuses itself: with
  // a handful of runs, one ten-day pause among daily scans can be the median.
  // With no others to compare against, only the floor applies, so two runs far
  // apart are shown as two markers rather than joined.
  const gaps: TrendGap[] = [];
  const segments: [number, number][] = [];
  for (let i = 1; i < points.length; i++) {
    const interval = intervals[i - 1];
    const threshold = Math.max(medianWithout(sortedIntervals, interval) * gapFactor, minGapMs);
    if (interval > threshold) {
      gaps.push({
        fromIndex: i - 1,
        toIndex: i,
        fromT: points[i - 1].t,
        toT: points[i].t,
        days: Math.round(interval / 86_400_000)
      });
    } else {
      segments.push([i - 1, i]);
    }
  }

  const maxCount = points.reduce(
    (m, p) => Math.max(m, ...TREND_SERIES.map(s => p.counts[s.key])),
    0
  );

  return {
    points,
    gaps,
    segments,
    medianIntervalMs,
    tMin: points.length ? points[0].t : 0,
    tMax: points.length ? points[points.length - 1].t : 0,
    maxCount
  };
}

/**
 * SVG path data for one series, one sub-path per connected run of points.
 *
 * Between two joined runs the value is held until the next run and then
 * steps: the engine recorded a value at each run, and a sloped line would
 * assert intermediate values nobody measured. A gap or a lone point starts a
 * fresh `M`, so there is no stroke across a silence. A lone point has no path;
 * its marker carries it.
 */
export function seriesPaths(
  trend: Trend,
  key: TrendSeriesKey,
  x: (t: number) => number,
  y: (count: number) => number
): string[] {
  const joined = new Set(trend.segments.map(([a]) => a));
  const paths: string[] = [];
  let current = '';
  for (let i = 0; i < trend.points.length; i++) {
    const p = trend.points[i];
    const px = round(x(p.t));
    const py = round(y(p.counts[key]));
    if (current) {
      // Hold the previous value up to this run, then step.
      current += ` H${px} V${py}`;
    } else {
      current = `M${px} ${py}`;
    }
    const continues = joined.has(i);
    if (!continues) {
      if (current.includes('H')) {
        paths.push(current);
      }
      current = '';
    }
  }
  return paths;
}

function round(n: number): number {
  return Math.round(n * 10) / 10;
}

/** The accessible tooltip for one run. */
export function pointSummary(p: TrendPoint): string {
  const by = TREND_SERIES.map(s => `${s.label.toLowerCase()} ${p.counts[s.key]}`).join(', ');
  const thin = p.thin ? ' Coverage was lower than the previous run, so a drop in findings here may reflect less being checked.' : '';
  return `${formatTime(p.at)}. ${plural(p.open, 'finding')} open (${by}). Coverage: ${p.coverage}.${thin}`;
}

export interface TrendTableRow {
  kind: 'run' | 'gap';
  point?: TrendPoint;
  gap?: TrendGap;
}

/** Rows for the text alternative, with each gap as its own row between runs. */
export function trendTableRows(trend: Trend): TrendTableRow[] {
  const gapAfter = new Map(trend.gaps.map(g => [g.fromIndex, g]));
  const rows: TrendTableRow[] = [];
  for (const point of trend.points) {
    rows.push({ kind: 'run', point });
    const gap = gapAfter.get(point.index);
    if (gap) rows.push({ kind: 'gap', gap });
  }
  return rows;
}

/** The label for a gap: how long nothing was recorded. */
export function gapLabel(gap: TrendGap): string {
  return `No scan recorded for ${plural(gap.days, 'day')}`;
}
