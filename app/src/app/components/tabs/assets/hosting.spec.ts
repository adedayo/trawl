import { describe, expect, it } from 'vitest';
import { byHost, summariseHosting } from './hosting';
import { AssetAttribution, DomainAssessment } from '../../../models/types';

function attribution(over: Partial<AssetAttribution> = {}): AssetAttribution {
  return {
    assetId: 'asset-1',
    host: 'www.example.com',
    role: 'host',
    address: '203.0.113.10',
    provider: 'aws',
    region: 'eu-west-2',
    jurisdiction: 'GB',
    observedAt: '2026-09-19T12:00:00Z',
    ...over
  };
}

function assessment(over: Partial<DomainAssessment> = {}): DomainAssessment {
  return {
    assetId: 'asset-1',
    domain: 'example.com',
    outcome: 'completed',
    coverage: { total: 1, ok: 1, notFound: 0, notChecked: 0, checkFailed: 0, assessedOnly: 1 },
    coverageFraction: 1,
    controls: [{
      control: 'network',
      posture: 'compliant',
      coverage: { total: 1, ok: 1, notFound: 0, notChecked: 0, checkFailed: 0, assessedOnly: 1 },
      checks: [{ checkId: 'net', state: 'ok' }],
      signals: []
    }],
    scenarios: [],
    unmapped: [],
    attribution: [attribution()],
    attributionProvenance: [
      { assetId: 'asset-1', provider: 'aws', url: 'https://aws.invalid/ranges', fetchedAt: '2026-09-18T06:00:00Z' }
    ],
    registryVersion: '1',
    libraryVersion: '1.4.0',
    ...over
  } as DomainAssessment;
}

describe('hosting summary', () => {
  it('reports the providers and jurisdictions it established', () => {
    const summary = summariseHosting(assessment());

    expect(summary.established).toBe(true);
    expect(summary.providers).toEqual(['aws']);
    expect(summary.jurisdictions).toEqual(['GB']);
  });

  // The whole reason this module exists. A failed network check means the
  // addresses could not be placed, so the rows that did arrive are an
  // incomplete picture. Rendering them as the hosting would understate the
  // estate in the reassuring direction.
  it('refuses to report hosting when the check could not tell', () => {
    const summary = summariseHosting(assessment({
      controls: [{
        control: 'network',
        posture: 'unknown',
        coverage: { total: 1, ok: 0, notFound: 0, notChecked: 0, checkFailed: 1, assessedOnly: 0 },
        checks: [{ checkId: 'net', state: 'check_failed', reason: 'provider ranges unavailable for azure' }],
        signals: []
      }]
    } as Partial<DomainAssessment>));

    expect(summary.established).toBe(false);
    expect(summary.providers).toEqual([]);
    expect(summary.reason).toContain('azure');
  });

  // "We never looked" and "we looked and found nothing" are different claims,
  // and the second is the only one that says anything about the estate.
  it('separates a check that never ran from one that resolved nothing', () => {
    const neverRan = summariseHosting(assessment({
      controls: [{
        control: 'network',
        posture: 'unknown',
        coverage: { total: 1, ok: 0, notFound: 0, notChecked: 1, checkFailed: 0, assessedOnly: 0 },
        checks: [{ checkId: 'net', state: 'not_checked', reason: 'excluded by egress policy' }],
        signals: []
      }],
      attribution: []
    } as Partial<DomainAssessment>));

    const resolvedNothing = summariseHosting(assessment({ attribution: [] }));

    expect(neverRan.established).toBe(false);
    expect(resolvedNothing.established).toBe(false);
    expect(neverRan.reason).not.toBe(resolvedNothing.reason);
  });

  it('counts addresses that matched no published range', () => {
    const summary = summariseHosting(assessment({
      attribution: [
        attribution(),
        attribution({ address: '192.0.2.5', provider: '', region: '', jurisdiction: '' })
      ]
    } as Partial<DomainAssessment>));

    expect(summary.total).toBe(2);
    expect(summary.unattributed).toBe(1);
    expect(summary.providers).toEqual(['aws']);
  });

  // A single freshly fetched source must not vouch for one that has not been
  // refreshed in days, so the weakest basis is the one reported.
  it('reports the oldest basis, not the newest', () => {
    const summary = summariseHosting(assessment({
      attributionProvenance: [
        { assetId: 'asset-1', provider: 'gcp', url: 'https://gcp.invalid/r', fetchedAt: '2026-09-19T11:00:00Z' },
        { assetId: 'asset-1', provider: 'aws', url: 'https://aws.invalid/r', fetchedAt: '2026-09-16T06:00:00Z' }
      ]
    } as Partial<DomainAssessment>));

    expect(summary.oldestBasis?.toISOString()).toBe('2026-09-16T06:00:00.000Z');
  });

  // Zero would be the strongest freshness claim available, asserted on no
  // evidence at all.
  it('reports a missing basis as unknown rather than as just fetched', () => {
    const summary = summariseHosting(assessment({ attributionProvenance: [] } as Partial<DomainAssessment>));

    expect(summary.oldestBasis).toBeNull();
  });

  it('treats a missing assessment as nothing assessed', () => {
    expect(summariseHosting(undefined).established).toBe(false);
  });
});

describe('byHost', () => {
  // A name balanced across two providers is a fact to show, not a
  // disagreement to resolve by picking whichever address came back first.
  it('keeps every address of a name together', () => {
    const groups = byHost([
      attribution({ address: '203.0.113.10', provider: 'aws' }),
      attribution({ address: '198.51.100.7', provider: 'gcp' }),
      attribution({ host: 'mail.example.com', address: '203.0.113.20', role: 'mail exchanger' })
    ]);

    expect(groups).toHaveLength(2);
    expect(groups[0].host).toBe('mail.example.com');
    expect(groups[1].rows).toHaveLength(2);
  });
});
