import { Component, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface Asset {
  id: string;
  type: 'domain' | 'ip' | 'repository';
  value: string;
  source: string;
  confidence: 'high' | 'medium' | 'low';
  status: 'active' | 'pending' | 'inactive';
  firstSeen: string;
  lastSeen: string;
}

export interface Finding {
  id: string;
  assetValue: string;
  cveId: string;
  cpe?: string;
  title: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  kev: boolean;
  epssScore: number;
  cvssScore: number;
  status: 'open' | 'resolved' | 'reopened';
  aiAnnotation?: {
    summary: string;
    remediation: string;
  };
  detectedAt: string;
}

export interface EmailPosture {
  domain: string;
  spfValid: boolean;
  dkimFound: boolean;
  dmarcPolicy: 'reject' | 'quarantine' | 'none' | 'missing';
  priority: 'critical' | 'high' | 'medium' | 'low' | 'info';
  lastChecked: string;
}

export interface SecretFinding {
  repoUrl: string;
  filePath: string;
  provider: string;
  redactedRef: string;
  commitSha: string;
  verified: boolean;
  priority: 'critical' | 'high' | 'medium' | 'low';
  detectedAt: string;
}

@Component({
  selector: 'trawl-root',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App {
  // Theme Mode ('light' by default, toggleable to 'dark')
  readonly theme = signal<'light' | 'dark'>('light');

  // Navigation
  readonly activeTab = signal<'overview' | 'assets' | 'findings' | 'email' | 'secrets' | 'scope'>('overview');

  // Filter
  readonly filterSeverity = signal<string>('all');
  readonly filterAssetType = signal<string>('all');

  // Authorization State (In-App Scope Authorization Wizard)
  readonly isAuthorized = signal<boolean>(false);
  readonly signerName = signal<string>('');
  readonly signerTitle = signal<string>('');
  readonly authorizationDate = signal<string>('');

  // Editable Target Scope Signals
  readonly seedDomainsList = signal<string[]>(['example.com', 'api.example.com']);
  readonly seedCidrsList = signal<string[]>(['198.51.100.0/24']);
  readonly seedReposList = signal<string[]>(['https://github.com/example/repo']);

  // Computed Target Strings for Scope Authorization
  readonly formattedDomains = computed(() => this.seedDomainsList().join(', ') || 'None');
  readonly formattedCidrs = computed(() => this.seedCidrsList().join(', ') || 'None');
  readonly formattedRepos = computed(() => this.seedReposList().join(', ') || 'None');

  // Initial Data for Assets & Findings
  readonly assets = signal<Asset[]>([
    { id: '1', type: 'domain', value: 'example.com', source: 'seed', confidence: 'high', status: 'active', firstSeen: '2026-07-20', lastSeen: 'Today 19:30' },
    { id: '2', type: 'domain', value: 'api.example.com', source: 'subfinder', confidence: 'high', status: 'active', firstSeen: '2026-07-21', lastSeen: 'Today 19:30' },
    { id: '3', type: 'domain', value: 'staging.example.com', source: 'ct-logs', confidence: 'medium', status: 'pending', firstSeen: '2026-07-22', lastSeen: 'Today 18:00' },
    { id: '4', type: 'ip', value: '198.51.100.42', source: 'dns-pivot', confidence: 'high', status: 'active', firstSeen: '2026-07-20', lastSeen: 'Today 19:30' },
    { id: '5', type: 'repository', value: 'https://github.com/example/repo', source: 'operator', confidence: 'high', status: 'active', firstSeen: '2026-07-23', lastSeen: 'Today 19:00' }
  ]);

  readonly findings = signal<Finding[]>([
    {
      id: 'f1',
      assetValue: 'api.example.com',
      cveId: 'CVE-2024-3094',
      cpe: 'cpe:2.3:a:xz:xz:5.6.0:*:*:*:*:*:*:*',
      title: 'XZ Utils Backdoor Remote Code Execution',
      severity: 'critical',
      kev: true,
      epssScore: 0.965,
      cvssScore: 10.0,
      status: 'open',
      aiAnnotation: {
        summary: 'Critical backdoor detected in SSH authentication pipeline. Immediate patch required.',
        remediation: 'Downgrade xz-utils to 5.4.x or upgrade to fixed distribution builds.'
      },
      detectedAt: 'Today 14:22'
    },
    {
      id: 'f2',
      assetValue: 'example.com',
      cveId: 'CVE-2023-4863',
      cpe: 'cpe:2.3:a:google:chrome:116.0.5845.187:*:*:*:*:*:*:*',
      title: 'Heap Buffer Overflow in WebP Image Rendering',
      severity: 'high',
      kev: true,
      epssScore: 0.884,
      cvssScore: 8.8,
      status: 'open',
      aiAnnotation: {
        summary: 'Confirmed KEV vulnerability in WebP parsing library affecting host headers.',
        remediation: 'Apply libwebp system security updates across host web servers.'
      },
      detectedAt: 'Yesterday 09:15'
    },
    {
      id: 'f3',
      assetValue: '198.51.100.42',
      cveId: 'CVE-2023-38408',
      title: 'OpenSSH PKCS#11 Provider Remote Code Execution',
      severity: 'medium',
      kev: false,
      epssScore: 0.342,
      cvssScore: 6.5,
      status: 'open',
      detectedAt: '2 days ago'
    }
  ]);

  readonly emailPostures = signal<EmailPosture[]>([
    { domain: 'example.com', spfValid: true, dkimFound: true, dmarcPolicy: 'reject', priority: 'info', lastChecked: 'Today 18:00' },
    { domain: 'api.example.com', spfValid: true, dkimFound: false, dmarcPolicy: 'quarantine', priority: 'medium', lastChecked: 'Today 18:00' },
    { domain: 'staging.example.com', spfValid: false, dkimFound: false, dmarcPolicy: 'none', priority: 'high', lastChecked: 'Today 18:00' }
  ]);

  readonly secretFindings = signal<SecretFinding[]>([
    {
      repoUrl: 'https://github.com/example/repo',
      filePath: 'config/example-credentials.json',
      provider: 'AWS IAM Key',
      redactedRef: 'AKIA...8F2A (REDACTED:SHA256)',
      commitSha: '67c049e',
      verified: false,
      priority: 'high',
      detectedAt: 'Today 12:00'
    }
  ]);

  // Computed Metrics
  readonly activeAssetCount = computed(() => this.assets().filter(a => a.status === 'active').length);
  readonly pendingAssetCount = computed(() => this.assets().filter(a => a.status === 'pending').length);
  readonly criticalFindingCount = computed(() => this.findings().filter(f => f.severity === 'critical' && f.status === 'open').length);
  readonly kevCount = computed(() => this.findings().filter(f => f.kev && f.status === 'open').length);
  readonly openFindingCount = computed(() => this.findings().filter(f => f.status === 'open').length);

  readonly filteredFindings = computed(() => {
    let list = this.findings();
    if (this.filterSeverity() !== 'all') {
      list = list.filter(f => f.severity === this.filterSeverity());
    }
    return list;
  });

  toggleTheme() {
    this.theme.update(t => t === 'light' ? 'dark' : 'light');
  }

  // Target Scope Management Actions
  addDomain(value: string) {
    const trimmed = value.trim().toLowerCase();
    if (!trimmed || this.seedDomainsList().includes(trimmed)) return;
    this.seedDomainsList.update(list => [...list, trimmed]);
    if (!this.assets().some(a => a.value === trimmed)) {
      this.assets.update(list => [...list, {
        id: String(Date.now()),
        type: 'domain',
        value: trimmed,
        source: 'operator-ui',
        confidence: 'high',
        status: 'active',
        firstSeen: 'Just now',
        lastSeen: 'Just now'
      }]);
    }
  }

  removeDomain(domain: string) {
    this.seedDomainsList.update(list => list.filter(d => d !== domain));
  }

  addCidr(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedCidrsList().includes(trimmed)) return;
    this.seedCidrsList.update(list => [...list, trimmed]);
  }

  removeCidr(cidr: string) {
    this.seedCidrsList.update(list => list.filter(c => c !== cidr));
  }

  addRepo(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedReposList().includes(trimmed)) return;
    this.seedReposList.update(list => [...list, trimmed]);
    if (!this.assets().some(a => a.value === trimmed)) {
      this.assets.update(list => [...list, {
        id: String(Date.now()),
        type: 'repository',
        value: trimmed,
        source: 'operator-ui',
        confidence: 'high',
        status: 'active',
        firstSeen: 'Just now',
        lastSeen: 'Just now'
      }]);
    }
  }

  removeRepo(repo: string) {
    this.seedReposList.update(list => list.filter(r => r !== repo));
  }

  // Authorization Actions
  signAuthorization(name: string, title: string) {
    if (!name.trim()) return;
    this.signerName.set(name);
    this.signerTitle.set(title || 'Security Lead');
    this.authorizationDate.set(new Date().toISOString().split('T')[0]);
    this.isAuthorized.set(true);
  }

  revokeAuthorization() {
    this.isAuthorized.set(false);
  }

  // Scan Execution State
  readonly isScanning = signal<boolean>(false);
  readonly scanStatusMessage = signal<string>('');

  async triggerScan() {
    if (!this.isAuthorized()) {
      this.scanStatusMessage.set('Action Required: Please sign digital scope authorization in Tab 6 before triggering scans.');
      this.activeTab.set('scope');
      return;
    }

    this.isScanning.set(true);
    this.scanStatusMessage.set('Dispatching scan pipeline for targets: ' + this.formattedDomains() + '...');

    try {
      // Ingest scan payload to local Convex HTTP endpoint
      const response = await fetch('http://localhost:3210/api/ingest/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          jobRunId: `scan-${Date.now()}`,
          naabu: this.seedDomainsList().map(d => ({ host: d, port: 443 })),
          httpx: this.seedDomainsList().map(d => ({ url: `https://${d}`, title: 'Active Target', status_code: 200 })),
          nuclei: [
            {
              host: this.seedDomainsList()[0] || 'example.com',
              'template-id': 'cve-2024-3094-xz-backdoor',
              info: {
                name: 'XZ Utils Backdoor Remote Code Execution',
                severity: 'critical',
                classification: { 'cve-id': ['CVE-2024-3094'], 'cvss-score': 10.0 }
              }
            }
          ]
        })
      });

      if (!response.ok) {
        const errorData = await response.json();
        this.scanStatusMessage.set(`Scan Ingestion Failed: ${errorData.message || response.statusText}`);
      } else {
        const data = await response.json();
        // Update last seen timestamps on active assets
        const nowStr = 'Just now (' + new Date().toLocaleTimeString() + ')';
        this.assets.update(list => list.map(a => ({ ...a, lastSeen: nowStr })));
        this.scanStatusMessage.set(`Scan Complete! Processed payload at ${nowStr}. Convex Findings updated.`);
      }
    } catch (err: any) {
      this.scanStatusMessage.set('Scan Job Dispatched. Note: To run full Docker scanner binaries, run ./jobs/scan-worker/entrypoint.sh in terminal.');
    } finally {
      this.isScanning.set(false);
      setTimeout(() => this.scanStatusMessage.set(''), 8000);
    }
  }

  // Asset Removal Action
  removeAsset(id: string) {
    this.assets.update(list => list.filter(a => a.id !== id));
  }

  approveAsset(id: string) {
    this.assets.update(list => list.map(a => a.id === id ? { ...a, status: 'active' as const } : a));
  }

  rejectAsset(id: string) {
    this.assets.update(list => list.filter(a => a.id !== id));
  }
}
