import { Component, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-header',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './header.html',
  styleUrls: ['./header.css']
})
export class HeaderComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  activeTab = this.wailsIpc.activeTab;
  isScanning = this.wailsIpc.isScanning;
  isAuthorized = this.wailsIpc.isAuthorized;
  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;

  scanStatusMessage = this.wailsIpc.scanStatusMessage;
  scanStatusTone = this.wailsIpc.scanStatusTone;

  /** Per-check detail from the running assessment, shown beneath the status. */
  assessmentProgress = this.wailsIpc.assessmentProgress;

  // Derived metrics for header badges
  activeAssetCount = computed(() => this.assets().filter(a => a.status === 'active').length);
  kevCount = computed(() => this.findings().filter(f => f.kev).length);

  toggleTheme() {
    this.wailsIpc.theme.update(t => t === 'light' ? 'dark' : 'light');
  }

  triggerScan() {
    if (!this.wailsIpc.isAuthorized()) {
      this.wailsIpc.setScanStatus(
        'Sign the scope authorisation before triggering scans.', 'warning');
      this.wailsIpc.activeTab.set('scope');
      return;
    }

    const targetCount =
      this.wailsIpc.seedDomainsList().length + this.wailsIpc.seedReposList().length;
    if (targetCount === 0) {
      // Nothing is in scope. Previously this fell back to a placeholder
      // domain, which meant the button appeared to work while assessing
      // something the operator had never authorised.
      this.wailsIpc.setScanStatus(
        'No targets in scope. Add a domain or repository before scanning.', 'warning');
      this.wailsIpc.activeTab.set('scope');
      return;
    }

    this.wailsIpc.setScanStatus(
      `Scanning ${targetCount} authorised ${targetCount === 1 ? 'target' : 'targets'}…`);

    // Every authorised target is scanned, not just the first. Scanning one and
    // reporting completion made the results look order-dependent: a domain
    // added later never produced findings, so the estate appeared to shrink.
    this.wailsIpc.scanAuthorisedScope().then(failures => {
      if (failures.length === 0) {
        // A scan that ran but came back incomplete has already been reported
        // by the service, naming the targets. Overwriting that with a success
        // line because no request happened to be rejected would hide the one
        // fact the operator most needs: the estate is not fully assessed.
        if (this.wailsIpc.lastPartialTargets().length > 0) {
          return;
        }
        this.wailsIpc.setScanStatus(
          `Scan complete across ${targetCount} ${targetCount === 1 ? 'target' : 'targets'}.`,
          'success');
        return;
      }
      // Naming the failures matters: the results shown are partial, and an
      // operator who is not told which targets are missing will read the gaps
      // as clean.
      this.wailsIpc.setScanStatus(
        `Scan finished with ${failures.length} of ${targetCount} ${failures.length === 1 ? 'target' : 'targets'} failing: ${failures.join(', ')}.`,
        'error'
      );
    }).catch(err => {
      console.error(err);
      this.wailsIpc.isScanning.set(false);
      this.wailsIpc.setScanStatus(
        `The scan could not be started: ${err instanceof Error ? err.message : err}`, 'error');
    });
  }

  /** Dismisses the status line. */
  dismissStatus() {
    this.wailsIpc.scanStatusMessage.set('');
  }
}
