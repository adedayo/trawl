import { Component, signal, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { SecretFindingUI } from '../../../models/types';

@Component({
  selector: 'app-secrets',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './secrets.html',
  styleUrls: ['./secrets.css']
})
export class SecretsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  secretFindings = this.wailsIpc.secretFindings;
  
  filterSecretPriority = signal<string>('all');
  filterSecretStatus = signal<string>('all');
  selectedSecret = signal<SecretFindingUI | null>(null);
  activeCheckmateVersion = signal<string>('v1.3.3');

  filteredSecretFindings = computed(() => {
    let list = this.secretFindings();
    const prio = this.filterSecretPriority();
    const status = this.filterSecretStatus();
    if (prio !== 'all') {
      list = list.filter(s => s.priority === prio);
    }
    if (status !== 'all') {
      list = list.filter(s => s.status === status);
    }
    return list;
  });

  viewSecretDetails(secret: SecretFindingUI) {
    this.selectedSecret.set(secret);
  }

  closeSecretDetails() {
    this.selectedSecret.set(null);
  }
}
