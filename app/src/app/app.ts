import { Component, signal, computed, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from './wails-ipc.service';
import { 
  AssetUI, 
  FindingUI, 
  EmailPostureUI, 
  SecretFindingUI 
} from './models/types';

import { HeaderComponent } from './components/layout/header/header';
import { OverviewComponent } from './components/tabs/overview/overview';
import { AssetsComponent } from './components/tabs/assets/assets';
import { FindingsComponent } from './components/tabs/findings/findings';
import { EmailPostureComponent } from './components/tabs/email-posture/email-posture';
import { SecretsComponent } from './components/tabs/secrets/secrets';
import { ScopeComponent } from './components/tabs/scope/scope';

@Component({
  selector: 'trawl-root',
  standalone: true,
  imports: [
    CommonModule,
    HeaderComponent,
    OverviewComponent,
    AssetsComponent,
    FindingsComponent,
    EmailPostureComponent,
    SecretsComponent,
    ScopeComponent
  ],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit, OnDestroy {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  activeTab = this.wailsIpc.activeTab;

  ngOnInit() {
    this.wailsIpc.refreshAssets();
    this.wailsIpc.loadSettings();
    this.wailsIpc.refreshEmailPostures();
    this.wailsIpc.refreshAssessments();
    this.wailsIpc.refreshSecretFindings();
  }

  ngOnDestroy() {
    // cleanup
  }
}
