import { ComponentFixture, TestBed } from '@angular/core/testing';

import { OverviewComponent, matchesServiceFilter, serviceEvidenceHeadline, summariseServiceEvidence } from './overview';
import { ServiceObservation } from '../../../models/types';

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
});
