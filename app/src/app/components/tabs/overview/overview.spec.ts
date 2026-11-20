import { ComponentFixture, TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { OverviewComponent, matchesServiceFilter, serviceEvidenceHeadline, summariseServiceEvidence } from './overview';
import { ServiceObservation } from '../../../models/types';
import { WailsIpcService } from '../../../wails-ipc.service';

const httpsObservation: ServiceObservation = {
  assetId: 'asset-1', host: 'api.example.com', port: 443, service: 'https',
  transport: 'tcp', protocol: 'https', layer: 'http', state: 'responding',
  coverage: 'ok', profile: 'most-common', observedAt: '2026-09-21T12:00:00Z',
  evidence: JSON.stringify({ http_status: 200, tls_version: 'TLS 1.3', hsts: '' })
};

describe('Overview', () => {
  let component: OverviewComponent;
  let fixture: ComponentFixture<OverviewComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [OverviewComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(OverviewComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('turns HTTPS evidence into facts, issues, and an action', () => {
    const summary = summariseServiceEvidence(httpsObservation);
    expect(summary.facts).toContain('HTTP 200');
    expect(summary.facts).toContain('Negotiated TLS 1.3');
    expect(summary.issues).toContain('HSTS is not advertised.');
    expect(summary.recommendation).toContain('Strict-Transport-Security');
  });

  it('handles malformed evidence without exposing a JSON dump', () => {
    const summary = summariseServiceEvidence({ ...httpsObservation, evidence: '{broken' });
    expect(summary.issues[0]).toContain('could not be parsed');
    expect(summary.recommendation.length).toBeGreaterThan(0);
  });

  it('filters by host, service, port, and advisory text', () => {
    expect(matchesServiceFilter(httpsObservation, 'api.example.com')).toBe(true);
    expect(matchesServiceFilter(httpsObservation, '443')).toBe(true);
    expect(matchesServiceFilter(httpsObservation, 'hsts')).toBe(true);
    expect(matchesServiceFilter(httpsObservation, 'redis')).toBe(false);
  });

  it('builds a collapsed headline with fact and issue counts', () => {
    const headline = serviceEvidenceHeadline(httpsObservation);
    expect(headline.headline).toBe('HSTS is not advertised.');
    expect(headline.issueCount).toBe(1);
    expect(headline.goodCount).toBe(3);
  });

  it('retains findings from earlier hosts when a subsequent host probe returns zero findings', async () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.assets.set([
      { id: 'asset-a', value: 'exposed.example.com', status: 'active' },
      { id: 'asset-b', value: 'quiet.example.com', status: 'active' }
    ]);

    vi.spyOn(ipc.transport, 'getFindings').mockImplementation(async (assetId: string) => {
      if (assetId === 'asset-a') {
        return [
          {
            id: 'service:asset-a:exposed.example.com:443:http:missing-hsts',
            assetId: 'asset-a',
            title: 'HTTPS service does not advertise HSTS',
            severity: 'low',
            status: 'open'
          }
        ];
      }
      if (assetId === 'asset-b') {
        return [];
      }
      return [];
    });

    await ipc.refreshFindings('asset-a');
    await ipc.refreshFindings('asset-b');

    fixture.detectChanges();
    await fixture.whenStable();

    expect(ipc.findings().length).toBe(1);
    expect(ipc.findings()[0].assetValue).toBe('exposed.example.com');
    expect(component.priorityFindings().length).toBe(1);

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).toContain('HTTPS service does not advertise HSTS');
    expect(text).toContain('exposed.example.com');
    expect(text).toContain('LOW');
  });

  it('omits resolved findings from Priority External Exposure Findings and orders open ones deterministically', () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.findings.set([
      { id: 'f-low', title: 'Low open', severity: 'low', status: 'open', kev: false, epssScore: 0 },
      { id: 'f-resolved', title: 'Closed finding', severity: 'critical', status: 'resolved', kev: true, epssScore: 0.9 },
      { id: 'f-high', title: 'High open', severity: 'high', status: 'open', kev: false, epssScore: 0 }
    ]);

    expect(component.priorityFindings().map(f => f.id)).toEqual(['f-high', 'f-low']);
  });
});
