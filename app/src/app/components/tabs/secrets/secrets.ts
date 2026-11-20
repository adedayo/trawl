import { Component, signal, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { SecretFindingUI } from '../../../models/types';
import { findSummaryForSecret } from '../../../dashboard/findings';

@Component({
  selector: 'app-secrets',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './secrets.html'
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

  /**
   * The key of the finding that records the selected secret, or null.
   *
   * Looked up in the findings index, which is the source of truth for keys.
   * When no entry matches unambiguously the link is not offered, because a link
   * to the wrong finding would be worse than none.
   */
  selectedFindingKey = computed(() => {
    const secret = this.selectedSecret();
    const index = this.wailsIpc.findingIndex();
    return secret && index.length > 0 ? findSummaryForSecret(index, secret)?.key ?? null : null;
  });

  openFinding(key: string) {
    this.wailsIpc.openFinding(key);
  }

  viewSecretDetails(secret: SecretFindingUI) {
    this.selectedSecret.set(secret);
  }

  closeSecretDetails() {
    this.selectedSecret.set(null);
  }
}
