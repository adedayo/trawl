import { DomainAssessment } from '../models/types';
import { DiscoveryOptions, DiscoveryResult, Scope, TrawlTransport } from './trawl-transport';

/**
 * HttpTransport speaks to the headless Trawl server over its REST API and
 * Server-Sent Events stream. It is what the container deployment uses, where
 * no Wails runtime exists.
 *
 * The base URL is relative by default because nginx proxies /api and the
 * event stream to the server, so the dashboard is same-origin and needs no
 * CORS configuration and no credentials in the browser.
 */
export class HttpTransport implements TrawlTransport {
  readonly kind = 'http' as const;

  private source?: EventSource;
  private readonly handlers = new Map<string, Set<(payload: any) => void>>();

  constructor(private readonly baseUrl: string = '') {}

  private url(path: string): string {
    return `${this.baseUrl}${path}`;
  }

  private async get<T>(path: string, fallback: T): Promise<T> {
    try {
      const response = await fetch(this.url(path), {
        headers: { Accept: 'application/json' }
      });
      if (!response.ok) {
        console.error(`Trawl: GET ${path} failed with ${response.status}`);
        return fallback;
      }
      return ((await response.json()) as T) ?? fallback;
    } catch (e) {
      console.error(`Trawl: GET ${path} failed:`, e);
      return fallback;
    }
  }

