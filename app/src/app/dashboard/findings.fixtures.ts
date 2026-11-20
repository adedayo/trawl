import {
  ChangeSet,
  FindingChange,
  FindingDetail,
  FindingEventView,
  FindingSummary,
  HistoryPoint,
  ScanRunView
} from '../models/types';

/**
 * Hand-built fixtures for the Change 025 specs.
 *
 * Not a spec file, so the runner does not collect it, and not imported by the
 * application, so it is not bundled. Each factory takes overrides so the field
 * an assertion depends on is written at the assertion, not buried here.
 */

export function summary(over: Partial<FindingSummary> = {}): FindingSummary {
  return {
    key: 'sig.example.com.spf-missing',
    source: 'signal',
    assetId: 'asset-1',
    assetValue: 'example.com',
    title: 'No SPF record is published',
    severity: 'medium',
    priority: 'medium',
    status: 'open',
    notReassessed: false,
    category: 'spf',
    kev: false,
    firstSeen: '2026-11-01T10:00:00Z',
    lastSeen: '2026-11-18T10:00:00Z',
    ageDays: 17,
    lastChange: '2026-11-10T10:00:00Z',
    lastChangeType: 'first_seen',
    ...over
  };
}

export function event(over: Partial<FindingEventView> = {}): FindingEventView {
  return {
    id: 'ev-1',
    type: 'first_seen',
    at: '2026-11-01T10:00:00Z',
    runId: 'run-1',
    cause: 'estate',
    ...over
  };
}

export function run(over: Partial<ScanRunView> = {}): ScanRunView {
  return {
    id: 'run-1',
    kind: 'assessment',
    outcome: 'completed',
    startedAt: '2026-11-01T09:55:00Z',
    finishedAt: '2026-11-01T10:00:00Z',
    coverage: { total: 10, ok: 8, notFound: 0, notChecked: 2, checkFailed: 0, assessedOnly: 8 },
    coverageFraction: 0.8,
    ...over
  };
}

export function point(over: Partial<HistoryPoint> & { runId?: string; coverageFraction?: number } = {}): HistoryPoint {
  const { runId, coverageFraction, ...rest } = over;
  return {
    run: run({ id: runId ?? 'run-1', coverageFraction: coverageFraction ?? 0.8 }),
    at: '2026-11-01T10:00:00Z',
    critical: 0,
    high: 1,
    medium: 2,
    low: 3,
    info: 0,
    open: 6,
    resolved: 0,
    new: 0,
    resolvedInRun: 0,
    ...rest
  };
}

export function change(over: Partial<FindingChange> = {}): FindingChange {
  return { finding: summary(), event: event(), ...over };
}

export function changeSet(over: Partial<ChangeSet> = {}): ChangeSet {
  return { new: [], resolved: [], worsened: [], improved: [], ...over };
}

/** A fully populated detail, with a signal-level explanation. */
export function detail(over: Partial<FindingDetail> = {}): FindingDetail {
  return {
    ...summary(),
    description: 'The domain publishes no Sender Policy Framework record.',
    detail: 'No TXT record beginning v=spf1 was found.',
    evidence: 'dig TXT example.com\n; no v=spf1 record',
    checkId: 'spf',
    checkState: 'not_found',
    checkReason: '',
    explanation: {
      level: 'signal',
      levelKey: 'spf.missing',
      version: 'narratives-2026.11',
      whatWeFound: 'The domain example.com does not publish an SPF record.',
      whyItMatters: 'Receiving mail systems cannot tell which senders are authorized for the domain.',
      howItCouldBeAbused: 'Someone could send messages that appear to come from the organization.',
      whatGoodLooksLike: 'A single SPF record that lists the authorized senders and ends in a restrictive policy.',
      whatToDo: ['Inventory the services that send mail for the domain.', 'Publish an SPF record that lists them.'],
      howRescanConfirms: 'The next scan looks up the record again and the finding resolves when one is published.',
      references: ['https://www.rfc-editor.org/rfc/rfc7208'],
      standards: ['NIST SP 800-177']
    },
    narrativeVersion: 'narratives-2026.11',
    timeline: [
      event({ id: 'ev-1', type: 'first_seen', at: '2026-11-01T10:00:00Z', runId: 'run-1', severity: 'medium' })
    ],
    regressions: [],
    provenance: { runId: 'run-1', libraryVersion: 'vantage v1.8.0', registryVersion: 'reg-12', feedSnapshots: [] },
    ...over
  };
}
