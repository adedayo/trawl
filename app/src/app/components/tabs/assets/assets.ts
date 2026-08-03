import { Component, inject } from '@angular/core';
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
  
  approveAsset(id: string) {}
  rejectAsset(id: string) {}
  removeAsset(id: string) {}
}
