import {
  CoverageState,
  Explanation,
  FindingDetail,
  FindingEventView,
  FindingSeverity,
  FindingSource,
  FindingSummary,
  SignalView,
  SecretFindingUI
} from '../models/types';

/**
 * Derivation for the findings list and drill-down (Change 025).
 *
 * Everything here is a pure function, for the same reason `estate.ts` is: the
 * list order, the filters and the wording of a timeline are properties of the
 * data, and asserting them through rendered markup would tie them to layout
 * decisions that are expected to change.
 *
 * Nothing here computes a severity, a priority or a status. Those arrive from
 * the engine and are rendered as given.
 */

/**
 * Makes a detail safe to render: the engine serialises an empty Go slice as
 * `null`, and a template that reads `.length` of it would throw. Nothing is
 * invented; absent lists become empty ones.
 */
export function normalizeDetail(detail: FindingDetail | null): FindingDetail | null {
  if (!detail) return null;
  const explanation = detail.explanation ?? ({} as Explanation);
  return {
    ...detail,
    explanation: {
      ...explanation,
      whatToDo: explanation.whatToDo ?? [],
      references: explanation.references ?? [],
      standards: explanation.standards ?? []
    },
    timeline: detail.timeline ?? [],
    regressions: detail.regressions ?? [],
    provenance: { ...(detail.provenance ?? {}), feedSnapshots: detail.provenance?.feedSnapshots ?? [] }
  };
}

// ---------------------------------------------------------------------------
// Filtering and sorting
// ---------------------------------------------------------------------------

/** The status filter. `active` is open plus reopened, and is the default. */
export type StatusFilter = 'active' | 'open' | 'resolved' | 'reopened' | 'all';

/** How far back the "changed since" control looks. */
export type ChangedSinceOption = 'any' | 'last-scan' | '24h' | '7d' | '30d';

export interface FindingListFilters {
  status: StatusFilter;
  severity: FindingSeverity | 'all';
  source: FindingSource | 'all';
  /** An asset id, or `all`. */
  asset: string;
  changedSince: ChangedSinceOption;
}

export const DEFAULT_FILTERS: FindingListFilters = {
  status: 'active',
  severity: 'all',
  source: 'all',
  asset: 'all',
  changedSince: 'any'
};

export const SEVERITY_ORDER: readonly FindingSeverity[] = ['critical', 'high', 'medium', 'low', 'info'];

/** Higher is more severe. Unknown values sort below `info` rather than first. */
export function severityRank(severity: string): number {
  const index = SEVERITY_ORDER.indexOf(severity as FindingSeverity);
  return index === -1 ? 0 : SEVERITY_ORDER.length - index;
}

const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;

/**
 * The earliest event time, in epoch milliseconds, that the control admits, or
 * null when it does not filter.
 *
 * `last-scan` needs the start of the newest run. When no run is known the
 * answer is `unavailable` rather than null: silently listing everything under
 * a label that promises "changed since the last scan" would be a false claim.
 */
export function changedSinceCutoff(
  option: ChangedSinceOption,
  nowMs: number,
  lastScanStartedAt: string | null
): number | null | 'unavailable' {
  switch (option) {
    case 'any':
      return null;
    case '24h':
      return nowMs - DAY_MS;
    case '7d':
      return nowMs - 7 * DAY_MS;
    case '30d':
      return nowMs - 30 * DAY_MS;
    case 'last-scan': {
      const parsed = lastScanStartedAt ? Date.parse(lastScanStartedAt) : NaN;
      return Number.isNaN(parsed) ? 'unavailable' : parsed;
    }
  }
}

function statusMatches(filter: StatusFilter, status: string): boolean {
  switch (filter) {
    case 'all':
      return true;
    case 'active':
      return status === 'open' || status === 'reopened';
    default:
      return status === filter;
  }
}

/**
 * Applies the list filters. `changedCutoff` comes from `changedSinceCutoff`;
 * an `unavailable` cutoff yields an empty list, never an unfiltered one.
 *
 * A finding "changed since" a time when its most recent event is at or after
 * it, which is the same as having any event at or after it.
 */
