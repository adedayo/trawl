import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';
import { ServiceObservation } from '../../../models/types';

export interface ServiceEvidenceSummary {
  facts: string[];
  issues: string[];
  recommendation: string;
}

export interface ServiceEvidenceHeadline extends ServiceEvidenceSummary {
  headline: string;
  goodCount: number;
  issueCount: number;
}

export function summariseServiceEvidence(observation: ServiceObservation): ServiceEvidenceSummary {
  const facts: string[] = [`${observation.layer.toUpperCase()} responded on port ${observation.port}`];
  const issues: string[] = [];
  let evidence: Record<string, unknown> = {};

  try {
    evidence = observation.evidence ? JSON.parse(observation.evidence) : {};
  } catch {
    issues.push('The probe returned evidence that could not be parsed.');
  }

  const tlsVersion = text(evidence['tls_version']);
  const cipher = text(evidence['cipher_suite']);
  const expiry = text(evidence['certificate_expiry']);
  const hsts = text(evidence['hsts']);
  const location = text(evidence['location']);
  const status = number(evidence['http_status']);
  const server = text(evidence['server']);

  if (status) facts.push(`HTTP ${status}`);
  if (tlsVersion) facts.push(`Negotiated ${tlsVersion}${cipher ? ` · ${cipher}` : ''}`);
  if (expiry) facts.push(`Certificate expires ${formatDate(expiry)}`);
  if (hsts) facts.push(`HSTS enabled: ${hsts}`);
  if (location) facts.push(`Redirects to ${location}`);
  if (server) facts.push(`Server identifies as ${server}`);

  if (observation.protocol.toLowerCase() === 'https' && !hsts) {
    issues.push('HSTS is not advertised.');
  }
  if (tlsVersion === 'TLS 1.0' || tlsVersion === 'TLS 1.1') {
    issues.push(`${tlsVersion} is obsolete.`);
  }
  if (expiry) {
    const expiryDate = new Date(expiry);
    if (!Number.isNaN(expiryDate.valueOf())) {
      const days = Math.ceil((expiryDate.valueOf() - Date.now()) / 86400000);
      if (days < 0) issues.push('The certificate is expired.');
      else if (days <= 30) issues.push(`The certificate expires in ${days} day${days === 1 ? '' : 's'}.`);
    }
  }
  if (observation.protocol.toLowerCase() === 'http' && observation.port === 80 && status >= 200 && status < 400 && !location) {
    issues.push('HTTP does not redirect visitors to HTTPS.');
  }

  return {
    facts,
    issues,
    recommendation: recommendationFor(observation, issues),
  };
}

export function serviceEvidenceHeadline(observation: ServiceObservation): ServiceEvidenceHeadline {
  const summary = summariseServiceEvidence(observation);
  return {
    ...summary,
    headline: summary.issues[0] || 'No immediate protocol issue detected.',
    goodCount: summary.facts.length,
    issueCount: summary.issues.length,
  };
}

export function matchesServiceFilter(observation: ServiceObservation, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const summary = summariseServiceEvidence(observation);
  return [
    observation.assetId, observation.host, observation.service, observation.protocol,
    String(observation.port), observation.layer, ...summary.facts, ...summary.issues,
    summary.recommendation,
  ].join(' ').toLowerCase().includes(needle);
}

function recommendationFor(observation: ServiceObservation, issues: string[]): string {
  if (issues.some(issue => issue.includes('expired'))) return 'Renew the certificate immediately and verify automated renewal.';
  if (issues.some(issue => issue.includes('obsolete'))) return 'Require TLS 1.2 or newer and remove obsolete protocol support.';
  if (issues.some(issue => issue.includes('HSTS'))) return 'Add Strict-Transport-Security after confirming the HTTPS estate is ready.';
  if (issues.some(issue => issue.includes('redirect'))) return 'Redirect HTTP to HTTPS and make the HTTPS endpoint canonical.';
  if (observation.service) return `Confirm that the exposed ${observation.service} service is intentional and restricted to the required sources.`;
  return 'Confirm that this externally reachable service is intentional and scoped.';
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function number(value: unknown): number {
  return typeof value === 'number' ? value : 0;
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleDateString();
}

@Component({
  selector: 'app-overview',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './overview.html'
})
export class OverviewComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  activeTab = this.wailsIpc.activeTab;
  assets = this.wailsIpc.assets;
  findings = this.wailsIpc.findings;
  assessments = this.wailsIpc.assessments;
  probing = signal(false);
  serviceFilter = signal('');

  exposedServices = computed(() => this.assessments()
    .flatMap(assessment => assessment.serviceExposures ?? [])
    .filter(exposure => exposure.stillExposed)
    .filter(exposure => this.serviceFilter().trim() === '' ||
      `${exposure.assetId} ${exposure.service}`.toLowerCase().includes(this.serviceFilter().trim().toLowerCase()))
    .slice(0, 100));

  recentServiceEvidence = computed(() => this.assessments()
    .flatMap(assessment => assessment.serviceObservations ?? [])
    .filter(observation => observation.state === 'responding')
    .filter(observation => matchesServiceFilter(observation, this.serviceFilter()))
    .sort((a, b) => b.observedAt.localeCompare(a.observedAt))
    .slice(0, 100));

  evidenceSummary(observation: ServiceObservation): ServiceEvidenceSummary {
    return summariseServiceEvidence(observation);
  }

  evidenceHeadline(observation: ServiceObservation): ServiceEvidenceHeadline {
    return serviceEvidenceHeadline(observation);
  }

  async probeServices(profile: 'most-common' | 'extended'): Promise<void> {
    this.probing.set(true);
    try {
      await this.wailsIpc.probeDiscoveredServices(profile);
    } finally {
      this.probing.set(false);
    }
  }

  pendingAssetCount = computed(() => this.assets().filter(a => a.status === 'pending').length);

  catalogueDate(finding: any, feed: string): string {
    return finding.enrichments?.find((item: any) => item.feed === feed)?.snapshot?.retrievedAt || 'not retrieved';
  }
}
