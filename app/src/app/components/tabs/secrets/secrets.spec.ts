import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SecretsComponent } from './secrets';
import { WailsIpcService } from '../../../wails-ipc.service';
import { summary } from '../../../dashboard/findings.fixtures';
import { SecretFindingUI } from '../../../models/types';

describe('Secrets', () => {
  let component: SecretsComponent;
  let fixture: ComponentFixture<SecretsComponent>;
  let ipc: WailsIpcService;

  const secret = (over: Partial<SecretFindingUI> = {}): SecretFindingUI => ({
    _id: 'sec-1',
    repoUrl: 'https://git.example/r',
    filePath: 'config.yml',
    provider: 'aws',
    redactedRef: 'AKIA****',
    commitSha: 'unknown',
    verified: false,
    priority: 'high',
    status: 'open',
    firstSeen: '2026-11-01T00:00:00Z',
    lastSeen: '2026-11-02T00:00:00Z',
    detectedAt: '2026-11-01T00:00:00Z',
    ...over
  });

  beforeEach(async () => {
    window.history.replaceState(null, '', window.location.pathname);
    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({
      imports: [SecretsComponent],
    }).compileComponents();

    ipc = TestBed.inject(WailsIpcService);
    fixture = TestBed.createComponent(SecretsComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  afterEach(() => {
    window.history.replaceState(null, '', window.location.pathname);
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  describe('link to the finding detail', () => {
    const link = () =>
      Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button')).find(b =>
        b.textContent?.includes('View history and explanation')
      );

    it('is offered when the index holds the matching finding, and opens it', async () => {
      ipc.findingIndex.set([
        summary({ key: 'sec-1', source: 'secret', title: 'AWS key in config.yml', assetValue: 'https://git.example/r' })
      ]);
      component.viewSecretDetails(secret());
      fixture.detectChanges();
      await fixture.whenStable();

      expect(link()).toBeDefined();
      link()!.click();
      expect(ipc.selectedFindingKey()).toBe('sec-1');
      expect(ipc.activeTab()).toBe('findings');
    });

    it('is hidden when no finding can be matched reliably', async () => {
      ipc.findingIndex.set([
        summary({ key: 'x1', source: 'secret', title: 'Unrelated', assetValue: 'https://git.example/other' })
      ]);
      component.viewSecretDetails(secret());
      fixture.detectChanges();
      await fixture.whenStable();

      expect(link()).toBeUndefined();
    });
  });
});
