import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-findings',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './findings.html',
  styleUrls: ['./findings.css']
})
export class FindingsComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  findings = this.wailsIpc.findings;
}
