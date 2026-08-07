import { DomainAssessment } from '../models/types';
import { DiscoveryOptions, DiscoveryResult, Scope, TrawlTransport } from './trawl-transport';

declare const window: any;

/**
 * WailsTransport speaks to the Go application over Wails IPC, which is what
 * the desktop build provides.
 *
 * Each binding is probed before it is called. A binding that is absent means
 * the desktop binary is older than this frontend, and returning an empty
 * result is the honest response: the UI can then say the capability is
 * unavailable rather than crashing on an undefined function.
 */
export class WailsTransport implements TrawlTransport {
  readonly kind = 'wails' as const;

  /** Reports whether the Wails runtime is present in this document. */
  static available(): boolean {
    return typeof window !== 'undefined' && !!window.go?.main?.App;
  }

  private get app(): any {
    return window.go?.main?.App ?? {};
  }

  private async call<T>(method: string, fallback: T, ...args: any[]): Promise<T> {
    const fn = this.app[method];
    if (typeof fn !== 'function') {
      console.warn(`Trawl: the desktop binding ${method} is unavailable in this build.`);
      return fallback;
    }
    return (await fn(...args)) ?? fallback;
  }

  /**
   * Invokes a binding with no fallback, throwing when it is absent.
   *
   * Read paths degrade to an empty result, because a missing list renders as
   * "nothing found" and the operator can see that for themselves. An action
   * has no such tell: silently doing nothing is indistinguishable from having
   * done it, and for a destructive action the operator would be left believing
   * their data was erased when every record remained.
   */
  private async callOrThrow<T>(method: string, ...args: any[]): Promise<T> {
    const fn = this.app[method];
    if (typeof fn !== 'function') {
      throw new Error(
        `Trawl: the desktop binding ${method} is unavailable in this build.`
      );
    }
    return await fn(...args);
  }

  getAssets(status = ''): Promise<any[]> {
    return this.call<any[]>('GetAssets', [], status);
  }

  getFindings(assetId: string): Promise<any[]> {
    return this.call<any[]>('GetFindings', [], assetId);
  }

  getSecretFindings(repoUrl = ''): Promise<any[]> {
    return this.call<any[]>('GetSecretFindings', [], repoUrl);
  }

  getEmailPostures(): Promise<any[]> {
    return this.call<any[]>('GetEmailPostures', []);
  }

  getRegressions(): Promise<any[]> {
    return this.call<any[]>('GetRegressions', []);
  }

  getAssessments(): Promise<DomainAssessment[]> {
    return this.call<DomainAssessment[]>('GetDomainAssessments', []);
  }

  getAssessment(domain: string): Promise<DomainAssessment | null> {
    return this.call<DomainAssessment | null>('GetDomainAssessment', null, domain);
  }

  assessDomain(domain: string): Promise<DomainAssessment | null> {
    return this.call<DomainAssessment | null>('AssessDomain', null, domain);
  }

  async getScope(): Promise<Scope | null> {
    const raw = await this.call<string>('GetSetting', '', 'scope_settings');
    if (!raw) {
      return null;
    }
    try {
      return JSON.parse(raw) as Scope;
    } catch {
      // A corrupted authorisation record is reported as absent, never as
      // permissive. Failing closed is the only safe reading of a scope
      // nobody can parse.
      console.error('Trawl: the stored scope could not be parsed; treating it as unset.');
      return null;
    }
  }

  async saveScope(scope: Scope): Promise<void> {
    await this.call<void>('SaveSetting', undefined, 'scope_settings', JSON.stringify(scope));
  }

  async triggerScan(domain: string, repoUrl: string): Promise<void> {
    await this.call<void>('TriggerScan', undefined, domain, repoUrl);
  }

  async eraseDiscoveredData(): Promise<void> {
    await this.callOrThrow('EraseDiscoveredData');
  }

  async discoverRelatedDomains(options: DiscoveryOptions): Promise<DiscoveryResult> {
    return (await this.callOrThrow('DiscoverRelatedDomains', options)) as DiscoveryResult;
  }

  async authoriseProposedDomains(domains: string[]): Promise<void> {
    await this.callOrThrow('AuthoriseProposedDomains', domains);
  }

  async dismissProposedDomains(domains: string[]): Promise<void> {
    await this.callOrThrow('DismissProposedDomains', domains);
  }

  async restoreDismissedDomain(domain: string): Promise<void> {
    await this.callOrThrow('RestoreDismissedDomain', domain);
  }

  async getDismissedDomains(): Promise<string[]> {
    return ((await this.callOrThrow('GetDismissedDomains')) as string[]) ?? [];
  }

  async removeAsset(id: string): Promise<void> {
    await this.callOrThrow('RemoveAsset', id);
  }

  on(event: string, handler: (payload: any) => void): () => void {
    if (!window.runtime?.EventsOn) {
      return () => undefined;
    }
    window.runtime.EventsOn(event, handler);
    return () => window.runtime.EventsOff?.(event);
  }
}