export function filterFindings(
  index: readonly FindingSummary[],
  filters: FindingListFilters,
  changedCutoff: number | null | 'unavailable'
): FindingSummary[] {
  if (changedCutoff === 'unavailable') {
    return [];
  }
  return index.filter(f => {
    if (!statusMatches(filters.status, f.status)) return false;
    if (filters.severity !== 'all' && f.severity !== filters.severity) return false;
    if (filters.source !== 'all' && f.source !== filters.source) return false;
    if (filters.asset !== 'all' && f.assetId !== filters.asset) return false;
    if (changedCutoff !== null) {
      const changed = Date.parse(f.lastChange);
      if (Number.isNaN(changed) || changed < changedCutoff) return false;
    }
    return true;
  });
}

/**
 * Orders by severity, most severe first, then by most recent change, then by
 * key so that two readers always see the same order.
 */
export function sortFindings(list: readonly FindingSummary[]): FindingSummary[] {
  return [...list].sort((a, b) => {
    if (a.source === 'candidate' && b.source === 'candidate' && a.kev !== b.kev) {
      return a.kev ? -1 : 1;
    }
    const byPriority = severityRank(b.priority || b.severity) - severityRank(a.priority || a.severity);
    if (byPriority !== 0) return byPriority;
    const bySeverity = severityRank(b.severity) - severityRank(a.severity);
    if (bySeverity !== 0) return bySeverity;
    const byRecency = timeOf(b.lastChange) - timeOf(a.lastChange);
    if (byRecency !== 0) return byRecency;
    return a.key.localeCompare(b.key);
  });
}

function timeOf(iso: string): number {
  const t = Date.parse(iso);
  return Number.isNaN(t) ? 0 : t;
}

