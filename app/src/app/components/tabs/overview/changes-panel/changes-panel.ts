import { Component, computed, inject, PendingTasks, signal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { ChangeSet, FindingChange, ScanRunView } from '../../../../models/types';
import { formatDay, formatTime, plural, sourceLabel } from '../../../../dashboard/findings';
import {
  bucketItems,
  CHANGE_BUCKETS,
  ChangeBucket,
  comparedLine,
  coverageDropText,
  coverageText,
  hasBaseline
} from '../../../../dashboard/history';

/** How many items a bucket lists before the operator asks for the rest. */
export const BUCKET_PREVIEW = 5;

/**
 * "Since the last scan", and a control to compare any two scans (Change 025,
 * Phase 5).
 *
 * The counts are only as meaningful as the coverage behind them, so each
 * comparison states the coverage of both runs and, when the later run checked
 * materially less, says so beside the counts. The engine's own note is shown
 * verbatim: it may carry a limit this view cannot derive.
 */
@Component({
  selector: 'app-changes-panel',
  standalone: true,
  imports: [NgTemplateOutlet],
  templateUrl: './changes-panel.html'
})
export class ChangesPanelComponent {
  readonly wailsIpc = inject(WailsIpcService);
  readonly theme = this.wailsIpc.theme;
  private readonly pending = inject(PendingTasks);

  readonly changeSet = this.wailsIpc.changeSet;
  readonly state = this.wailsIpc.changeSetState;

  readonly buckets = CHANGE_BUCKETS;
  readonly bucketItems = bucketItems;
  readonly comparedLine = comparedLine;
  readonly coverageDropText = coverageDropText;
  readonly coverageText = coverageText;
  readonly hasBaseline = hasBaseline;
  readonly formatDay = formatDay;
  readonly sourceLabel = sourceLabel;
  readonly preview = BUCKET_PREVIEW;

  /** Buckets the operator has asked to see in full, keyed by panel and bucket. */
  readonly expanded = signal<ReadonlySet<string>>(new Set());

  // Compare scans
  readonly fromRun = signal('');
  readonly toRun = signal('');
  readonly comparison = signal<ChangeSet | null>(null);
  readonly comparisonState = signal<'idle' | 'loading' | 'ready' | 'error'>('idle');
  readonly comparisonError = signal('');

  /** Runs, newest first, whichever way the engine ordered them. */
  readonly runs = computed(() =>
    [...this.wailsIpc.scanRuns()].sort((a, b) => Date.parse(b.startedAt) - Date.parse(a.startedAt))
  );

  private runById(id: string): ScanRunView | undefined {
    return this.runs().find(r => r.id === id);
  }

  /** Why Compare is not available yet, or '' when it is. */
  readonly compareBlocked = computed(() => {
    const from = this.runById(this.fromRun());
    const to = this.runById(this.toRun());
    if (!from || !to) return 'Choose two scans to compare.';
    if (from.id === to.id) return 'Choose two different scans.';
    if (Date.parse(from.startedAt) > Date.parse(to.startedAt)) return 'Choose the earlier scan first.';
    return '';
  });

  readonly compared = computed(() => this.comparison());

  runLabel(run: ScanRunView): string {
    const target = run.assetValue ? ` · ${run.assetValue}` : '';
    return `${formatTime(run.finishedAt || run.startedAt)} · ${run.kind}${target} · ${Math.round(run.coverageFraction * 100)}% coverage`;
  }

  isExpanded(panel: string, bucket: ChangeBucket): boolean {
    return this.expanded().has(`${panel}:${bucket}`);
  }

  toggle(panel: string, bucket: ChangeBucket): void {
    const next = new Set(this.expanded());
    const key = `${panel}:${bucket}`;
    if (next.has(key)) next.delete(key);
    else next.add(key);
    this.expanded.set(next);
  }

  shown(items: readonly FindingChange[], panel: string, bucket: ChangeBucket): readonly FindingChange[] {
    return this.isExpanded(panel, bucket) ? items : items.slice(0, BUCKET_PREVIEW);
  }

  count(n: number): string {
    return plural(n, 'finding');
  }

  open(change: FindingChange): void {
    this.wailsIpc.openFinding(change.finding.key);
  }

  itemLabel(change: FindingChange, bucket: ChangeBucket): string {
    const f = change.finding;
    return `${bucket}: ${f.severity} severity, ${f.title}${f.assetValue ? ' on ' + f.assetValue : ''}. Open details.`;
  }

  async compare(): Promise<void> {
    if (this.compareBlocked()) {
      return;
    }
    this.comparisonState.set('loading');
    this.comparisonError.set('');
    await this.pending.run(async () => {
      try {
        this.comparison.set(await this.wailsIpc.compareScans(this.fromRun(), this.toRun()));
        this.comparisonState.set('ready');
      } catch (error) {
        this.comparison.set(null);
        this.comparisonError.set(error instanceof Error ? error.message : 'The comparison could not be loaded.');
        this.comparisonState.set('error');
      }
    });
  }

  severityClasses(severity: string): string {
    switch (severity) {
      case 'critical':
        return 'bg-rose-600 text-white';
      case 'high':
        return 'bg-amber-500 text-slate-950';
      case 'medium':
        return 'bg-blue-600 text-white';
      case 'low':
        return 'bg-slate-600 text-white';
      default:
        return 'bg-slate-500 text-white';
    }
  }
}
