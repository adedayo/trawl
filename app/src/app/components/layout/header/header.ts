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

  // Derived metrics for header badges
  activeAssetCount = computed(() => this.assets().filter(a => a.status === 'active').length);
  kevCount = computed(() => this.findings().filter(f => f.kev).length);

  toggleTheme() {
    this.wailsIpc.theme.update(t => t === 'light' ? 'dark' : 'light');
  }

  triggerScan() {
    if (!this.wailsIpc.isAuthorized()) {
      this.wailsIpc.scanStatusMessage.set('Action Required: Please sign digital scope authorization in Tab 6 before triggering scans.');
      this.wailsIpc.activeTab.set('scope');
      return;
    }
    
    this.wailsIpc.scanStatusMessage.set('Enqueueing scan job for targets...');
    this.wailsIpc.isScanning.set(true);
    
    // Call Go backend to start the native scan engines
    // (In a real scenario, domain and repoUrl would come from the authorized scope list)
    const domain = this.wailsIpc.seedDomainsList().length > 0 ? this.wailsIpc.seedDomainsList()[0] : 'example.com';
    const repoUrl = this.wailsIpc.seedReposList().length > 0 ? this.wailsIpc.seedReposList()[0] : 'https://github.com/example/test';
    
    this.wailsIpc.triggerScan(domain, repoUrl).then(() => {
      this.wailsIpc.scanStatusMessage.set('Scan engine started natively...');
    }).catch(err => {
      console.error(err);
      this.wailsIpc.isScanning.set(false);
      this.wailsIpc.scanStatusMessage.set('Error starting scan.');
    });
  }
}
