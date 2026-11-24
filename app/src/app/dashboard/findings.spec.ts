import {
  ageLine,
  assetOptions,
  causeLabel,
  changedSinceCutoff,
  DEFAULT_FILTERS,
  encodeFindingKey,
  eventTypeLabel,
  explanationText,
  filterFindings,
  findingHash,
  findingKeyForFinding,
  findSummaryForSecret,
  findSummaryForSignal,
  narrativeLevelLine,
  normalizeDetail,
  orderTimeline,
  parseFindingHash,
  severityRank,
  sortFindings,
  transition,
  unsupportedClaims
} from './findings';
import { detail, event, summary } from './findings.fixtures';
import { FindingDetail } from '../models/types';

const NOW = Date.parse('2026-11-20T12:00:00Z');

describe('filterFindings', () => {
  const index = [
    summary({ key: 'a', status: 'open', severity: 'high', source: 'signal', assetId: 'x', lastChange: '2026-11-20T08:00:00Z' }),
    summary({ key: 'b', status: 'reopened', severity: 'low', source: 'finding', assetId: 'y', lastChange: '2026-11-01T08:00:00Z' }),
    summary({ key: 'c', status: 'resolved', severity: 'high', source: 'secret', assetId: 'x', lastChange: '2026-11-19T13:00:00Z' })
  ];

  it('shows active findings (open and reopened) by default', () => {
    const keys = filterFindings(index, DEFAULT_FILTERS, null).map(f => f.key);
    expect(keys).toEqual(['a', 'b']);
  });

  it('treats open, resolved and reopened as exact statuses, and all as no filter', () => {
    const only = (status: any) => filterFindings(index, { ...DEFAULT_FILTERS, status }, null).map(f => f.key);
    expect(only('open')).toEqual(['a']);
    expect(only('resolved')).toEqual(['c']);
    expect(only('reopened')).toEqual(['b']);
    expect(only('all')).toEqual(['a', 'b', 'c']);
  });

  it('combines severity, source and asset', () => {
    const f = { ...DEFAULT_FILTERS, status: 'all' as const };
    expect(filterFindings(index, { ...f, severity: 'high' }, null).map(x => x.key)).toEqual(['a', 'c']);
    expect(filterFindings(index, { ...f, source: 'finding' }, null).map(x => x.key)).toEqual(['b']);
    expect(filterFindings(index, { ...f, asset: 'x' }, null).map(x => x.key)).toEqual(['a', 'c']);
    expect(filterFindings(index, { ...f, asset: 'x', severity: 'high', source: 'secret' }, null).map(x => x.key)).toEqual(['c']);
  });

  it('keeps exactly the findings that changed since the cutoff', () => {
    const f = { ...DEFAULT_FILTERS, status: 'all' as const };
    const cutoff = changedSinceCutoff('24h', NOW, null);
    expect(filterFindings(index, f, cutoff).map(x => x.key)).toEqual(['a', 'c']);
    expect(filterFindings(index, f, changedSinceCutoff('30d', NOW, null)).map(x => x.key)).toEqual(['a', 'b', 'c']);
  });

  it('includes a change that happened exactly at the cutoff', () => {
    const exact = summary({ key: 'e', lastChange: '2026-11-19T12:00:00Z' });
    expect(filterFindings([exact], DEFAULT_FILTERS, NOW - 24 * 3_600_000).length).toBe(1);
  });

  it('drops a finding whose last change cannot be read when filtering by time', () => {
    const odd = summary({ key: 'odd', lastChange: 'not a date' });
    expect(filterFindings([odd], DEFAULT_FILTERS, 0)).toEqual([]);
  });

  it('returns nothing, not everything, when "since the last scan" cannot be evaluated', () => {
    expect(changedSinceCutoff('last-scan', NOW, null)).toBe('unavailable');
    expect(changedSinceCutoff('last-scan', NOW, 'garbage')).toBe('unavailable');
    expect(filterFindings(index, DEFAULT_FILTERS, 'unavailable')).toEqual([]);
  });

  it('uses the start of the last scan for "since the last scan"', () => {
    expect(changedSinceCutoff('last-scan', NOW, '2026-11-20T00:00:00Z')).toBe(Date.parse('2026-11-20T00:00:00Z'));
    expect(changedSinceCutoff('any', NOW, null)).toBeNull();
  });
});

