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
