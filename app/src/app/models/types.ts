export interface AssetUI {
  id: string;
  type: 'domain' | 'ip' | 'repository';
  value: string;
  dnsNames?: string[];
  source: string;
  confidence: 'high' | 'medium' | 'low';
  status: 'active' | 'pending' | 'inactive' | 'rejected';
  firstSeen: string;
  lastSeen: string;
}

export interface FindingUI {
  id: string;
  assetValue: string;
  cveId: string;
  cpe?: string;
  title: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info';
  kev: boolean;
  epssScore: number;
  cvssScore: number;
  status: 'open' | 'resolved' | 'reopened';
  aiAnnotation?: {
    summary: string;
    remediation: string;
  };
  detectedAt: string;
  enrichments?: FindingEnrichmentUI[];
}

export interface FindingEnrichmentUI {
  feed: 'cisa-kev' | 'epss' | 'nvd' | string;
  state: 'ok' | 'not_found' | 'not_checked' | 'check_failed' | string;
  snapshotId?: string;
  kevListed?: boolean;
  epss?: number;
  checkedAt?: string;
  snapshot?: {
    id: string;
    feed?: string;
    retrievedAt: string;
    sourceUrl: string;
    contentDigest?: string;
    recordCount?: number;
  };
}

export interface FeedStatusUI {
  feed: 'cisa-kev' | 'epss' | 'nvd' | string;
  snapshot?: {
    id: string;
    feed: string;
    sourceUrl: string;
    retrievedAt: string;
    contentDigest: string;
    recordCount: number;
  };
  /**
   * The last time the source was successfully consulted, whether or not its
   * content had changed. `snapshot.retrievedAt` is when the content in force
   * first arrived, which stops moving while the catalogue is unchanged.
   */
  lastCheckedAt?: string;
  /** Whether the catalogue's entries are held locally, not only its metadata. */
  recordsStored?: boolean;
  lastFailureAt?: string;
  lastError?: string;
  stale: boolean;
}

export interface IntervalUI {
  point: number;
  low: number;
  high: number;
}

export interface ContactFactorTraceUI {
  id: string;
  name: string;
  value: number;
  source: string;
  evidenceClass: string;
  observedAt?: string;
  state: string;
}

export interface ContactPriorEstimateUI {
  assetId: string;
  serviceId: string;
  serviceClass: string;
  weaknessClass: string;
  available: boolean;
  suppressedReason?: string;
  lambdaContact: IntervalUI;
  windowDays: number;
  windowRationale: string;
  pContact: IntervalUI;
  timeAtRisk: {
    observedDays: number;
    inferredDays: number;
    estimatedBlindDays: number;
    expectedDetectionDays: number;
    leftCensored: boolean;
    stillExposed: boolean;
  };
  factors: ContactFactorTraceUI[];
  excludedFactors?: string[];
  coverage: {
    checksConcluded: number;
    checksApplicable: number;
  };
  label: 'illustrative' | 'sourced' | 'calibrated' | string;
  scopeStatement: string;
  isPrior: boolean;
}

export interface AssetContactPanelUI {
  assetId: string;
  assetValue: string;
  reachability: 'responding' | 'not_responding' | 'unknown' | string;
  estimates: ContactPriorEstimateUI[];
  scopeStatement: string;
}

export interface InstanceAggregationUI {
  weaknessClass: string;
  totalInstances: number;
  confirmedRemediated: number;
  uncoveredInstances: number;
  unknownReachability: number;
  available: boolean;
  allInstancesPContact: IntervalUI;
  residualPContact: IntervalUI;
  coverage: {
    checksConcluded: number;
    checksApplicable: number;
  };
  label: string;
  scopeStatement: string;
  isPrior: boolean;
}

/**
 * The assessed state of one email-authentication control on one domain.
 *
 * This mirrors `store.EmailControl`. It replaced a boolean, and the reason is
 * that `spfValid: boolean` cannot distinguish "the domain publishes no SPF
 * record" from "the resolver never answered" — and those demand opposite
 * responses. Collapsed into one value, an outage reads as a misconfiguration,
 * or a misconfiguration reads as a clean bill of health.
 */
