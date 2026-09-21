import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-overview',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './overview.html'
})
export class OverviewComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  activeTab = this.wailsIpc.activeTab;
  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;
  assessments = this.wailsIpc.assessments;
  probing = signal(false);

  exposedServices = computed(() => this.assessments()
    .flatMap(assessment => assessment.serviceExposures ?? [])
    .filter(exposure => exposure.stillExposed)
    .slice(0, 10));

  recentServiceEvidence = computed(() => this.assessments()
    .flatMap(assessment => assessment.serviceObservations ?? [])
    .filter(observation => observation.state === 'responding')
    .sort((a, b) => b.observedAt.localeCompare(a.observedAt))
    .slice(0, 10));

  async probeServices(profile: 'most-common' | 'extended'): Promise<void> {
    this.probing.set(true);
    try {
      await this.wailsIpc.probeDiscoveredServices(profile);
    } finally {
      this.probing.set(false);
    }
  }

  pendingAssetCount = computed(() => this.assets().filter(a => a.status === 'pending').length);

  catalogueDate(finding: any, feed: string): string {
    return finding.enrichments?.find((item: any) => item.feed === feed)?.snapshot?.retrievedAt || 'not retrieved';
  }
}
