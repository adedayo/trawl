import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-assets',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './assets.html',
  styleUrls: ['./assets.css']
})
export class AssetsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  assets = this.wailsIpc.assets;

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
