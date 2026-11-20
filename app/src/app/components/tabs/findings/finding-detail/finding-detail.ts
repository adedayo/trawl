import { afterNextRender, Component, computed, effect, ElementRef, inject, Injector, input, output, PendingTasks, signal, untracked, viewChild } from '@angular/core';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { FindingDetail, FindingEventView } from '../../../../models/types';
import {
  ageLine,
  causeLabel,
  checkStateLabel,
  eventTypeLabel,
  formatDay,
  formatTime,
  isNotEstateChange,
  narrativeLevelLine,
  orderTimeline,
  sourceLabel,
  statusLabel,
  transition
} from '../../../../dashboard/findings';

type LoadState = 'loading' | 'ready' | 'notfound' | 'error';

/**
 * One finding in full: facts, the executive explanation, evidence, the
 * timeline of everything that happened to it, and what it rests on.
 *
 * It is an inline panel that replaces the list rather than a dialog over it,
 * so there is no focus trap to build and get wrong. On open, focus moves to the
 * heading; Escape and the Back button close it, and the Findings tab returns
 * focus to the row that opened it.
 *
 * The explanation is the engine's deterministic wording, rendered as given.
 * No AI annotation appears here, whatever legacy rows may hold: it is advisory
 * and subordinate, and this change renders none of it.
 */
@Component({
  selector: 'app-finding-detail',
  standalone: true,
  templateUrl: './finding-detail.html',
  host: { '(document:keydown.escape)': 'close()' }
})
export class FindingDetailComponent {
  private readonly wailsIpc = inject(WailsIpcService);
  private readonly injector = inject(Injector);
  private readonly pending = inject(PendingTasks);
  readonly theme = this.wailsIpc.theme;

  /** The key to show. */
  readonly findingKey = input.required<string>();
  /** Emitted when the operator closes the detail by any route. */
  readonly closed = output<void>();

  readonly state = signal<LoadState>('loading');
  readonly detail = signal<FindingDetail | null>(null);
  readonly errorMessage = signal('');

  private readonly heading = viewChild<ElementRef<HTMLElement>>('heading');

  /** The list's own summary, so the heading has a name while the detail loads. */
  private readonly summary = computed(() => this.wailsIpc.findingIndex().find(f => f.key === this.findingKey()) ?? null);

  readonly title = computed(() => this.detail()?.title ?? this.summary()?.title ?? 'Finding');

  /** The timeline in time order. */
  readonly timeline = computed(() => orderTimeline(this.detail()?.timeline ?? []));

  readonly levelLine = computed(() => {
    const d = this.detail();
    return d ? narrativeLevelLine(d.explanation) : '';
  });

  readonly narrativeVersion = computed(() => {
    const d = this.detail();
    return d ? d.narrativeVersion || d.explanation?.version || '' : '';
  });

  private requestId = 0;

  readonly formatTime = formatTime;
  readonly formatDay = formatDay;
  readonly ageLine = ageLine;
  readonly sourceLabel = sourceLabel;
  readonly statusLabel = statusLabel;
  readonly eventTypeLabel = eventTypeLabel;
  readonly causeLabel = causeLabel;
  readonly checkStateLabel = checkStateLabel;
  readonly isNotEstateChange = isNotEstateChange;

  constructor() {
    effect(() => {
      const key = this.findingKey();
      // Registered as a pending task so the view is not considered stable, in
      // tests or in prerendering, while the finding is still being fetched.
      untracked(() => void this.pending.run(() => this.load(key)));
    });

    afterNextRender(() => this.heading()?.nativeElement.focus(), { injector: this.injector });
  }

  private async load(key: string): Promise<void> {
    const request = ++this.requestId;
    this.state.set('loading');
    this.detail.set(null);
    this.errorMessage.set('');
    try {
      const detail = await this.wailsIpc.getFindingDetail(key);
      if (request !== this.requestId) {
        return;
      }
      if (!detail) {
        this.state.set('notfound');
        return;
      }
      this.detail.set(detail);
      this.state.set('ready');
    } catch (error) {
      if (request !== this.requestId) {
        return;
      }
      this.errorMessage.set(error instanceof Error ? error.message : 'The finding could not be loaded.');
      this.state.set('error');
    }
  }

  close(): void {
    this.closed.emit();
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

  /** `high to critical`, or '' when the event recorded no change in the field. */
  severityChange(e: FindingEventView): string {
    return transition(e.prevSeverity, e.severity);
  }

  priorityChange(e: FindingEventView): string {
    return transition(e.prevPriority, e.priority);
  }

  isHttpLink(reference: string): boolean {
    return /^https?:\/\//i.test(reference);
  }
}