describe('sortFindings', () => {
  it('orders by severity, then by most recent change, then by key', () => {
    const list = [
      summary({ key: 'low-new', severity: 'low', lastChange: '2026-11-19T00:00:00Z' }),
      summary({ key: 'crit-old', severity: 'critical', lastChange: '2026-01-01T00:00:00Z' }),
      summary({ key: 'high-old', severity: 'high', lastChange: '2026-02-01T00:00:00Z' }),
      summary({ key: 'high-new', severity: 'high', lastChange: '2026-11-01T00:00:00Z' }),
      summary({ key: 'b-tie', severity: 'info', lastChange: '2026-03-01T00:00:00Z' }),
      summary({ key: 'a-tie', severity: 'info', lastChange: '2026-03-01T00:00:00Z' })
    ];
    expect(sortFindings(list).map(f => f.key)).toEqual(['crit-old', 'high-new', 'high-old', 'low-new', 'a-tie', 'b-tie']);
  });

  it('does not mutate its input', () => {
    const list = [summary({ key: 'x', severity: 'low' }), summary({ key: 'y', severity: 'high' })];
    sortFindings(list);
    expect(list.map(f => f.key)).toEqual(['x', 'y']);
  });

  it('ranks an unknown severity below info rather than first', () => {
    expect(severityRank('critical')).toBeGreaterThan(severityRank('info'));
    expect(severityRank('info')).toBeGreaterThan(severityRank('mystery'));
  });
});

describe('assetOptions', () => {
  it('lists each asset once, labelled by value, in name order', () => {
    const options = assetOptions([
      summary({ assetId: '2', assetValue: 'b.example.com' }),
      summary({ assetId: '1', assetValue: 'a.example.com' }),
      summary({ assetId: '2', assetValue: 'b.example.com' })
    ]);
    expect(options).toEqual([
      { id: '1', label: 'a.example.com' },
      { id: '2', label: 'b.example.com' }
    ]);
  });
});

describe('timeline wording', () => {
  it('labels a library-caused change as the scanner changing, not the estate', () => {
    expect(causeLabel(event({ type: 'changed', cause: 'library' }))).toBe('scanner library changed');
    expect(causeLabel(event({ type: 'changed', cause: 'library', libraryVersion: 'v1.9.0' }))).toContain('v1.9.0');
  });

  it('labels a feed-caused change as a feed update and cites the snapshot', () => {
    const label = causeLabel(event({ type: 'priority_changed', cause: 'feed', snapshotId: 'snap-42' }));
    expect(label).toContain('feed update');
    expect(label).toContain('snap-42');
  });

  it('labels baseline and pre-history events as recorded when history began', () => {
    expect(causeLabel(event({ type: 'baseline', cause: 'baseline' }))).toBe('recorded when history began');
    // Whatever the cause says, a baseline event is not an observation.
    expect(causeLabel(event({ type: 'baseline', cause: 'estate' }))).toBe('recorded when history began');
  });

  it('labels an ordinary observation as an estate change', () => {
    expect(causeLabel(event({ type: 'resolved', cause: 'estate' }))).toBe('change in the estate');
    expect(causeLabel(event({ type: 'first_seen', cause: 'estate' }))).toBe('first observed in the estate');
  });

  it('names every event type and passes an unknown one through', () => {
    for (const t of ['baseline', 'first_seen', 'changed', 'priority_changed', 'resolved', 'reopened']) {
      expect(eventTypeLabel(t)).not.toBe(t);
    }
    expect(eventTypeLabel('novel')).toBe('novel');
  });

  it('states a transition only when something changed', () => {
    expect(transition('high', 'critical')).toBe('high to critical');
    expect(transition('high', 'high')).toBe('');
    expect(transition(undefined, undefined)).toBe('');
  });

  it('orders events by time and keeps the engine order for equal times', () => {
    const ordered = orderTimeline([
      event({ id: 'c', at: '2026-11-03T00:00:00Z' }),
      event({ id: 'a1', at: '2026-11-01T00:00:00Z' }),
      event({ id: 'a2', at: '2026-11-01T00:00:00Z' }),
      event({ id: 'b', at: '2026-11-02T00:00:00Z' })
    ]);
    expect(ordered.map(e => e.id)).toEqual(['a1', 'a2', 'b', 'c']);
  });
});

describe('age line', () => {
  it('says how long an open finding has been open', () => {
    expect(ageLine({ status: 'open', ageDays: 3 })).toBe('Open for 3 days since it was first seen');
    expect(ageLine({ status: 'reopened', ageDays: 1 })).toBe('Open for 1 day since it was first seen');
  });

  it('gives the resolution date for a resolved finding', () => {
    expect(ageLine({ status: 'resolved', ageDays: 9, resolvedAt: '2026-11-10T08:00:00Z' })).toBe(
      'Resolved on 2026-11-10, 9 days after it was first seen'
    );
  });
});

