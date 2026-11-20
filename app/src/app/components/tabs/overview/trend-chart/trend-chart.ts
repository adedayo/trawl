import { Component, computed, inject, signal } from '@angular/core';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { formatDay, formatTime, plural } from '../../../../dashboard/findings';
import {
  buildTrend,
  gapLabel,
  pointSummary,
  seriesPaths,
  TREND_SERIES,
  TrendPoint,
  TrendSeriesKey,
  trendTableRows
} from '../../../../dashboard/history';

/** The geometry of the drawing, in viewBox units. */
const VIEW_W = 720;
const PLOT_LEFT = 48;
const PLOT_RIGHT = 704;
const PLOT_TOP = 16;
const PLOT_BOTTOM = 176;
const COV_TOP = 214;
const COV_BOTTOM = 254;
const VIEW_H = 290;

type Shape = 'circle' | 'square' | 'diamond' | 'triangle' | 'cross';

/**
 * Each severity has a colour and a marker shape, so the chart does not depend
 * on telling hues apart. The full class names are written out because the
 * stylesheet is generated from the literals found in source.
 */
const STYLE: Record<TrendSeriesKey, { shape: Shape; stroke: string; fill: string; swatch: string }> = {
  critical: { shape: 'circle', stroke: 'stroke-rose-600', fill: 'fill-rose-600', swatch: 'bg-rose-600' },
  high: { shape: 'square', stroke: 'stroke-amber-500', fill: 'fill-amber-500', swatch: 'bg-amber-500' },
  medium: { shape: 'diamond', stroke: 'stroke-blue-600', fill: 'fill-blue-600', swatch: 'bg-blue-600' },
  low: { shape: 'triangle', stroke: 'stroke-slate-500', fill: 'fill-slate-500', swatch: 'bg-slate-500' },
  info: { shape: 'cross', stroke: 'stroke-slate-400', fill: 'fill-slate-400', swatch: 'bg-slate-400' }
};

const RANGES = [
  { value: '30', label: 'Last 30 days', days: 30 },
  { value: '90', label: 'Last 90 days', days: 90 },
  { value: '365', label: 'Last year', days: 365 },
  { value: 'all', label: 'Everything recorded', days: 0 }
] as const;

/**
 * Open findings by severity, one point per recorded scan (Change 025, Phase 5).
 *
 * Three rules keep this chart from saying more than the engine knows. Points
 * sit at the run's real time, not at an evenly spaced rank. Nothing is drawn
 * across a silence: a long gap between scans is shown as a gap, and short
 * intervals are joined by a step that holds the last recorded value rather
 * than a slope that invents the days between. And each run's coverage is drawn
 * beneath it, with a run that concluded fewer checks than the one before
 * marked, so a thin run cannot pass for an improvement.
 *
 * The same data is available as a table, for anyone the drawing does not
 * serve.
 */
@Component({
  selector: 'app-trend-chart',
  standalone: true,
  templateUrl: './trend-chart.html'
})
export class TrendChartComponent {
  private readonly wailsIpc = inject(WailsIpcService);
  readonly theme = this.wailsIpc.theme;
  readonly state = this.wailsIpc.historyState;

  readonly ranges = RANGES;
  readonly range = signal<string>('all');
  readonly series = TREND_SERIES;
  readonly style = STYLE;
  readonly viewBox = `0 0 ${VIEW_W} ${VIEW_H}`;
  readonly plotLeft = PLOT_LEFT;
  readonly plotRight = PLOT_RIGHT;
  readonly plotTop = PLOT_TOP;
  readonly plotBottom = PLOT_BOTTOM;
  readonly covTop = COV_TOP;
  readonly covBottom = COV_BOTTOM;

  readonly trend = computed(() => buildTrend(this.wailsIpc.history()));
  readonly rows = computed(() => trendTableRows(this.trend()));
  readonly hasThin = computed(() => this.trend().points.some(p => p.thin));

  /**
   * Points rendered as SVG markers and coverage bars.
   *
   * When an existing database holds thousands of back-to-back per-host runs,
   * drawing every marker turns the chart into an unreadable band and stalls
   * the DOM. Above 60 points we keep the endpoints, coverage drops, count
   * transitions, and evenly spaced checkpoints (capped at 60) while keeping
   * all points intact for smaller histories.
   */
  readonly plottedPoints = computed<TrendPoint[]>(() => {
    const pts = this.trend().points;
    const maxPlotted = 60;
    if (pts.length <= maxPlotted) return pts;

    const keep = new Set<number>([0, pts.length - 1]);
    for (let i = 1; i < pts.length - 1; i++) {
      const prev = pts[i - 1];
      const cur = pts[i];
      if (cur.thin || cur.open !== prev.open || cur.counts.critical !== prev.counts.critical || cur.counts.high !== prev.counts.high) {
        keep.add(i);
      }
    }
    const sorted = Array.from(keep).sort((a, b) => a - b);
    if (sorted.length <= maxPlotted) {
      // Fill remaining slots evenly so steady spans still have markers.
      const step = (pts.length - 1) / (maxPlotted - 1);
      for (let k = 0; k < maxPlotted && keep.size < maxPlotted; k++) {
        keep.add(Math.round(k * step));
      }
      return Array.from(keep)
        .sort((a, b) => a - b)
        .map(i => pts[i]);
    }
    // Too many change points: sample evenly across the sorted set, always keeping first and last.
    const out: TrendPoint[] = [pts[sorted[0]]];
    const step = (sorted.length - 1) / (maxPlotted - 1);
    for (let k = 1; k < maxPlotted - 1; k++) {
      out.push(pts[sorted[Math.round(k * step)]]);
    }
    out.push(pts[sorted[sorted.length - 1]]);
    return out;
  });

