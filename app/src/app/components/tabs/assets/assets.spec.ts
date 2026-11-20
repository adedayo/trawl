import { ComponentFixture, TestBed } from '@angular/core/testing';

import { AssetsComponent } from './assets';
import { WailsIpcService } from '../../../wails-ipc.service';

describe('Assets', () => {
  let component: AssetsComponent;
  let fixture: ComponentFixture<AssetsComponent>;
  let ipc: WailsIpcService;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AssetsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(AssetsComponent);
    component = fixture.componentInstance;
    ipc = TestBed.inject(WailsIpcService);
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('shows identified, version unknown, unmapped, and not assessed software states distinctly', async () => {
    ipc.assets.set([
      { id: 'a-1', value: 'app.example.com', type: 'subdomain', source: 'httpx', status: 'active', firstSeen: '2026-11-20', lastSeen: '2026-11-20' },
      { id: 'a-2', value: 'unscanned.example.com', type: 'subdomain', source: 'subfinder', status: 'active', firstSeen: '2026-11-20', lastSeen: '2026-11-20' }
    ]);
    ipc.softwareCoverage.set([
      { assetId: 'a-1', state: 'identified', identified: 1, versionUnknown: 1, unmapped: 1, superseded: 1 },
      { assetId: 'a-2', state: 'not_assessed', identified: 0, versionUnknown: 0, unmapped: 0, superseded: 0 }
    ]);
    ipc.assetSoftware.set([
      {
        id: 'sw-1', assetId: 'a-1', observedName: 'Nginx', cpePart: 'a', vendor: 'f5', product: 'nginx',
        mappingState: 'mapped', mappingVersion: '1.0.0', version: '1.24.0', versionState: 'known',
        source: 'httpx', rawString: 'Nginx:1.24.0', status: 'current', firstSeen: '2026-11-20', lastSeen: '2026-11-20'
      },
      {
        id: 'sw-2', assetId: 'a-1', observedName: 'WordPress', cpePart: 'a', vendor: 'wordpress', product: 'wordpress',
        mappingState: 'mapped', mappingVersion: '1.0.0', version: '', versionState: 'unknown',
        source: 'httpx', rawString: 'WordPress', status: 'current', firstSeen: '2026-11-20', lastSeen: '2026-11-20'
      },
      {
        id: 'sw-3', assetId: 'a-1', observedName: 'Cloudflare',
        mappingState: 'unmapped', version: '', versionState: 'unknown',
        source: 'httpx', rawString: 'Cloudflare', status: 'current', firstSeen: '2026-11-20', lastSeen: '2026-11-20'
      },
      {
        id: 'sw-4', assetId: 'a-1', observedName: 'Nginx', cpePart: 'a', vendor: 'f5', product: 'nginx',
        mappingState: 'mapped', mappingVersion: '1.0.0', version: '1.18.0', versionState: 'known',
        source: 'httpx', rawString: 'Nginx:1.18.0', status: 'superseded', firstSeen: '2026-11-19', lastSeen: '2026-11-19'
      }
    ]);

    fixture.detectChanges();
    await fixture.whenStable();

    const root = fixture.nativeElement as HTMLElement;
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('[data-testid="toggle-software-btn"]'));
    expect(buttons).toHaveLength(2);
    expect(buttons[0].textContent).toContain('1 identified');
    expect(buttons[1].textContent).toContain('Not assessed');

    buttons[0].click();
    fixture.detectChanges();
    await fixture.whenStable();

    const panel1 = root.querySelector('[data-testid="asset-software-panel"]')?.textContent ?? '';
    expect(panel1).toContain('Nginx');
    expect(panel1).toContain('1.24.0');
    expect(panel1).toContain('f5:nginx');
    expect(panel1).toContain('version unknown');
    expect(panel1).toContain('unmapped');
    expect(panel1).toContain('superseded');

    buttons[1].click();
    fixture.detectChanges();
    await fixture.whenStable();

    const panel2 = root.querySelector('[data-testid="asset-software-panel"]')?.textContent ?? '';
    expect(panel2).toContain('Not assessed for software');
    expect(panel2).toContain('not evidence of zero software vulnerabilities');
  });
});