describe('narrative level line', () => {
  it('says nothing when the wording was written for the signal itself', () => {
    expect(narrativeLevelLine({ level: 'signal', levelKey: 'spf.missing', version: 'v1' })).toBe('');
  });

  it('names the fallback level, its key and the version when it was not', () => {
    const line = narrativeLevelLine({ level: 'weakness-class', levelKey: 'email-spoofing', version: 'v1' });
    expect(line).toContain('weakness class');
    expect(line).toContain('email-spoofing');
    expect(line).toContain('v1');
  });
});

describe('finding address', () => {
  it('round-trips a key through the hash', () => {
    for (const key of ['sig.example.com.spf-missing', 'a_b-c.1', 'odd key/with?chars']) {
      expect(parseFindingHash(findingHash(key))).toBe(key);
    }
  });

  it('reads no key from an address that is not a finding', () => {
    for (const hash of ['', '#', '#/', '#/assets/1', '#/findings/', '#findings/x', null, undefined]) {
      expect(parseFindingHash(hash as string)).toBeNull();
    }
  });

  it('survives a malformed escape instead of throwing', () => {
    expect(parseFindingHash('#/findings/%E0%A4%A')).toBeNull();
  });
});

describe('linking other tabs to a finding', () => {
  const index = [
    summary({ key: 'sig.example.com.spf-missing', title: 'No SPF record is published', assetId: 'a1', assetValue: 'example.com' }),
    summary({ key: 'sig.example.com.dmarc-none', title: 'DMARC policy is none', assetId: 'a1', assetValue: 'example.com' }),
    summary({ key: 'sig.other.org.spf-missing', title: 'No SPF record is published', assetId: 'a2', assetValue: 'other.org' }),
    summary({ key: 'sec-1', source: 'secret', title: 'AWS key in config.yml', assetId: 'r1', assetValue: 'https://git.example/r' }),
    summary({ key: 'sec-2', source: 'secret', title: 'Token in deploy.sh', assetId: 'r1', assetValue: 'https://git.example/r' })
  ];

  it('finds the advisory for a signal by asset and identifier in the key', () => {
    const hit = findSummaryForSignal(index, { domain: 'example.com', assetId: 'a1' }, { signalId: 'spf-missing', condition: '' });
    expect(hit?.key).toBe('sig.example.com.spf-missing');
  });

  it('matches on the source row id when the key is opaque', () => {
    // Real keys are base64, so an identifier is never legible inside one.
    const opaque = [
      summary({ key: 'signal.c2lnLWExLVNVUkYtU1BGLTAwMw', signalId: 'SURF-SPF-003', title: 'x', assetId: 'a1', assetValue: 'example.com' }),
      summary({ key: 'signal.c2lnLWExLVNVUkYtU1BGLTAwNA', signalId: 'SURF-SPF-004', title: 'x', assetId: 'a1', assetValue: 'example.com' })
    ];
    const hit = findSummaryForSignal(opaque, { domain: 'example.com', assetId: 'a1' }, { signalId: 'SURF-SPF-004', condition: 'x' });
    expect(hit?.key).toBe('signal.c2lnLWExLVNVUkYtU1BGLTAwNA');
  });

  it('matches a secret on its own id when the key is opaque', () => {
    const opaque = [
      summary({ key: 'secret.AAA', source: 'secret', secretId: 'secret:one', title: 'x', assetValue: 'https://git.example/r' }),
      summary({ key: 'secret.BBB', source: 'secret', secretId: 'secret:two', title: 'x', assetValue: 'https://git.example/r' })
    ];
    const hit = findSummaryForSecret(opaque, { _id: 'secret:two', repoUrl: 'https://git.example/r', redactedRef: '', filePath: '' });
    expect(hit?.key).toBe('secret.BBB');
  });

  it('encodes and resolves finding keys for prioritised findings', () => {
    const key = encodeFindingKey('finding', 'service:a1:host:443:tls/1:exposed');
    expect(key).toBe('finding.c2VydmljZTphMTpob3N0OjQ0Mzp0bHMvMTpleHBvc2Vk');
    expect(findingKeyForFinding([], { id: 'service:a1:host:443:tls/1:exposed' })).toBe(key);
    expect(
      findingKeyForFinding(
        [summary({ key, source: 'finding', assetId: 'a1', title: 'Exposed TLS' })],
        { id: 'service:a1:host:443:tls/1:exposed', assetId: 'a1', title: 'Exposed TLS' }
      )
    ).toBe(key);
  });

  it('falls back to the title when the identifier is not in the key', () => {
    const hit = findSummaryForSignal(
      [summary({ key: 'k1', title: 'DMARC policy is none', assetValue: 'example.com' })],
      { domain: 'example.com' },
      { signalId: 'dmarc.policy.none', condition: 'DMARC policy is none' }
    );
    expect(hit?.key).toBe('k1');
  });

  it('does not guess between two candidates', () => {
    const ambiguous = [
      summary({ key: 'k1', title: 'Same title', assetValue: 'example.com' }),
      summary({ key: 'k2', title: 'Same title', assetValue: 'example.com' })
    ];
    expect(findSummaryForSignal(ambiguous, { domain: 'example.com' }, { signalId: 'x.y', condition: 'Same title' })).toBeNull();
  });

  it('returns nothing for a signal on another domain or with no match', () => {
    expect(findSummaryForSignal(index, { domain: 'nope.test' }, { signalId: 'spf-missing', condition: '' })).toBeNull();
    expect(findSummaryForSignal(index, { domain: 'example.com' }, { signalId: 'zzz', condition: 'nothing' })).toBeNull();
    expect(findSummaryForSignal([], { domain: 'example.com' }, { signalId: 'spf-missing', condition: '' })).toBeNull();
  });

  it('finds a secret by its id, and otherwise by file path, only when unambiguous', () => {
    const base = { repoUrl: 'https://git.example/r', redactedRef: 'AKIA****', filePath: 'config.yml' };
    expect(findSummaryForSecret(index, { ...base, _id: 'sec-2' })?.key).toBe('sec-2');
    expect(findSummaryForSecret(index, { ...base, _id: 'unknown' })?.key).toBe('sec-1');
    expect(findSummaryForSecret(index, { ...base, _id: 'unknown', filePath: 'src/other.go' })).toBeNull();
    expect(findSummaryForSecret(index, { ...base, repoUrl: 'https://git.example/elsewhere' })).toBeNull();
  });
});

