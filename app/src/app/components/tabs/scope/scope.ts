import { Component, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-scope',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './scope.html',
  styleUrls: ['./scope.css']
})
export class ScopeComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  
  seedDomainsList = this.wailsIpc.seedDomainsList;
  seedCidrsList = this.wailsIpc.seedCidrsList;
  seedReposList = this.wailsIpc.seedReposList;

  isAuthorized = this.wailsIpc.isAuthorized;
  signerName = this.wailsIpc.signerName;
  signerTitle = this.wailsIpc.signerTitle;
  authorizationDate = this.wailsIpc.authorizationDate;

  formattedDomains = computed(() => this.seedDomainsList().length > 0 ? this.seedDomainsList().join(', ') : 'None');
  formattedCidrs = computed(() => this.seedCidrsList().length > 0 ? this.seedCidrsList().join(', ') : 'None');
  formattedRepos = computed(() => this.seedReposList().length > 0 ? this.seedReposList().join(', ') : 'None');

  addDomain(value: string) {
    const trimmed = value.trim().toLowerCase();
    if (!trimmed || this.seedDomainsList().includes(trimmed)) return;
    this.seedDomainsList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeDomain(domain: string) {
    this.seedDomainsList.update(list => list.filter(d => d !== domain));
    this.wailsIpc.saveSettings();
  }

  addCidr(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedCidrsList().includes(trimmed)) return;
    this.seedCidrsList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeCidr(cidr: string) {
    this.seedCidrsList.update(list => list.filter(c => c !== cidr));
    this.wailsIpc.saveSettings();
  }

  addRepo(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedReposList().includes(trimmed)) return;
    this.seedReposList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeRepo(repo: string) {
    this.seedReposList.update(list => list.filter(r => r !== repo));
    this.wailsIpc.saveSettings();
  }

  signAuthorization(name: string, title: string) {
    if (!name.trim()) return;
    this.wailsIpc.isAuthorized.set(true);
    this.wailsIpc.signerName.set(name);
    this.wailsIpc.signerTitle.set(title || 'Security Lead');
    this.wailsIpc.authorizationDate.set(new Date().toISOString().split('T')[0]);
    this.wailsIpc.saveSettings();
  }

  revokeAuthorization() {
    this.wailsIpc.isAuthorized.set(false);
    this.wailsIpc.saveSettings();
  }

  wipeAllData() {
    if (window.confirm("WARNING: This will permanently erase all discovered assets, findings, and alerts. Your configuration and target scope will be preserved. Are you sure you want to start over?")) {
      alert("All discovered data has been successfully wiped. The engine is ready for a fresh scan.");
    }
  }
}
