import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WailsIpcService } from '../../../wails-ipc.service';

@Component({
  selector: 'app-scope',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './scope.html',
  styleUrls: ['./scope.css']
})
export class ScopeComponent {
  wailsIpc = inject(WailsIpcService);
  theme = this.wailsIpc.theme;
  
  seedDomainsList = this.wailsIpc.seedDomainsList;
  seedCidrsList = this.wailsIpc.seedCidrsList;
  seedReposList = this.wailsIpc.seedReposList;

  isAuthorized = this.wailsIpc.isAuthorized;
  signerName = this.wailsIpc.signerName;
  signerTitle = this.wailsIpc.signerTitle;
  authorizationDate = this.wailsIpc.authorizationDate;

  formattedDomains = computed(() => this.seedDomainsList().length > 0 ? this.seedDomainsList().join(', ') : 'None');
  formattedCidrs = computed(() => this.seedCidrsList().length > 0 ? this.seedCidrsList().join(', ') : 'None');
  formattedRepos = computed(() => this.seedReposList().length > 0 ? this.seedReposList().join(', ') : 'None');

  addDomain(value: string) {
    const trimmed = value.trim().toLowerCase();
    if (!trimmed || this.seedDomainsList().includes(trimmed)) return;
    this.seedDomainsList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeDomain(domain: string) {
    this.seedDomainsList.update(list => list.filter(d => d !== domain));
    this.wailsIpc.saveSettings();
  }

  addCidr(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedCidrsList().includes(trimmed)) return;
    this.seedCidrsList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeCidr(cidr: string) {
    this.seedCidrsList.update(list => list.filter(c => c !== cidr));
    this.wailsIpc.saveSettings();
  }

  addRepo(value: string) {
    const trimmed = value.trim();
    if (!trimmed || this.seedReposList().includes(trimmed)) return;
    this.seedReposList.update(list => [...list, trimmed]);
    this.wailsIpc.saveSettings();
  }

  removeRepo(repo: string) {
    this.seedReposList.update(list => list.filter(r => r !== repo));
    this.wailsIpc.saveSettings();
  }

  signAuthorization(name: string, title: string) {
    if (!name.trim()) return;
    this.wailsIpc.isAuthorized.set(true);
    this.wailsIpc.signerName.set(name);
    this.wailsIpc.signerTitle.set(title || 'Security Lead');
    this.wailsIpc.authorizationDate.set(new Date().toISOString().split('T')[0]);
    this.wailsIpc.saveSettings();
  }

  revokeAuthorization() {
    this.wailsIpc.isAuthorized.set(false);
    this.wailsIpc.saveSettings();
  }

  /** True while an erasure is in flight, so the button can be disabled. */
  readonly wiping = signal(false);

  /** True once the operator has asked to erase and must confirm in place. */
  readonly confirmingWipe = signal(false);

  /** The outcome of the last erasure, shown in the panel. */
  readonly wipeMessage = signal<string>('');

  /** Whether the last outcome was a failure, so it can be coloured as one. */
  readonly wipeFailed = signal(false);

  /**
   * Asks for confirmation in the page rather than through window.confirm.
   *
   * The desktop build runs in a WKWebView that does not implement the native
   * JavaScript dialogs, so confirm() returned falsy and the erasure was
   * silently abandoned — a button that appeared to do nothing at all. An
   * in-page confirmation behaves identically in the desktop and browser
   * deployments, which is the only way this guard can be trusted.
   */
  requestWipe(): void {
    this.wipeMessage.set('');
    this.wipeFailed.set(false);
    this.confirmingWipe.set(true);
  }

  cancelWipe(): void {
    this.confirmingWipe.set(false);
  }

  /**
   * Erases the discovered estate.
   *
   * Success is reported only once the backend confirms it, and failure is
   * reported just as plainly: telling an operator their data was erased when
   * every record remains is worse than a broken button.
   */
  async wipeAllData(): Promise<void> {
    this.confirmingWipe.set(false);
    this.wiping.set(true);
    this.wipeMessage.set('');
    this.wipeFailed.set(false);
    try {
      await this.wailsIpc.eraseDiscoveredData();
      this.wipeMessage.set('All discovered data has been erased. The engine is ready for a fresh scan.');
    } catch (e) {
      console.error('Trawl: erasing discovered data failed:', e);
      this.wipeFailed.set(true);
      this.wipeMessage.set(
        'The data could not be erased and nothing has been changed. Your discovered data is still present: ' +
        `${e instanceof Error ? e.message : e}`
      );
    } finally {
      this.wiping.set(false);
    }
  }

  // ---------------------------------------------------------------------
  // Discovery: find related domains, then decide on them.
  // ---------------------------------------------------------------------

  proposedDomains = this.wailsIpc.proposedDomains;
  dismissedDomains = this.wailsIpc.dismissedDomains;
  isDiscovering = this.wailsIpc.isDiscovering;
  lastDiscovery = this.wailsIpc.lastDiscovery;

  /** Proposals the operator has ticked, keyed by domain. */
  selectedProposals = signal<Set<string>>(new Set());

  /** Whether the tuning panel is open. Closed by default: the defaults are
   *  the right answer for most operators, and an options panel presented
   *  up-front invites fiddling with settings whose risk is not yet understood. */
  showTuning = signal<boolean>(false);

  discoveryMessage = signal<string>('');
  discoveryFailed = signal<boolean>(false);
  showDismissed = signal<boolean>(false);

  /** Search depth: how many hops of shared-certificate evidence to follow. */
  depth = signal<number>(1);
  /** Ceiling on how many domains a single pass will enumerate. */
  budget = signal<number>(50);
  /** Largest certificate from which shared ownership is inferred. */
  maxSans = signal<number>(25);

  /**
   * The three named presets, so the common cases need no arithmetic.
   *
   * Presets exist because the safe combination is not obvious from the
   * individual numbers: depth 2 with a low SAN ceiling is careful, whereas
   * depth 2 with a high one is not, and an operator should not have to work
   * that out from three sliders.
   */
  readonly presets = [
    {
      id: 'conservative',
      name: 'Conservative',
      summary: 'Only domains sharing a certificate directly with your own.',
      depth: 1, budget: 50, maxSans: 25
    },
    {
      id: 'balanced',
      name: 'Balanced',
      summary: 'Follows one further hop. Finds group structures; needs review.',
      depth: 2, budget: 75, maxSans: 25
    },
    {
      id: 'exhaustive',
      name: 'Exhaustive',
      summary: 'Widest search. Expect unrelated domains in the results.',
      depth: 3, budget: 150, maxSans: 40
    }
  ];

  /** The preset matching the current values, or null when hand-tuned. */
  activePreset = computed(() => {
    const p = this.presets.find(
      x => x.depth === this.depth() && x.budget === this.budget() && x.maxSans === this.maxSans());
    return p ? p.id : null;
  });

  applyPreset(id: string): void {
    const p = this.presets.find(x => x.id === id);
    if (!p) return;
    this.depth.set(p.depth);
    this.budget.set(p.budget);
    this.maxSans.set(p.maxSans);
  }

  /**
   * How risky the current combination is, driving the banner in the panel.
   *
   * Depth and the SAN ceiling compound: each extra hop multiplies the effect
   * of a loose ceiling, because a wrong inference at one hop becomes the
   * starting point for the next.
   */
  riskLevel = computed<'low' | 'moderate' | 'high'>(() => {
    const d = this.depth();
    const s = this.maxSans();
    if (d >= 3 || s >= 60 || (d >= 2 && s >= 40)) return 'high';
    if (d >= 2 || s > 25) return 'moderate';
    return 'low';
  });

  riskExplanation = computed(() => {
    switch (this.riskLevel()) {
      case 'low':
        return 'Every proposal will have shared a certificate directly with a domain ' +
          'you already authorised. False matches are rare, though a genuinely ' +
          'separate estate reached through an intermediary will be missed.';
      case 'moderate':
        return 'Some proposals will be inferred indirectly, or drawn from larger ' +
          'certificates. Expect a handful of domains that are not yours, and read ' +
          'the evidence on each before authorising.';
      case 'high':
        return 'Proposals will include domains related only by several steps of ' +
          'inference, or by sitting on a large shared certificate. Many will belong ' +
          'to other organisations. Review every one individually.';
    }
  });

  /** Whether the run button should warn rather than simply proceed. */
  isHighRisk = computed(() => this.riskLevel() === 'high');

  toggleProposal(domain: string): void {
    this.selectedProposals.update(set => {
      const next = new Set(set);
      if (next.has(domain)) {
        next.delete(domain);
      } else {
        next.add(domain);
      }
      return next;
    });
  }

  isSelected(domain: string): boolean {
    return this.selectedProposals().has(domain);
  }

  selectAllProposals(): void {
    this.selectedProposals.set(new Set(this.proposedDomains().map(p => p.domain)));
  }

  clearSelection(): void {
    this.selectedProposals.set(new Set());
  }

  selectionCount = computed(() => this.selectedProposals().size);

  /**
   * Runs a discovery pass.
   *
   * Nothing discovered here is touched: the pass reads certificate logs and
   * writes a proposal queue. That is what makes it safe to widen the settings
   * and try again — the cost of an over-broad search is a longer review, not
   * traffic sent to somebody else's infrastructure.
   */
  async runDiscovery(): Promise<void> {
    this.discoveryMessage.set('');
    this.discoveryFailed.set(false);
    this.clearSelection();
    try {
      const result = await this.wailsIpc.discoverRelatedDomains({
        depth: this.depth(),
        budget: this.budget(),
        maxSans: this.maxSans()
      });

      const n = result.proposals?.length ?? 0;
      if (n === 0) {
        this.discoveryMessage.set(
          'No related domains were found that you have not already decided upon.');
      } else {
        this.discoveryMessage.set(
          `${n} related ${n === 1 ? 'domain' : 'domains'} found. ` +
          'Nothing has been assessed: review the evidence and authorise what is yours.');
      }
      if (result.budgetExhausted) {
        this.discoveryMessage.update(m =>
          m + ' The search limit was reached, so there may be more to find.');
      }
    } catch (e) {
      console.error('Trawl: discovery failed:', e);
      this.discoveryFailed.set(true);
      this.discoveryMessage.set(
        'The search could not be completed, so this is not a statement that ' +
        `nothing is out there: ${e instanceof Error ? e.message : e}`);
    }
  }

  /** Brings the ticked proposals into scope, making them assessable. */
  async authoriseSelected(): Promise<void> {
    const domains = [...this.selectedProposals()];
    if (domains.length === 0) return;
    this.discoveryFailed.set(false);
    try {
      await this.wailsIpc.authoriseProposedDomains(domains);
      this.clearSelection();
      this.discoveryMessage.set(
        `${domains.length} ${domains.length === 1 ? 'domain is' : 'domains are'} ` +
        'now in scope and will be included in the next scan.');
    } catch (e) {
      console.error('Trawl: authorising domains failed:', e);
      this.discoveryFailed.set(true);
      this.discoveryMessage.set(
        `The domains were not authorised and your scope is unchanged: ${e instanceof Error ? e.message : e}`);
    }
  }

  /** Rules the ticked proposals out so they are not proposed again. */
  async dismissSelected(): Promise<void> {
    const domains = [...this.selectedProposals()];
    if (domains.length === 0) return;
    this.discoveryFailed.set(false);
    try {
      await this.wailsIpc.dismissProposedDomains(domains);
      this.clearSelection();
      this.discoveryMessage.set(
        `${domains.length} ${domains.length === 1 ? 'domain' : 'domains'} dismissed. ` +
        'They will not be proposed again unless you restore them.');
    } catch (e) {
      console.error('Trawl: dismissing domains failed:', e);
      this.discoveryFailed.set(true);
      this.discoveryMessage.set(
        `The domains were not dismissed: ${e instanceof Error ? e.message : e}`);
    }
  }

  async restoreDismissed(domain: string): Promise<void> {
    try {
      await this.wailsIpc.restoreDismissedDomain(domain);
      this.discoveryMessage.set(
        `${domain} has been restored and may be proposed again by the next search.`);
    } catch (e) {
      console.error('Trawl: restoring domain failed:', e);
      this.discoveryFailed.set(true);
      this.discoveryMessage.set(
        `${domain} could not be restored: ${e instanceof Error ? e.message : e}`);
    }
  }

  /** Plain-language confidence wording for a proposal's depth. */
  confidenceLabel(depth: number): string {
    if (depth <= 1) return 'Direct';
    if (depth === 2) return 'Indirect';
    return 'Distant';
  }

  confidenceHint(depth: number): string {
    if (depth <= 1) return 'Shared a certificate directly with a domain you authorised.';
    if (depth === 2) return 'Reached through one intermediate domain. Verify before authorising.';
    return `Reached through ${depth - 1} intermediate domains. Treat with scepticism.`;
  }
}