describe('normalizeDetail', () => {
  it('turns the null lists a Go service serialises into empty ones', () => {
    const raw = detail() as unknown as Record<string, any>;
    raw['timeline'] = null;
    raw['regressions'] = null;
    raw['explanation'].references = null;
    raw['explanation'].whatToDo = null;
    raw['provenance'].feedSnapshots = null;
    const safe = normalizeDetail(raw as unknown as FindingDetail)!;
    expect(safe.timeline).toEqual([]);
    expect(safe.regressions).toEqual([]);
    expect(safe.explanation.references).toEqual([]);
    expect(safe.explanation.whatToDo).toEqual([]);
    expect(safe.provenance.feedSnapshots).toEqual([]);
  });

  it('passes an absent detail through as absent', () => {
    expect(normalizeDetail(null)).toBeNull();
  });
});

describe('the narrative boundary', () => {
  it('finds currency amounts', () => {
    for (const text of ['It could cost $1,200 to fix', 'a loss of £5m', 'about €20k', 'USD 500', '3 million dollars']) {
      expect(unsupportedClaims(text), text).not.toEqual([]);
    }
  });

  it('finds probability-of-compromise claims', () => {
    for (const text of [
      'There is a 35% chance of compromise',
      'a 12% likelihood of attack',
      'probability of 0.4%',
      'a 1 in 5 chance',
      'p = 0.2'
    ]) {
      expect(unsupportedClaims(text), text).not.toEqual([]);
    }
  });

  it('allows prose that names what it does not claim', () => {
    const prose = 'This explanation states no expected loss and no probability of compromise. Version 2 of the record is 100 bytes.';
    expect(unsupportedClaims(prose)).toEqual([]);
  });

  it('allows the fixture explanation, including its references', () => {
    expect(unsupportedClaims(explanationText(detail().explanation))).toEqual([]);
  });

  it('would catch a claim in any section of an explanation', () => {
    const base = detail().explanation;
    const sections: (keyof typeof base)[] = [
      'whatWeFound', 'whyItMatters', 'howItCouldBeAbused', 'whatGoodLooksLike', 'howRescanConfirms', 'exploitationStatus', 'coverageNote'
    ];
    for (const section of sections) {
      const tainted = { ...base, [section]: 'Expect a $50,000 loss.' };
      expect(unsupportedClaims(explanationText(tainted)), String(section)).not.toEqual([]);
    }
    const step = { ...base, whatToDo: ['Do it now; a 40% chance of compromise remains.'] };
    expect(unsupportedClaims(explanationText(step))).not.toEqual([]);
  });
});
