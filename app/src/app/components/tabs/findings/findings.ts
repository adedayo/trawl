import { Component, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { isSnapshotStale } from '../../../dashboard/estate';

@Component({
  selector: 'app-findings',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './findings.html'
})
export class FindingsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  findings = this.wailsIpc.findings;

  readonly uncheckedKevCount = computed(() =>
    this.findings().filter(f => {
      if (f.status === 'resolved' || !f.cveId) {
        return false;
      }
      const state = f.enrichments?.find((e: any) => e.feed === 'cisa-kev')?.state ?? 'not_checked';
      return state === 'not_checked' || state === 'check_failed';
    }).length
  );

  catalogueDate(finding: any, feed: string): string {
    const enrichment = finding.enrichments?.find((item: any) => item.feed === feed);
    const retrievedAt =
      enrichment?.snapshot?.retrievedAt ??
      this.wailsIpc.feedStatuses().find(s => s.feed === feed)?.snapshot?.retrievedAt;
    if (!retrievedAt) {
      return enrichment?.state === 'check_failed' ? 'check failed' : 'not retrieved';
    }
    return isSnapshotStale(retrievedAt) ? `${retrievedAt} (stale)` : retrievedAt;
  }

  isFeedStale(finding: any, feed: string): boolean {
    const enrichment = finding.enrichments?.find((item: any) => item.feed === feed);
    const retrievedAt =
      enrichment?.snapshot?.retrievedAt ??
      this.wailsIpc.feedStatuses().find(s => s.feed === feed)?.snapshot?.retrievedAt;
    return isSnapshotStale(retrievedAt);
  }
}