export interface EmailControlUI {
  state: CoverageState;
  /** The salient value where the control has one: the DMARC policy, the MTA-STS mode. */
  detail?: string;
  /**
   * Why the state is not an assessment. Render this wherever a control is
   * `not_checked` or `check_failed`: "we could not tell" is only actionable
   * when it says why.
   */
  reason?: string;
  /**
   * Whether the state may be read as a fact about the domain rather than
   * about the limits of the search.
   *
   * This exists for DKIM. Selectors cannot be enumerated from DNS, so probing
   * the common list and finding nothing establishes nothing. A `not_found`
   * DKIM control with `conclusive: false` must never be rendered as "DKIM
   * absent" — the domain may sign every message with a selector nobody
   * guessed, and an operator told otherwise would commission work already done.
   */
  conclusive: boolean;
}

/**
 * A domain's email-authentication posture, as the backend reports it.
 *
 * The DMARC tags are carried as data rather than prose because severity is a
 * deterministic function of them, computed in Go. The UI renders; it does not
 * decide severity.
 */
export interface EmailPostureUI {
  domain: string;

  /** The three authentication controls. */
  spf: EmailControlUI;
  dkim: EmailControlUI;
  dmarc: EmailControlUI;

  /** The adjacent records. Never as consequential as a missing DMARC policy. */
  mtaSts: EmailControlUI;
  tlsRpt: EmailControlUI;
  bimi: EmailControlUI;
  caa: EmailControlUI;

  dmarcPolicy: string;
  dmarcSubdomainPolicy?: string;
  dmarcPercent: number;
  dmarcAlignmentSpf?: string;
  dmarcAlignmentDkim?: string;
  dmarcReporting: boolean;

  /**
   * The qualifier on the terminating all-mechanism. "+" authorises the entire
   * internet to send as the domain.
   */
  spfAllMechanism?: string;
  /**
   * How many DNS-querying mechanisms the record expands to. Above ten,
   * receivers may stop evaluating it, so a domain can publish a careful policy
   * that is enforced nowhere.
   */
  spfLookups: number;

  /**
   * Both are rendered together deliberately: "1 of 13 selectors found" tells a
   * reader something quite different from "a key exists".
   */
  dkimSelectorsExamined?: string[];
  dkimSelectorsFound?: string[];

  /** Computed in Go from the tags above. Never set by the AI-triage layer. */
  priority: 'critical' | 'high' | 'medium' | 'low' | 'info' | '';
  lastChecked: string;

  /**
   * How many of the seven controls reached a conclusion, and how many were
   * examined at all.
   *
   * Computed in Go by `EmailPosture.Assessed` and added at the serialisation
   * boundary, not mirrored here. "Assessed" is defined once, in the engine
   * that also computes severity from it; a second definition in TypeScript
   * would eventually disagree, and then one half of the product would report
   * coverage the other half denied.
   *
   * Optional only so that a record written by an older build — which served
   * no such field — reads as absent rather than as zero controls assessed.
   */
  assessedControls?: number;
  totalControls?: number;

  /**
   * True when this record was written before the posture became four-state,
   * and therefore carries no control states at all.
   *
   * Change 006 Phase 9 widened the record and deliberately did not promote the
   * old booleans, because reconstructing four states from two would have
   * preserved exactly the collapse the widening removed. So an installation
   * upgraded across that boundary holds domains that read as wholly unassessed
   * until they are next scanned.
   *
   * That is true, and it is still misleading: seven unassessed controls look
   * identical to a domain nobody has reached, and both look — to a hurried
   * reader — like a clean result. The flag is what lets the view say which it
   * is, and the remedy is a rescan rather than anything the operator must fix
   * on the domain.
   *
   * Computed in Go by `EmailPosture.PredatesAssessment` and omitted entirely
   * when false, so its presence is the signal.
   */
  predatesAssessment?: boolean;
}

/** The seven controls a posture carries, in the order an operator reasons about them. */
export const EMAIL_POSTURE_CONTROLS = [
  'spf', 'dkim', 'dmarc', 'mtaSts', 'tlsRpt', 'bimi', 'caa'
] as const;

export type EmailPostureControl = typeof EMAIL_POSTURE_CONTROLS[number];

/**
 * The four-state assessment outcome, carried end to end from vantage.
 *
 * These are never collapsed to a boolean in the UI. "No record published",
 * "we did not look" and "we looked and could not tell" are three different
 * statements, and rendering any of them as a tick would tell an operator a
 * control is in place when nothing of the sort was established.
 */
export type CoverageState = 'ok' | 'not_found' | 'not_checked' | 'check_failed';

