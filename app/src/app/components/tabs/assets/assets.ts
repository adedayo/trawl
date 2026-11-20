import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { AssetAttribution, AssetContactPanelUI, AssetSoftwareUI, IntervalUI, SoftwareCoverageUI } from '../../../models/types';
import { byHost, HostingSummary, noHosting, summariseHosting } from './hosting';

@Component({
  selector: 'app-assets',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './assets.html'
})
export class AssetsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  assets = this.wailsIpc.assets;

  /**
   * Hosting, indexed by asset. Built once per change rather than per row, so
   * that a portfolio of any size costs one pass instead of one per asset per
   * render.
   */
  private hostingIndex = computed(() => {
    const index = new Map<string, HostingSummary>();
    for (const a of this.wailsIpc.assessments()) {
      index.set(a.assetId, summariseHosting(a));
    }
    return index;
  });

  private attributionIndex = computed(() => {
    const index = new Map<string, AssetAttribution[]>();
    for (const a of this.wailsIpc.assessments()) {
      index.set(a.assetId, a.attribution ?? []);
    }
    return index;
  });

  /** The asset whose hosting detail is open, if any. */
  readonly expandedId = signal<string>('');

  /** The asset whose contact-prior panel is open, if any. */
  readonly contactAssetId = signal<string>('');
  readonly contactPanel = signal<AssetContactPanelUI | null>(null);
  readonly contactLoading = signal<boolean>(false);
  readonly excludedFactors = signal<string[]>([]);

  /** The asset whose software inventory panel is open, if any. */
  readonly softwareAssetId = signal<string>('');

  private softwareByAsset = computed(() => {
    const map = new Map<string, AssetSoftwareUI[]>();
    for (const item of this.wailsIpc.assetSoftware()) {
      const list = map.get(item.assetId) ?? [];
      list.push(item);
      map.set(item.assetId, list);
    }
    return map;
  });

  private softwareCoverageByAsset = computed(() => {
    const map = new Map<string, SoftwareCoverageUI>();
    for (const cov of this.wailsIpc.softwareCoverage()) {
      map.set(cov.assetId, cov);
    }
    return map;
  });

  softwareFor(assetId: string): AssetSoftwareUI[] {
    return this.softwareByAsset().get(assetId) ?? [];
  }

  softwareCoverageFor(assetId: string): SoftwareCoverageUI {
    return (
      this.softwareCoverageByAsset().get(assetId) ?? {
        assetId,
        state: 'not_assessed',
        identified: 0,
        versionUnknown: 0,
        unmapped: 0,
        superseded: 0
      }
    );
  }

  softwareCoverageLabel(cov: SoftwareCoverageUI): string {
    switch (cov.state) {
      case 'identified':
        return `${cov.identified} identified`;
      case 'version_unknown':
        return `${cov.versionUnknown} version unknown`;
      case 'seen_unmapped':
        return `${cov.unmapped} unmapped`;
      default:
        return 'Not assessed';
    }
  }

  toggleSoftwarePanel(assetId: string): void {
    this.softwareAssetId.update(current => (current === assetId ? '' : assetId));
  }

  hosting(assetId: string): HostingSummary {
    return this.hostingIndex().get(assetId) ?? noHosting;
  }

  hostGroups(assetId: string): { host: string; rows: AssetAttribution[] }[] {
    return byHost(this.attributionIndex().get(assetId) ?? []);
  }

  toggleHosting(assetId: string): void {
    this.expandedId.update(current => (current === assetId ? '' : assetId));
  }

  async toggleContactPanel(assetId: string): Promise<void> {
    if (this.contactAssetId() === assetId) {
      this.contactAssetId.set('');
      this.contactPanel.set(null);
      this.excludedFactors.set([]);
      return;
    }
    this.contactAssetId.set(assetId);
    this.excludedFactors.set([]);
    await this.reloadContactPanel(assetId, []);
  }

  async toggleExcludedFactor(factorId: string): Promise<void> {
    const assetId = this.contactAssetId();
    if (!assetId || factorId === 'reachability' || factorId.startsWith('ambient:') || factorId === 'time-at-risk') {
      return;
    }
    const current = new Set(this.excludedFactors());
    if (current.has(factorId)) {
      current.delete(factorId);
    } else {
      current.add(factorId);
    }
    const next = Array.from(current);
    this.excludedFactors.set(next);
    await this.reloadContactPanel(assetId, next);
  }

  isFactorExcluded(factorId: string): boolean {
    return this.excludedFactors().includes(factorId);
  }

  canRemoveFactor(factorId: string): boolean {
    return factorId.startsWith('discoverability:') || factorId.startsWith('barrier:');
  }

  formatProbInterval(iv: IntervalUI): string {
    const pct = (n: number) => `${(n * 100).toFixed(1)}%`;
    return `${pct(iv.low)} - ${pct(iv.high)} (central ${pct(iv.point)})`;
  }

  formatRateInterval(iv: IntervalUI): string {
    return `${iv.low.toFixed(2)} - ${iv.high.toFixed(2)} / day (central ${iv.point.toFixed(2)})`;
  }

  private async reloadContactPanel(assetId: string, excluded: string[]): Promise<void> {
    this.contactLoading.set(true);
    try {
      this.contactPanel.set(await this.wailsIpc.getAssetContactPanel(assetId, excluded));
    } finally {
      this.contactLoading.set(false);
    }
  }

  /** Assets with an action in flight, so their buttons can be disabled. */
  private pending = signal<Set<string>>(new Set());

  /** The last failure, shown in the row rather than swallowed to the console. */
  readonly actionError = signal<string>('');

  /** The asset awaiting an in-row confirmation, if any. */
  readonly confirmingId = signal<string>('');

  isPending(id: string): boolean {
    return this.pending().has(id);
  }

  /** The display name of an asset, used in the confirmation prompt. */
  assetName(id: string): string {
    return this.assets().find((a: any) => a.id === id)?.value ?? id;
  }

  /**
   * Asks for confirmation in the row rather than through window.confirm.
   *
   * The desktop build runs in a WKWebView with no native JavaScript dialogs,
   * so confirm() returned falsy and the removal was abandoned without a word.
   */
  requestRemove(id: string): void {
    this.actionError.set('');
    this.confirmingId.set(id);
  }

  cancelRemove(): void {
    this.confirmingId.set('');
  }

  removeAsset(id: string): Promise<void> {
    this.confirmingId.set('');
    return this.run(id, () => this.wailsIpc.removeAsset(id));
  }

  /**
   * Runs an action, surfacing failure rather than leaving the row unchanged
   * with no explanation. A button that silently does nothing is
   * indistinguishable from one that worked and had no effect.
   */
  private async run(id: string, action: () => Promise<void>): Promise<void> {
    this.pending.set(new Set(this.pending()).add(id));
    this.actionError.set('');
    try {
      await action();
    } catch (e) {
      console.error('Trawl: the asset action failed:', e);
      this.actionError.set(
        `That action failed and nothing was changed: ${e instanceof Error ? e.message : e}`
      );
    } finally {
      const next = new Set(this.pending());
      next.delete(id);
      this.pending.set(next);
    }
  }
}
