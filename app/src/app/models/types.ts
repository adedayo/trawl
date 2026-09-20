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