/** The derived standing of one control. Unknown is never evidence of health. */
export type ControlPosture = 'compliant' | 'deficient' | 'unknown';

export interface CoverageSummary {
  total: number;
  ok: number;
  notFound: number;
  notChecked: number;
  checkFailed: number;
  assessedOnly: number;
}

export interface SignalView {
  signalId: string;
  checkId: string;
  condition: string;
  weaknessClass: string;
  scenario: string;
  stage: string;
  control: string;
  direction: 'aggravating' | 'mitigating' | '';
  state: CoverageState;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info';
  evidence: string;
  /**
   * Vantage's catalogue prose for this identifier: what the observation
   * means, what to do about it, and the standard that says so. Optional
   * because an observation stored under a different library version may name
   * an identifier the installed catalogue does not define.
   */
  description?: string;
  remediation?: string;
  references?: string[];
  /**
   * The part of the explanation that is specific to this observation: which
   * include term is broken, which exchanger does not resolve. It comes from
   * the stored observation rather than the catalogue, because the catalogue
   * describes an identifier and has never seen this domain.
   *
   * Rendered as prose next to the description, so that the item at fault is
   * read rather than recovered from the evidence string. Absent for signals
   * that name no particular item, and for observations recorded before the
   * field existed.
   */
  detail?: string;
  mapped: boolean;
  registryVersion: string;
  libraryVersion: string;
  observedAt: string;
  firstSeen: string;
}

export interface CheckView {
  checkId: string;
  state: CoverageState;
  reason?: string;
}

export interface ControlView {
  control: string;
  posture: ControlPosture;
  coverage: CoverageSummary;
  checks: CheckView[];
  signals: SignalView[];
}

export interface ScenarioView {
  scenario: string;
  coverage: CoverageSummary;
  aggravating: number;
  /**
   * The subset of `aggravating` at medium severity or above. Carried
   * alongside rather than instead of it: leading with the weightier number
   * must not make the lighter findings cease to exist.
   */
  significant: number;
  mitigating: number;
  supported: boolean;
}

/**
 * One address an asset's name resolves to, and what it could be attributed to.
 *
 * Held per address rather than per name, because a name routinely resolves to
 * several and they need not agree. A name balanced across two jurisdictions is
 * a fact to show, not a discrepancy to reduce to one winner.
 */
export interface AssetAttribution {
  assetId: string;
  host: string;
  role?: string;
  address: string;
  /**
   * The operator announcing the address. Empty means no published range
   * matched — which is the absence of a match, not a claim that the address is
   * unhosted. It is only trustworthy when coverage says the ranges loaded, and
   * the engine degrades the check to `check_failed` when they did not.
   */
  provider?: string;
  region?: string;
  /**
   * The ISO 3166-1 alpha-2 country. Empty means unknown, never "assume home":
   * an unknown jurisdiction rendered as the domestic one would turn a
   * data-residency question into a false reassurance.
   */
  jurisdiction?: string;
  source?: string;
  libraryVersion?: string;
  observedAt: string;
}

/**
 * Where a provider's range data came from, and when.
 *
 * Travels with the attribution and is never shown apart from it: attribution
 * displayed without its basis reads as current when it may have come from a
 * cache days old.
 */
export interface AttributionProvenance {
  assetId: string;
  provider: string;
  url: string;
  fetchedAt: string;
}

export interface ServiceExposure {
  assetId: string;
  service: string;
  firstObserved: string;
  lastObserved: string;
  stillExposed: boolean;
  leftCensored: boolean;
  observedDurationSeconds: number;
  inferredDurationSeconds: number;
  blindDurationSeconds: number;
  expectedBlindSeconds: number;
  worstBlindSeconds: number;
}

export interface ServiceObservation {
  assetId: string;
  host: string;
  port: number;
  service: string;
  transport: string;
  protocol: string;
  layer: string;
  state: string;
  coverage: CoverageState;
  evidence?: string;
  profile: string;
  observedAt: string;
}

export interface DomainAssessment {
  assetId: string;
  domain: string;
  outcome: 'completed' | 'partial' | 'failed' | 'refused' | 'cancelled';
  error?: string;
  coverage: CoverageSummary;
  coverageFraction: number;
  controls: ControlView[];
  scenarios: ScenarioView[];
  unmapped: SignalView[];
  attribution: AssetAttribution[];
  attributionProvenance: AttributionProvenance[];
  serviceExposures?: ServiceExposure[];
  serviceObservations?: ServiceObservation[];
  registryVersion: string;
  libraryVersion: string;
  assessedAt?: string;
}

