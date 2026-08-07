import { DomainAssessment } from '../models/types';

/**
 * The scope is the operator's declared authorisation. It is the same record in
 * both deployments, held in the same settings key, so a portfolio authorised
 * in the desktop app is still authorised when the container is pointed at the
 * same database.
 */
export interface Scope {
  seedDomainsList: string[];
  seedCidrsList: string[];
  seedReposList: string[];
  consentedEndpoints: string[];
  isAuthorized: boolean;
  signerName: string;
  signerTitle: string;
  authorizationDate: string;
  proposedDomains?: ProposedDomain[];
  dismissedDomains?: string[];
}

/**
 * A registrable domain that certificate evidence suggests belongs to the
 * operator, awaiting their decision.
 *
 * It is not an asset. Nothing has been queried and nothing will be until it is
 * authorised: certificate co-tenancy is evidence of ownership, not proof, and
 * only the operator can close that gap.
 */
export interface ProposedDomain {
  domain: string;
  /** The authorised domain whose certificate named this one — the evidence. */
  via: string;
  /** Hops of co-tenancy from an authorised domain. Higher means weaker. */
  depth: number;
  hostnames: string[];
  issuer: string;
  discoveredAt: string;
}

/** One discovery pass. */
export interface DiscoveryResult {
  proposals: ProposedDomain[];
  searched: string[];
  /** The walk stopped early, so the proposal list may be incomplete. */
  budgetExhausted: boolean;
  errors: string[];
}

/**
 * How far certificate evidence is followed.
 *
 * Zero values mean the conservative backend defaults, so a caller that has no
 * opinion does not have to invent one.
 */
export interface DiscoveryOptions {
  depth: number;
  budget: number;
  maxSans: number;
}

/**
 * TrawlTransport is every operation the UI performs, expressed independently
 * of how it reaches the backend.
 *
 * Trawl ships as a Wails desktop application and as a container serving the
 * same Angular bundle over nginx. Without this seam the frontend would have to
 * know which one it is running inside, and in practice that knowledge spreads:
 * a component calls `window.go` directly, and the browser deployment loses a
 * feature that nobody notices is missing until an operator asks for it.
 *
 * So components depend on this interface and never on a transport.
 */
export interface TrawlTransport {
  /** Names the transport, for diagnostics and for the UI to disclose it. */
  readonly kind: 'wails' | 'http';

  getAssets(status?: string): Promise<any[]>;

  /**
   * Deletes an asset and everything recorded against it.
   *
   * Discovery is high-fidelity, so assets are not held for approval; removal
   * is the operator's only ruling on an asset.
   */
  removeAsset(id: string): Promise<void>;
  getFindings(assetId: string): Promise<any[]>;
  getSecretFindings(repoUrl?: string): Promise<any[]>;
  getEmailPostures(): Promise<any[]>;
  getRegressions(): Promise<any[]>;

  getAssessments(): Promise<DomainAssessment[]>;
  getAssessment(domain: string): Promise<DomainAssessment | null>;
  assessDomain(domain: string): Promise<DomainAssessment | null>;

  getScope(): Promise<Scope | null>;
  saveScope(scope: Scope): Promise<void>;

  triggerScan(domain: string, repoUrl: string): Promise<void>;

  /**
   * Erases everything the engine discovered, preserving configuration and
   * authorised scope.
   *
   * It rejects when the erasure fails. A caller must be able to tell the
   * operator the truth about a destructive action, and a signature that
   * cannot fail would force it to claim success unconditionally.
   */
  eraseDiscoveredData(): Promise<void>;

  /**
   * Searches certificate transparency logs for domains related to the
   * authorised ones, returning them as proposals.
   *
   * This assesses nothing and touches none of the discovered domains. It is
   * the "discover" half of discover-then-authorise, and is safe to run
   * speculatively precisely because it is inert.
   */
  discoverRelatedDomains(options: DiscoveryOptions): Promise<DiscoveryResult>;

  /** Brings proposals into the authorised scope, making them assessable. */
  authoriseProposedDomains(domains: string[]): Promise<void>;

  /** Rules proposals out and remembers the decision, so they do not return. */
  dismissProposedDomains(domains: string[]): Promise<void>;

  /** Returns a dismissed domain to consideration, undoing a dismissal. */
  restoreDismissedDomain(domain: string): Promise<void>;

  /** The domains previously ruled out. */
  getDismissedDomains(): Promise<string[]>;

  /**
   * Subscribes to a backend event, returning an unsubscribe function.
   *
   * Both transports deliver the same event names and payloads — the Wails
   * runtime emits them directly, the HTTP transport receives them as
   * Server-Sent Events — so a component that reacts to `scan:progress` needs
   * no knowledge of which one is beneath it.
   */
  on(event: string, handler: (payload: any) => void): () => void;
}
