import { AssetAttribution, AttributionProvenance, DomainAssessment } from '../../../models/types';

/**
 * What the inventory says about where one asset is hosted.
 *
 * `established` is the distinction the rest of this file exists to protect. An
 * empty provider list can mean two very different things — every address was
 * checked and matched nothing, or the provider ranges never loaded — and only
 * the first is a statement about the estate. The engine already separates them
 * by degrading the network check to `check_failed`, so the view reads that
 * state rather than inferring hosting from an absence of rows.
 */
export interface HostingSummary {
  /** Whether attribution actually concluded. */
  established: boolean;
  /** Why not, when it did not. */
  reason: string;
  /** Distinct providers, in display order. */
  providers: string[];
  /** Distinct jurisdictions. Never guessed: an unknown one is simply absent. */
  jurisdictions: string[];
  /** Addresses that matched no published range. */
  unattributed: number;
  /** Total addresses considered. */
  total: number;
  /** The oldest basis any attribution rests on, or null when there is none. */
  oldestBasis: Date | null;
}

/** The empty summary, used when nothing was attributed and nothing was tried. */
export const noHosting: HostingSummary = {
  established: false,
  reason: 'Not assessed',
  providers: [],
  jurisdictions: [],
  unattributed: 0,
  total: 0,
  oldestBasis: null
};

/**
 * Builds the hosting summary for one assessment.
 *
 * Exported as a plain function so it can be tested without a component, a
 * fixture or a change-detection cycle. The decisions it makes are the ones
 * worth testing.
 */
export function summariseHosting(assessment: DomainAssessment | undefined): HostingSummary {
  if (!assessment) {
    return noHosting;
  }

  const net = netCheck(assessment);
  const rows = assessment.attribution ?? [];

  // A failed network check means the addresses could not be placed, so the
  // rows that did arrive are an incomplete picture rather than a short one.
  // Reporting them as the hosting would understate the estate in the
  // reassuring direction.
  if (net && (net.state === 'check_failed' || net.state === 'not_checked')) {
    return {
      ...noHosting,
      reason: net.reason || (net.state === 'not_checked' ? 'Not assessed' : 'Could not be established'),
      total: rows.length
    };
  }

  if (rows.length === 0) {
    // No rows and no failed check: nothing resolved. That is a real result,
    // but it is not a claim that the asset is hosted nowhere.
    return { ...noHosting, reason: net ? 'No addresses resolved' : 'Not assessed' };
  }

  const providers = distinct(rows.map(r => r.provider ?? '').filter(p => p !== ''));
  const jurisdictions = distinct(rows.map(r => r.jurisdiction ?? '').filter(j => j !== ''));

  return {
    established: true,
    reason: '',
    providers,
    jurisdictions,
    unattributed: rows.filter(r => !r.provider).length,
    total: rows.length,
    oldestBasis: oldestBasis(assessment.attributionProvenance ?? [])
  };
}

/**
 * The age of the weakest basis the attribution rests on.
 *
 * Oldest rather than newest, deliberately. A single freshly fetched source
 * cannot vouch for one that has not been refreshed in a week, and reporting
 * the newest would let one good source conceal every stale one behind it.
 *
 * Returns null when there is no provenance at all, which is not the same as
 * "fetched just now" — zero would be the strongest freshness claim available,
 * asserted on no evidence.
 */
function oldestBasis(provenance: AttributionProvenance[]): Date | null {
  let oldest: Date | null = null;
  for (const p of provenance) {
    const at = new Date(p.fetchedAt);
    if (Number.isNaN(at.getTime())) {
      continue;
    }
    if (oldest === null || at < oldest) {
      oldest = at;
    }
  }
  return oldest;
}

/** The network check's coverage, wherever it sits in the control tree. */
function netCheck(assessment: DomainAssessment) {
  for (const control of assessment.controls ?? []) {
    for (const check of control.checks ?? []) {
      if (check.checkId === 'net') {
        return check;
      }
    }
  }
  return undefined;
}

function distinct(values: string[]): string[] {
  return [...new Set(values)].sort();
}

/** Groups attribution rows by the name that resolved, for the detail view. */
export function byHost(rows: AssetAttribution[]): { host: string; rows: AssetAttribution[] }[] {
  const groups = new Map<string, AssetAttribution[]>();
  for (const r of rows) {
    groups.set(r.host, [...(groups.get(r.host) ?? []), r]);
  }
  return [...groups.entries()]
    .map(([host, rows]) => ({ host, rows }))
    .sort((a, b) => a.host.localeCompare(b.host));
}
