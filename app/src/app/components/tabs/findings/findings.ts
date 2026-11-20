import { afterNextRender, Component, computed, effect, ElementRef, inject, Injector, signal, untracked } from '@angular/core';
import { WailsIpcService } from '../../../wails-ipc.service';
import { FindingSummary, ScanRunView } from '../../../models/types';
import {
  assetOptions,
  changedSinceCutoff,
  ChangedSinceOption,
  DEFAULT_FILTERS,
  eventTypeLabel,
  filterFindings,
  FindingListFilters,
  formatDay,
  formatTime,
  plural,
  sortFindings,
  sourceLabel,
  statusLabel
} from '../../../dashboard/findings';
import { coverageText } from '../../../dashboard/history';
import { FindingDetailComponent } from './finding-detail/finding-detail';

/**
 * The unified findings list: every source in one place, filterable, with each
 * row a real button that opens that finding's detail (Change 025).
 *
 * Two rules govern what this view may say. It never reads an empty list as a
 * clear surface: an empty index can mean nothing was found, nothing was
 * checked, or the request failed, and the text names which it knows. And it
 * keeps a finding that is open only because its check could not conclude
 * visibly apart from one that was re-assessed and is still present.
 */
@Component({
  selector: 'app-findings',
  standalone: true,
  imports: [FindingDetailComponent],
  templateUrl: './findings.html'
})
export class FindingsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  /** The legacy list, still loaded for the KEV enrichment notice below. */
  findings = this.wailsIpc.findings;
  index = this.wailsIpc.findingIndex;
  indexState = this.wailsIpc.findingIndexState;
  indexError = this.wailsIpc.findingIndexError;
  selectedKey = this.wailsIpc.selectedFindingKey;

  /** The clock, replaceable so a test can fix "now". */
  now: () => number = () => Date.now();

  readonly filters = signal<FindingListFilters>({ ...DEFAULT_FILTERS });

  readonly statusOptions = [
    { value: 'active', label: 'Active (open and reopened)' },
    { value: 'open', label: 'Open' },
    { value: 'resolved', label: 'Resolved' },
    { value: 'reopened', label: 'Reopened' },
    { value: 'all', label: 'All' }
  ] as const;
  readonly severityOptions = [
    { value: 'all', label: 'All severities' },
    { value: 'critical', label: 'Critical' },
    { value: 'high', label: 'High' },
    { value: 'medium', label: 'Medium' },
    { value: 'low', label: 'Low' },
    { value: 'info', label: 'Info' }
  ] as const;
  readonly sourceOptions = [
    { value: 'all', label: 'All sources' },
    { value: 'signal', label: 'Signals (email, DNS, TLS)' },
    { value: 'finding', label: 'Scan findings (CVE, services)' },
    { value: 'secret', label: 'Secrets' },
    { value: 'candidate', label: 'Software candidates (unconfirmed)' }
  ] as const;
  readonly changedOptions: readonly { value: ChangedSinceOption; label: string }[] = [
    { value: 'any', label: 'Any time' },
    { value: 'last-scan', label: 'Since the last scan' },
    { value: '24h', label: 'Last 24 hours' },
    { value: '7d', label: 'Last 7 days' },
    { value: '30d', label: 'Last 30 days' }
  ];

  /** The newest run in the ledger, whichever way the engine ordered it. */
  readonly latestRun = computed(() =>
    this.wailsIpc.scanRuns().reduce<ScanRunView | null>(
      (best, run) => (!best || Date.parse(run.startedAt) > Date.parse(best.startedAt) ? run : best),
      null
    )
  );

  private readonly cutoff = computed(() =>
    changedSinceCutoff(this.filters().changedSince, this.now(), this.latestRun()?.startedAt ?? null)
  );

  readonly assets = computed(() => assetOptions(this.index()));

  readonly visible = computed(() =>
    sortFindings(filterFindings(this.index(), this.filters(), this.cutoff()))
  );

  /** True when "since the last scan" was chosen but no scan is on record. */
  readonly lastScanUnknown = computed(() => this.cutoff() === 'unavailable');

  readonly uncheckedKevCount = computed(() =>
    this.findings().filter(f => {
      if (f.status === 'resolved' || !f.cveId) {
        return false;
      }
      const state = f.enrichments?.find((e: any) => e.feed === 'cisa-kev')?.state ?? 'not_checked';
      return state === 'not_checked' || state === 'check_failed';
    }).length
  );

  readonly sourceLabel = sourceLabel;
  readonly statusLabel = statusLabel;
  readonly formatDay = formatDay;
  readonly formatTime = formatTime;
  readonly coverageText = coverageText;

  constructor() {
    // Closing the detail, by any route (button, Escape, the back button, a
    // changed address), returns focus to the row that opened it. Without that
    // a keyboard user is dropped at the top of the page and has to find their
    // place again.
    let previous: string | null = null;
    effect(() => {
      const key = this.selectedKey();
      untracked(() => {
        if (key) {
          previous = key;
        } else if (previous) {
          const returnTo = previous;
          previous = null;
          afterNextRender(() => this.focusRow(returnTo), { injector: this.injector });
        }
      });
    });
  }

  setFilter<K extends keyof FindingListFilters>(name: K, value: string): void {
    this.filters.update(f => ({ ...f, [name]: value as FindingListFilters[K] }));
  }

  resetFilters(): void {
    this.filters.set({ ...DEFAULT_FILTERS });
  }

  /** True when the filters differ from the default view. */
  readonly filtered = computed(() => {
    const f = this.filters();
    return (Object.keys(DEFAULT_FILTERS) as (keyof FindingListFilters)[]).some(k => f[k] !== DEFAULT_FILTERS[k]);
  });

  open(finding: FindingSummary): void {
    this.wailsIpc.openFinding(finding.key);
  }

  closeDetail(): void {
    this.wailsIpc.closeFinding();
  }

  private focusRow(key: string): void {
    const root = this.host.nativeElement;
    const row = Array.from(root.querySelectorAll<HTMLElement>('[data-finding-key]')).find(
      el => el.getAttribute('data-finding-key') === key
    );
    // The row can be absent: the filters may hide it, or the finding may have
    // gone. Fall back to the heading so focus still lands somewhere sensible.
    (row ?? root.querySelector<HTMLElement>('[data-findings-heading]'))?.focus();
  }

  rowLabel(f: FindingSummary): string {
    const parts = [
      `${f.severity} severity`,
      statusLabel(f.status).toLowerCase(),
      f.notReassessed ? 'not re-assessed' : '',
      f.title,
      f.assetValue ? `on ${f.assetValue}` : '',
      sourceLabel(f.source),
      `first seen ${formatDay(f.firstSeen)}`,
      f.lastChange ? `last change ${eventTypeLabel(f.lastChangeType).toLowerCase()} ${formatDay(f.lastChange)}` : ''
    ].filter(Boolean);
    return `${parts.join(', ')}. Open details.`;
  }

  ageText(f: FindingSummary): string {
    return f.status === 'resolved'
      ? `${plural(f.ageDays, 'day')} from first seen to resolution`
      : `${plural(f.ageDays, 'day')} since first seen`;
  }

  lastChangeText(f: FindingSummary): string {
    return f.lastChange ? `${eventTypeLabel(f.lastChangeType)} ${formatDay(f.lastChange)}` : 'not recorded';
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

  statusClasses(status: string): string {
    switch (status) {
      case 'resolved':
        return 'bg-emerald-600 text-white';
      case 'reopened':
        return 'bg-rose-800 text-white';
      default:
        return 'bg-slate-600 text-white';
    }
  }
}