  private async send<T>(method: string, path: string, body?: unknown, fallback?: T): Promise<T | undefined> {
    try {
      const response = await fetch(this.url(path), {
        method,
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body)
      });
      if (!response.ok) {
        console.error(`Trawl: ${method} ${path} failed with ${response.status}`);
        return fallback;
      }
      if (response.status === 204) {
        return fallback;
      }
      return ((await response.json()) as T) ?? fallback;
    } catch (e) {
      console.error(`Trawl: ${method} ${path} failed:`, e);
      return fallback;
    }
  }

  getAssets(status = ''): Promise<any[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.get<any[]>(`/api/v1/assets${query}`, []);
  }

  getFindings(assetId: string): Promise<any[]> {
    const query = assetId ? `?assetId=${encodeURIComponent(assetId)}` : '';
    return this.get<any[]>(`/api/v1/findings${query}`, []);
  }

  getSecretFindings(repoUrl = ''): Promise<any[]> {
    const query = repoUrl ? `?repoUrl=${encodeURIComponent(repoUrl)}` : '';
    return this.get<any[]>(`/api/v1/secret-findings${query}`, []);
  }

  getEmailPostures(): Promise<any[]> {
    return this.get<any[]>('/api/v1/email-postures', []);
  }

  getRegressions(): Promise<any[]> {
    return this.get<any[]>('/api/v1/regressions', []);
  }

  getAssessments(): Promise<DomainAssessment[]> {
    return this.get<DomainAssessment[]>('/api/v1/assessments', []);
  }

  getAssessment(domain: string): Promise<DomainAssessment | null> {
    return this.get<DomainAssessment | null>(`/api/v1/assessments/${encodeURIComponent(domain)}`, null);
  }

  async assessDomain(domain: string): Promise<DomainAssessment | null> {
    const result = await this.send<DomainAssessment>(
      'POST',
      `/api/v1/assessments/${encodeURIComponent(domain)}`
    );
    return result ?? null;
  }

  getScope(): Promise<Scope | null> {
    return this.get<Scope | null>('/api/v1/scope', null);
  }

  async saveScope(scope: Scope): Promise<void> {
    await this.send('PUT', '/api/v1/scope', scope);
  }

  async triggerScan(domain: string, repoUrl: string): Promise<void> {
    await this.send('POST', '/api/v1/scans', { domain, repoUrl });
  }

  /**
   * Unlike the other writes, this one propagates failure rather than logging
   * it and returning. The operator is about to be told their estate was
   * erased, and a console message they will never see is not an answer.
   */
  async eraseDiscoveredData(): Promise<void> {
    const response = await fetch(this.url('/api/v1/discovered-data'), {
      method: 'DELETE',
      headers: { Accept: 'application/json' }
    });
    if (!response.ok) {
      throw new Error(`Trawl: erasing discovered data failed with ${response.status}`);
    }
  }

  removeAsset(id: string): Promise<void> {
    return this.mutate('DELETE', `/api/v1/assets/${encodeURIComponent(id)}`);
  }

  /**
   * Discovery propagates failure rather than swallowing it. An empty proposal
   * list and a failed search look identical in the UI, and the difference
   * matters: one means "nothing else is out there", the other means "we do not
   * know". Only an exception can tell them apart.
   */
  async discoverRelatedDomains(options: DiscoveryOptions): Promise<DiscoveryResult> {
    const response = await fetch(this.url('/api/v1/discovery/related'), {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify(options)
    });
    if (!response.ok) {
      throw new Error(`Trawl: discovery failed with ${response.status}`);
    }
    return (await response.json()) as DiscoveryResult;
  }

  async authoriseProposedDomains(domains: string[]): Promise<void> {
    await this.sendOrThrow('POST', '/api/v1/discovery/authorise', { domains });
  }

  async dismissProposedDomains(domains: string[]): Promise<void> {
    await this.sendOrThrow('POST', '/api/v1/discovery/dismiss', { domains });
  }

  async restoreDismissedDomain(domain: string): Promise<void> {
    await this.sendOrThrow('POST', '/api/v1/discovery/restore', { domain });
  }

  getDismissedDomains(): Promise<string[]> {
    return this.get<string[]>('/api/v1/discovery/dismissed', []);
  }

  /**
   * A write that reports failure. The scope-changing calls above must not
   * silently do nothing: authorising a domain is the operator asserting
   * authority, and they have to know whether it took effect.
   */
  private async sendOrThrow(method: string, path: string, body: unknown): Promise<void> {
    const response = await fetch(this.url(path), {
      method,
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!response.ok) {
      throw new Error(`Trawl: ${method} ${path} failed with ${response.status}`);
    }
  }

  /**
   * Performs a write, throwing on failure.
   *
   * `send` logs and returns a fallback, which suits a fire-and-forget write
   * whose effect the operator will see reflected anyway. An action taken on a
   * specific record is not that: if it fails, the row stays as it was and the
   * only evidence is a console line nobody is reading.
   */
  private async mutate(method: string, path: string, body?: unknown): Promise<void> {
    const response = await fetch(this.url(path), {
      method,
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
    if (!response.ok) {
      throw new Error(`Trawl: ${method} ${path} failed with ${response.status}`);
    }
  }

  on(event: string, handler: (payload: any) => void): () => void {
    this.ensureStream();

    let set = this.handlers.get(event);
    if (!set) {
      set = new Set();
      this.handlers.set(event, set);
      // Named SSE events must be registered individually on the EventSource.
      this.source?.addEventListener(event, (e: MessageEvent) => this.dispatch(event, e));
    }
    set.add(handler);

    return () => set!.delete(handler);
  }

  /**
   * Opens the stream lazily and only once. The browser reconnects on its own
   * when the connection drops, which is why this is SSE rather than a socket
   * the application would have to babysit.
   */
  private ensureStream(): void {
    if (this.source || typeof EventSource === 'undefined') {
      return;
    }
    this.source = new EventSource(this.url('/api/v1/events'));
    this.source.onerror = () => {
      // Reported, not repaired: EventSource retries by itself, and a manual
      // reconnect here would race with it and multiply the connections.
      console.warn('Trawl: the event stream dropped; the browser will retry.');
    };
  }

  private dispatch(event: string, e: MessageEvent): void {
    let payload: any = null;
    try {
      // The server frames each event as the full bus record; components care
      // only about its payload, which is what Wails delivers too.
      payload = JSON.parse(e.data)?.payload ?? null;
    } catch {
      payload = null;
    }
    this.handlers.get(event)?.forEach(handler => handler(payload));
  }
}
