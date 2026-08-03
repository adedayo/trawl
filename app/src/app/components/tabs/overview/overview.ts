import { Component, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-overview',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './overview.html',
  styleUrls: ['./overview.css']
})
export class OverviewComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  activeTab = this.wailsIpc.activeTab;
  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;

  pendingAssetCount = computed(() => this.assets().filter(a => a.status === 'pending').length);
}
