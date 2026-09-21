import { Component, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { ThemeService } from '../../../theme.service';

@Component({
  selector: 'app-header',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './header.html'
})
export class HeaderComponent {
  wailsIpc = inject(WailsIpcService);
  private themes = inject(ThemeService);
  theme = this.wailsIpc.theme;
  /** True while the interface is tracking the operating system's appearance. */
  followsEnvironment = this.themes.followsEnvironment;
  activeTab = this.wailsIpc.activeTab;
  isScanning = this.wailsIpc.isScanning;
  isAuthorized = this.wailsIpc.isAuthorized;
  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;

  /**
   * The engine's version, read from the running binary.
   *
   * Empty until the first successful read, and the badge is hidden in that
   * case. This was `v0.1.0-OSS`, hardcoded in the template, which is a claim
   * about which build produced what is on screen — and it went on making that
   * claim, unchanged, across every release after the one it was typed in.
   */
  engineVersion = this.wailsIpc.engineVersion;

  scanStatusMessage = this.wailsIpc.scanStatusMessage;
  scanStatusTone = this.wailsIpc.scanStatusTone;

  /** Per-check detail from the running assessment, shown beneath the status. */
  assessmentProgress = this.wailsIpc.assessmentProgress;

  // Derived metrics for header badges
  activeAssetCount = computed(() => this.assets().filter(a => a.status === 'active').length);
  kevCount = computed(() => this.findings().filter(f => f.kev).length);

  /**
   * Switches appearance and records the choice.
   *
   * Recording it is the point: until the operator touches this, the interface
   * follows their system, and from the moment they do it follows them instead
   * — on this launch and every later one.
   */
  toggleTheme() {
    this.themes.toggle();
  }

  /** Withdraws that choice, so the system's appearance is followed again. */
  followSystemTheme() {
    this.themes.followEnvironment();
  }

  /** Wording for the appearance control, which states what it will do. */
  themeButtonTitle = computed(() =>
    this.theme() === 'dark' ? 'Switch to light appearance' : 'Switch to dark appearance'
  );

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