  readonly isDownsampled = computed(() => this.plottedPoints().length < this.trend().points.length);

  /** Most recent rows for the expandable table, capped so huge histories stay responsive. */
  readonly visibleRows = computed(() => {
    const all = this.rows();
    return all.length > 100 ? all.slice(all.length - 100) : all;
  });

  readonly yMax = computed(() => Math.max(1, this.trend().maxCount));

  readonly yTicks = computed(() => {
    const max = this.yMax();
    return [...new Set([0, Math.ceil(max / 2), max])];
  });

  readonly xTicks = computed(() => {
    const t = this.trend();
    if (t.points.length === 0) return [];
    if (t.tMin === t.tMax) return [t.tMin];
    return [t.tMin, Math.round((t.tMin + t.tMax) / 2), t.tMax];
  });

  /** One path per connected run of points, per severity. */
  readonly paths = computed(() => {
    const t = this.trend();
    return TREND_SERIES.map(s => ({
      key: s.key,
      d: seriesPaths(t, s.key, v => this.x(v), c => this.y(c))
    }));
  });

  /** A text summary of the drawing for assistive technology. */
  readonly description = computed(() => {
    const t = this.trend();
    if (t.points.length === 0) return 'No scans have been recorded.';
    const last = t.points[t.points.length - 1];
    const gaps = t.gaps.length > 0 ? ` ${plural(t.gaps.length, 'gap')} with no scan recorded are shown as gaps.` : '';
    return `Open findings by severity for ${plural(t.points.length, 'recorded scan')}, from ${formatDay(new Date(t.tMin).toISOString())} to ${formatDay(new Date(t.tMax).toISOString())}. The latest scan shows ${plural(last.open, 'open finding')}.${gaps} A table with the same data follows.`;
  });

  readonly formatTime = formatTime;
  readonly gapLabel = gapLabel;
  readonly pointSummary = pointSummary;

  x(t: number): number {
    const tr = this.trend();
    if (tr.tMax === tr.tMin) return (PLOT_LEFT + PLOT_RIGHT) / 2;
    return PLOT_LEFT + ((t - tr.tMin) / (tr.tMax - tr.tMin)) * (PLOT_RIGHT - PLOT_LEFT);
  }

  y(count: number): number {
    return PLOT_BOTTOM - (count / this.yMax()) * (PLOT_BOTTOM - PLOT_TOP);
  }

  coverageY(fraction: number): number {
    return COV_BOTTOM - Math.max(0, Math.min(1, fraction)) * (COV_BOTTOM - COV_TOP);
  }

  tickLabel(t: number): string {
    const tr = this.trend();
    const iso = new Date(t).toISOString();
    // When all recorded scans fall within 48 hours, show time of day so the
    // axis does not repeat the same calendar date three times.
    if (tr.tMax - tr.tMin < 2 * 86_400_000) {
      return formatTime(iso);
    }
    return formatDay(iso);
  }

  tickAnchor(index: number, count: number): 'start' | 'middle' | 'end' {
    if (count <= 1) return 'middle';
    if (index === 0) return 'start';
    if (index === count - 1) return 'end';
    return 'middle';
  }

  /** Marker outline for one point of one series. */
  marker(shape: Shape, cx: number, cy: number): string {
    const r = 4;
    switch (shape) {
      case 'circle':
        return `M${cx - r} ${cy} a${r} ${r} 0 1 0 ${2 * r} 0 a${r} ${r} 0 1 0 ${-2 * r} 0`;
      case 'square':
        return `M${cx - r} ${cy - r} h${2 * r} v${2 * r} h${-2 * r} Z`;
      case 'diamond':
        return `M${cx} ${cy - r - 1} l${r + 1} ${r + 1} l${-(r + 1)} ${r + 1} l${-(r + 1)} ${-(r + 1)} Z`;
      case 'triangle':
        return `M${cx} ${cy - r - 1} l${r + 1} ${2 * r + 1} h${-2 * (r + 1)} Z`;
      case 'cross':
        return `M${cx - r} ${cy - r} l${2 * r} ${2 * r} M${cx + r} ${cy - r} l${-2 * r} ${2 * r}`;
    }
  }

  point(p: TrendPoint, key: TrendSeriesKey): { d: string; hollow: boolean } {
    return { d: this.marker(STYLE[key].shape, this.x(p.t), this.y(p.counts[key])), hollow: p.thin };
  }

  async setRange(value: string): Promise<void> {
    this.range.set(value);
    const days = RANGES.find(r => r.value === value)?.days ?? 0;
    const since = days > 0 ? new Date(Date.now() - days * 86_400_000).toISOString() : '';
    await this.wailsIpc.loadHistorySince(since);
  }
}
