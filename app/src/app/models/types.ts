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

export interface EmailPostureUI {
  domain: string;
  spfValid: boolean;
  dkimFound: boolean;
  dmarcPolicy: 'reject' | 'quarantine' | 'none' | 'missing';
  priority: 'critical' | 'high' | 'medium' | 'low' | 'info';
  lastChecked: string;
}

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