export interface SecretFindingUI {
  _id?: string;
  _creationTime?: number;
  repoUrl: string;
  filePath: string;
  provider: string;
  redactedRef: string;
  commitSha: string;
  verified: boolean;
  lineNumber?: number;
  checkmateVersion?: string;
  priority: 'critical' | 'high' | 'medium' | 'low';
  status: 'open' | 'resolved';
  firstSeen: string;
  lastSeen: string;
  detectedAt: string;
}

// ---------------------------------------------------------------------------
// Change 025: findings history, drill-down and explanation.
//
// These mirror pkg/service/findings_types.go and pkg/narrative/types.go field
// for field. They are the wire contract, so a change on the Go side is a change
// here. Optional fields are the Go `omitempty` ones.
// ---------------------------------------------------------------------------

/** Which table holds a finding's current state. */
export type FindingSource = 'signal' | 'finding' | 'secret' | 'candidate';

export type FindingSeverity = 'critical' | 'high' | 'medium' | 'low' | 'info';

/** The lifecycle status the engine records. `active` exists only as a filter. */
export type FindingStatus = 'open' | 'resolved' | 'reopened';

/** The filter the index endpoint accepts. Empty fields do not filter. */
export interface FindingFilter {
  /** `open`, `resolved`, `reopened`, or `active` (open and reopened). */
  status?: string;
  severity?: string;
  source?: string;
  assetId?: string;
  /** RFC 3339. Only findings with an event at or after it are returned. */
  changedSince?: string;
}

/** One row of the unified findings list. */
export interface FindingSummary {
  /** The stable, URL-safe address of the finding. */
  key: string;
  source: FindingSource;
  assetId: string;
  assetValue: string;
  title: string;

  severity: FindingSeverity;
  priority: string;
  status: FindingStatus;

  /**
   * True when the finding is open only because the check that raises it could
   * not conclude on the latest run. Rendered distinctly from an open finding
   * that was re-assessed and is still present.
   */
  notReassessed: boolean;
  notReassessedReason?: string;
  /** The source row's own id, for signal findings. Exact; prefer it to matching text. */
  signalId?: string;
  /** The source row's own id, for secret findings. */
  secretId?: string;

  category: string;
  cve?: string;
  kev: boolean;
  /** True for software-matched candidates: unconfirmed until verified. */
  unconfirmed?: boolean;
  /** Rating origin for candidates (for example `nvd`). */
  severitySource?: string;

  firstSeen: string;
  lastSeen: string;
  resolvedAt?: string;
  /** Whole days from first seen to now, or to resolution. */
  ageDays: number;
  lastChange: string;
  lastChangeType: string;
}

export type FindingEventType =
  | 'baseline'
  | 'first_seen'
  | 'changed'
  | 'priority_changed'
  | 'resolved'
  | 'reopened';

/**
 * Why an event happened. `library` means the scanner changed between
 * observations, which is not a change in the estate; `feed` cites a feed
 * snapshot; `baseline` marks pre-history rows.
 */
export type FindingEventCause = 'estate' | 'library' | 'feed' | 'baseline';

/** One entry in a finding's timeline. */
export interface FindingEventView {
  id: string;
  type: FindingEventType | string;
  at: string;
  runId?: string;
  cause: FindingEventCause | string;
  prevSeverity?: string;
  severity?: string;
  prevPriority?: string;
  priority?: string;
  libraryVersion?: string;
  registryVersion?: string;
  /** Cites the feed snapshot behind a feed-caused change. */
  snapshotId?: string;
  note?: string;
}

/** A feed snapshot, as the store serialises it. */
export interface FeedSnapshotUI {
  id: string;
  feed: string;
  sourceUrl: string;
  retrievedAt: string;
  contentDigest: string;
  recordCount: number;
}

/** What the finding's current state rests on. */
export interface FindingProvenance {
  runId?: string;
  libraryVersion?: string;
  registryVersion?: string;
  feedSnapshots?: FeedSnapshotUI[];
}

/**
 * The rendered, sectioned explanation of one finding (pkg/narrative).
 *
 * Curated, versioned and deterministic. It is distinct from any AI annotation,
 * which is advisory and, in this change, not rendered at all.
 */
