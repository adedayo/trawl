import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-email-posture',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './email-posture.html',
  styleUrls: ['./email-posture.css']
})
export class EmailPostureComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  emailPostures = this.wailsIpc.emailPostures;
}