/** Distinct assets present in the index, for the asset filter. */
export function assetOptions(index: readonly FindingSummary[]): { id: string; label: string }[] {
  const seen = new Map<string, string>();
  for (const f of index) {
    if (f.assetId && !seen.has(f.assetId)) {
      seen.set(f.assetId, f.assetValue || f.assetId);
    }
  }
  return [...seen.entries()]
    .map(([id, label]) => ({ id, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
}

// ---------------------------------------------------------------------------
// Labels
// ---------------------------------------------------------------------------

export function sourceLabel(source: string): string {
  switch (source) {
    case 'signal':
      return 'Signal';
    case 'finding':
      return 'Scan finding';
    case 'secret':
      return 'Secret';
    case 'candidate':
      return 'Software candidate (unconfirmed)';
    default:
      return source || 'Unknown source';
  }
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'open':
      return 'Open';
    case 'resolved':
      return 'Resolved';
    case 'reopened':
      return 'Reopened';
    default:
      return status || 'Unknown';
  }
}

const EVENT_TYPE_LABELS: Record<string, string> = {
  baseline: 'Recorded at the start of history',
  first_seen: 'First seen',
  changed: 'Changed',
  priority_changed: 'Priority changed',
  resolved: 'Resolved',
  reopened: 'Reopened'
};

export function eventTypeLabel(type: string): string {
  return EVENT_TYPE_LABELS[type] ?? type;
}

/**
 * Why an event happened, in words an operator can act on.
 *
 * The distinction matters: a library change and a feed update are not changes
 * in the estate, and labelling them as such would send someone looking for a
 * deployment that never happened.
 */
export function causeLabel(event: Pick<FindingEventView, 'type' | 'cause' | 'snapshotId' | 'libraryVersion'>): string {
  if (event.type === 'baseline' || event.cause === 'baseline') {
    return 'recorded when history began';
  }
  switch (event.cause) {
    case 'library':
      return event.libraryVersion
        ? `scanner library changed (${event.libraryVersion})`
        : 'scanner library changed';
    case 'feed':
      return event.snapshotId ? `feed update (snapshot ${event.snapshotId})` : 'feed update';
    case 'estate':
      return event.type === 'first_seen' ? 'first observed in the estate' : 'change in the estate';
    default:
      return event.cause || 'cause not recorded';
  }
}

/** True for events that are not a change in the estate itself. */
export function isNotEstateChange(event: Pick<FindingEventView, 'type' | 'cause'>): boolean {
  return event.type === 'baseline' || event.cause === 'library' || event.cause === 'feed' || event.cause === 'baseline';
}

/** `prev to new` for a field that changed, or '' when nothing is recorded. */
export function transition(prev?: string, next?: string): string {
  if (prev && next && prev !== next) return `${prev} to ${next}`;
  if (!prev && next) return next;
  return '';
}

/** Events in time order. Stable, so equal times keep the engine's order. */
export function orderTimeline(events: readonly FindingEventView[]): FindingEventView[] {
  return events
    .map((event, index) => ({ event, index }))
    .sort((a, b) => timeOf(a.event.at) - timeOf(b.event.at) || a.index - b.index)
    .map(x => x.event);
}

const COVERAGE_STATE_LABELS: Record<string, string> = {
  ok: 'concluded, present',
  not_found: 'concluded, nothing found',
  not_checked: 'not checked',
  check_failed: 'check failed'
};

export function checkStateLabel(state: CoverageState | string | undefined): string {
  return (state && COVERAGE_STATE_LABELS[state]) || '';
}

/** Which narrative level supplied the wording, as a sentence, or '' when it was the finding's own. */
export function narrativeLevelLine(explanation: Pick<Explanation, 'level' | 'levelKey' | 'version'>): string {
  if (!explanation || explanation.level === 'signal') {
    return '';
  }
  const named: Record<string, string> = {
    'weakness-class': 'the weakness class',
    check: 'the check family',
    'service-class': 'the service class',
    'secret-type': 'the secret type',
    cve: 'the CVE template',
    generic: 'the general fallback'
  };
  const level = named[explanation.level] ?? explanation.level;
  const key = explanation.levelKey ? ` "${explanation.levelKey}"` : '';
  const version = explanation.version ? `, narrative version ${explanation.version}` : '';
  return `This wording comes from ${level}${key}, not from text written for this specific finding${version}.`;
}

// ---------------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------------

/** `2026-11-20 12:30 UTC`, deterministic regardless of the viewer's locale. */
export function formatTime(iso: string | undefined | null): string {
  if (!iso) return 'not recorded';
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  return `${new Date(t).toISOString().slice(0, 16).replace('T', ' ')} UTC`;
}

export function formatDay(iso: string | undefined | null): string {
  if (!iso) return 'not recorded';
  const t = Date.parse(iso);
  return Number.isNaN(t) ? iso : new Date(t).toISOString().slice(0, 10);
}

export function plural(n: number, singular: string, pluralForm = `${singular}s`): string {
  return `${n} ${n === 1 ? singular : pluralForm}`;
}

/** The "open for N days" / "resolved on" line of a header. */
export function ageLine(f: Pick<FindingSummary, 'status' | 'ageDays' | 'resolvedAt'>): string {
  if (f.status === 'resolved') {
    return f.resolvedAt
      ? `Resolved on ${formatDay(f.resolvedAt)}, ${plural(f.ageDays, 'day')} after it was first seen`
      : `Resolved, ${plural(f.ageDays, 'day')} after it was first seen`;
  }
  return `Open for ${plural(f.ageDays, 'day')} since it was first seen`;
}

// ---------------------------------------------------------------------------
// Deep link address
// ---------------------------------------------------------------------------

const HASH_PREFIX = '#/findings/';

/** The address of a finding's detail. */
export function findingHash(key: string): string {
  return `${HASH_PREFIX}${encodeURIComponent(key)}`;
}

/**
 * The finding a location hash names, or null when it names none.
 *
 * A malformed escape yields null rather than throwing: an address typed or
 * mangled by hand must leave the application usable.
 */
export function parseFindingHash(hash: string | null | undefined): string | null {
  if (!hash || !hash.startsWith(HASH_PREFIX)) return null;
  const raw = hash.slice(HASH_PREFIX.length).split(/[?/]/)[0];
  if (!raw) return null;
  try {
    return decodeURIComponent(raw) || null;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Linking other tabs to the same detail
// ---------------------------------------------------------------------------

const norm = (s: string | undefined | null) => (s ?? '').trim().toLowerCase();

/**
 * Finds the index entry for an advisory shown on the Email tab.
 *
 * The index is the source of truth for keys, and the wire contract does not
 * carry the signal identifier on a summary, so the match is made on source,
 * asset and then on what the summary does expose: the identifier appearing in
 * the key, or the title equalling the advisory's condition. A match is only
 * returned when it is unambiguous. When nothing reliable matches the caller
 * hides the link, because a link to the wrong finding is worse than none.
 */
export function findSummaryForSignal(
  index: readonly FindingSummary[],
  asset: { domain: string; assetId?: string },
  signal: Pick<SignalView, 'signalId' | 'condition'>
): FindingSummary | null {
  const domain = norm(asset.domain);
  const candidates = index.filter(
    f =>
      f.source === 'signal' &&
      ((asset.assetId && f.assetId === asset.assetId) || norm(f.assetValue) === domain)
  );
  const id = norm(signal.signalId);
  if (!id || candidates.length === 0) return null;

  // The source row's own identifier is exact. Everything below is a fallback
  // for an index that does not carry it.
  const exact = candidates.filter(f => norm(f.signalId) === id);
  if (exact.length === 1) return exact[0];

  const byKey = candidates.filter(f => norm(f.key).includes(id));
  if (byKey.length === 1) return byKey[0];

  const pool = byKey.length > 1 ? byKey : candidates;
  const condition = norm(signal.condition);
  const byTitle = pool.filter(f => (condition && norm(f.title) === condition) || norm(f.title).includes(id));
  return byTitle.length === 1 ? byTitle[0] : null;
}

/**
 * Finds the index entry for a secret shown on the Secrets tab, or null.
 *
 * Same rule as for signals: unambiguous or nothing. The secret's own id in the
 * key is the strongest evidence; failing that the redacted reference or the
 * file path in the title.
 */
export function findSummaryForSecret(
  index: readonly FindingSummary[],
  secret: Pick<SecretFindingUI, '_id' | 'repoUrl' | 'redactedRef' | 'filePath'>
): FindingSummary | null {
  const repo = norm(secret.repoUrl);
  const candidates = index.filter(
    f => f.source === 'secret' && (!repo || norm(f.assetValue) === repo || norm(f.assetId) === repo)
  );
  if (candidates.length === 0) return null;

  const id = norm(secret._id);
  if (id) {
    const exact = candidates.filter(f => norm(f.secretId) === id);
    if (exact.length === 1) return exact[0];
    const byId = candidates.filter(f => norm(f.key).includes(id));
    if (byId.length === 1) return byId[0];
  }
  const ref = norm(secret.redactedRef);
  if (ref) {
    const byRef = candidates.filter(f => norm(f.title).includes(ref) || norm(f.key).includes(ref));
    if (byRef.length === 1) return byRef[0];
  }
  const path = norm(secret.filePath);
  if (path) {
    const byPath = candidates.filter(f => norm(f.title).includes(path));
    if (byPath.length === 1) return byPath[0];
  }
  return null;
}

// ---------------------------------------------------------------------------
// The narrative boundary
// ---------------------------------------------------------------------------

/** Every string the explanation renders, joined, for boundary checks. */
export function explanationText(e: Explanation): string {
  return [
    e.whatWeFound,
    e.whyItMatters,
    e.howItCouldBeAbused,
    e.whatGoodLooksLike,
    ...(e.whatToDo ?? []),
    e.howRescanConfirms,
    ...(e.references ?? []),
    ...(e.standards ?? []),
    e.exploitationStatus ?? '',
    e.coverageNote ?? ''
  ].join('\n');
}

/**
 * Currency amounts and probability-of-compromise claims found in `text`.
 *
 * Change 012's boundary: this view may state facts and wording, never an
 * expected loss or a likelihood of compromise, because those figures need the
 * loss model and calibration the explanation does not have. It looks for a
 * quantity, not for the words, so prose that names what it denies is allowed.
 * An empty result means no claim was found.
 */
export function unsupportedClaims(text: string): string[] {
  const found: string[] = [];
  const patterns: RegExp[] = [
    // £1,000  $2m  EUR 5k  1.2 million dollars
    /[£$€]\s?\d[\d,.]*\s?(?:[kKmMbB]\b|thousand|million|billion)?/g,
    /\b\d[\d,.]*\s?(?:thousand|million|billion|[kmb])?\s?(?:usd|gbp|eur|dollars?|pounds?|euros?)\b/gi,
    /\b(?:usd|gbp|eur)\s?\d[\d,.]*/gi,
    // 30% chance, 0.4 probability, 1 in 5 likelihood, p = 0.2
    /\d+(?:\.\d+)?\s?%\s*(?:likelihood|chance|probability|risk of|likely)/gi,
    /\b(?:likelihood|chance|probability|odds)\s+(?:of\s+)?(?:compromise\s+)?(?:is|of|at|around|about)?\s*\d+(?:\.\d+)?\s?%/gi,
    /\b\d+\s+in\s+\d+\s+(?:chance|likelihood|probability|odds)\b/gi,
    /\bp\s?=\s?0?\.\d+/gi
  ];
  for (const pattern of patterns) {
    for (const match of text.matchAll(pattern)) {
      found.push(match[0].trim());
    }
  }
  return found;
}