export interface Explanation {
  /** Which narrative level supplied the wording: signal, weakness-class, ... */
  level: string;
  levelKey: string;
  /** The narrative set the wording came from. */
  version: string;

  whatWeFound: string;
  whyItMatters: string;
  howItCouldBeAbused: string;
  whatGoodLooksLike: string;
  whatToDo: string[];
  howRescanConfirms: string;
  references: string[];
  standards?: string[];

  /** KEV/EPSS in words with the date checked, or that it could not be established. */
  exploitationStatus?: string;
  /** Set when the assessment behind the finding did not conclude. */
  coverageNote?: string;
}

/** Everything the drill-down view shows. */
export interface FindingDetail extends FindingSummary {
  description?: string;
  /** The occurrence-specific sentence; evidence is the measured value. */
  detail?: string;
  evidence?: string;

  checkId?: string;
  checkState?: CoverageState;
  checkReason?: string;

  // Candidate software-match evidence (populated when source === 'candidate').
  observedString?: string;
  matchedVendor?: string;
  matchedProduct?: string;
  matchedVersion?: string;
  lookupSnapshotId?: string;
  lookupAt?: string;

  explanation: Explanation;
  narrativeVersion: string;

  timeline: FindingEventView[];
  regressions: RegressionRecord[];
  provenance: FindingProvenance;
}

/** A confirmed posture regression, as `store.Regression` serialises it. */
export interface RegressionRecord {
  id: string;
  assetId: string;
  attributeType: string;
  previousValue: string;
  currentValue: string;
  consecutiveFails: number;
  confirmedAt: string;
}

/** One run in the ledger, with the coverage of what it attempted. */
export interface ScanRunView {
  id: string;
  /** assessment, service-probe, ingest-scan, secret-scan or feed-refresh. */
  kind: string;
  assetId?: string;
  assetValue?: string;
  profile?: string;
  outcome: string;
  libraryVersion?: string;
  registryVersion?: string;
  startedAt: string;
  finishedAt: string;
  coverage: CoverageSummary;
  /** Concluded checks over intended checks, 0 to 1. Zero when none were intended. */
  coverageFraction: number;
}

/**
 * One run's summary: open findings by severity when the run finished, the
 * transitions it caused, and its coverage. Trend lines are drawn from these
 * and only these; nothing is interpolated between runs.
 */
export interface HistoryPoint {
  run: ScanRunView;
  at: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
  info: number;
  open: number;
  resolved: number;
  /** Transitions this run caused. */
  new: number;
  resolvedInRun: number;
}

/** One finding that moved between two points in time. */
export interface FindingChange {
  finding: FindingSummary;
  event: FindingEventView;
}

/** The difference between two runs, or between a run and now. */
export interface ChangeSet {
  from?: HistoryPoint | null;
  to?: HistoryPoint | null;
  new: FindingChange[];
  resolved: FindingChange[];
  worsened: FindingChange[];
  improved: FindingChange[];
  /** Anything that limits how the comparison should be read, notably a coverage drop. */
  note?: string;
}

// ---------------------------------------------------------------------------
// Change 026: software inventory and unconfirmed candidate vulnerabilities.
// ---------------------------------------------------------------------------

export type SoftwareMappingState = 'mapped' | 'unmapped';
export type SoftwareVersionState = 'known' | 'unknown';
export type SoftwareStatus = 'current' | 'superseded';
export type SoftwareCoverageState =
  | 'not_assessed'
  | 'seen_unmapped'
  | 'version_unknown'
  | 'identified';

export interface AssetSoftwareUI {
  id: string;
  assetId: string;
  observedName: string;
  cpePart?: string;
  vendor?: string;
  product?: string;
  mappingState: SoftwareMappingState;
  mappingVersion?: string;
  version?: string;
  versionState: SoftwareVersionState;
  source: string;
  rawString: string;
  status: SoftwareStatus;
  firstSeen: string;
  lastSeen: string;
  supersededAt?: string;
}

export interface SoftwareCoverageUI {
  assetId: string;
  state: SoftwareCoverageState;
  identified: number;
  versionUnknown: number;
  unmapped: number;
  superseded: number;
}

export interface CandidateRefreshSummaryUI {
  runId: string;
  snapshotId?: string;
  queried: number;
  cacheHits: number;
  candidates: number;
  resolved: number;
  skippedVersionUnknown: number;
  skippedUnmapped: number;
}
