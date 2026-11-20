import { ComponentFixture, TestBed } from '@angular/core/testing';
import axe from 'axe-core';
import { vi } from 'vitest';

import { FindingsComponent } from './findings';
import { WailsIpcService } from '../../../wails-ipc.service';
import { detail, run, summary } from '../../../dashboard/findings.fixtures';
import { FindingSummary } from '../../../models/types';

describe('Findings', () => {
  let component: FindingsComponent;
  let fixture: ComponentFixture<FindingsComponent>;
  let ipc: WailsIpcService;
  let root: HTMLElement;

  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  async function load(index: FindingSummary[], state: 'ready' | 'loading' | 'error' = 'ready'): Promise<void> {
    ipc.findingIndex.set(index);
    ipc.findingIndexState.set(state);
    await settle();
  }

  const rows = () => Array.from(root.querySelectorAll<HTMLButtonElement>('[data-finding-key]'));
  const titles = () => rows().map(r => r.querySelector('span.block')?.textContent?.trim());

  beforeEach(async () => {
    window.history.replaceState(null, '', window.location.pathname);
    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({ imports: [FindingsComponent] }).compileComponents();

    ipc = TestBed.inject(WailsIpcService);
    ipc.findings.set([]);
    ipc.scanRuns.set([]);
    vi.spyOn(ipc.transport, 'getFindingDetail').mockResolvedValue(detail());

    fixture = TestBed.createComponent(FindingsComponent);
    component = fixture.componentInstance;
    root = fixture.nativeElement as HTMLElement;
    await fixture.whenStable();
  });

  afterEach(() => {
    window.history.replaceState(null, '', window.location.pathname);
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  describe('the unified list', () => {
    it('lists findings from every source together, without visiting another tab', async () => {
      await load([
        summary({ key: 'k-signal', source: 'signal', title: 'Email authentication gap' }),
        summary({ key: 'k-finding', source: 'finding', title: 'Exposed admin service' }),
        summary({ key: 'k-secret', source: 'secret', title: 'Token in repository' })
      ]);

      expect(rows().map(r => r.dataset['findingKey']).sort()).toEqual(['k-finding', 'k-secret', 'k-signal']);
      const text = root.textContent ?? '';
      expect(text).toContain('Signal');
      expect(text).toContain('Scan finding');
      expect(text).toContain('Secret');
    });

    it('shows active findings by default and everything on request', async () => {
      await load([
        summary({ key: 'open', status: 'open', title: 'Open one' }),
        summary({ key: 'reopened', status: 'reopened', title: 'Reopened one' }),
        summary({ key: 'resolved', status: 'resolved', title: 'Resolved one' })
      ]);
      expect(rows().map(r => r.dataset['findingKey'])).toEqual(expect.arrayContaining(['open', 'reopened']));
      expect(rows().map(r => r.dataset['findingKey'])).not.toContain('resolved');

      component.setFilter('status', 'all');
      await settle();
      expect(rows()).toHaveLength(3);

      component.setFilter('status', 'resolved');
      await settle();
      expect(rows().map(r => r.dataset['findingKey'])).toEqual(['resolved']);
    });

    it('orders rows by severity, then by most recent change', async () => {
      await load([
        summary({ key: 'a', title: 'low recent', severity: 'low', lastChange: '2026-11-19T00:00:00Z' }),
        summary({ key: 'b', title: 'critical old', severity: 'critical', lastChange: '2026-01-01T00:00:00Z' }),
        summary({ key: 'c', title: 'high old', severity: 'high', lastChange: '2026-02-01T00:00:00Z' }),
        summary({ key: 'd', title: 'high new', severity: 'high', lastChange: '2026-11-01T00:00:00Z' })
      ]);
      expect(titles()).toEqual(['critical old', 'high new', 'high old', 'low recent']);
    });

    it('filters to what changed since the last scan', async () => {
      ipc.scanRuns.set([run({ id: 'r2', startedAt: '2026-11-18T00:00:00Z', finishedAt: '2026-11-18T00:05:00Z' })]);
      await load([
        summary({ key: 'changed-1', lastChange: '2026-11-18T00:02:00Z' }),
        summary({ key: 'changed-2', lastChange: '2026-11-19T00:00:00Z' }),
        summary({ key: 'changed-3', lastChange: '2026-11-18T00:00:00Z' }),
        summary({ key: 'older', lastChange: '2026-11-10T00:00:00Z' })
      ]);

      component.setFilter('changedSince', 'last-scan');
      await settle();

      expect(rows().map(r => r.dataset['findingKey']).sort()).toEqual(['changed-1', 'changed-2', 'changed-3']);
    });

    it('does not pretend to filter by "last scan" when no scan is on record', async () => {
      await load([summary({ key: 'k1' })]);
      component.setFilter('changedSince', 'last-scan');
      await settle();

      expect(rows()).toHaveLength(0);
      expect(root.textContent).toContain('No scan has been recorded');
    });

    it('filters by severity, source and asset', async () => {
      await load([
        summary({ key: 'a', severity: 'high', source: 'signal', assetId: 'x', assetValue: 'x.example.com' }),
        summary({ key: 'b', severity: 'low', source: 'finding', assetId: 'y', assetValue: 'y.example.com' })
      ]);
      component.setFilter('severity', 'high');
      await settle();
      expect(rows().map(r => r.dataset['findingKey'])).toEqual(['a']);

      component.resetFilters();
      component.setFilter('source', 'finding');
      await settle();
      expect(rows().map(r => r.dataset['findingKey'])).toEqual(['b']);

      component.resetFilters();
      component.setFilter('asset', 'x');
      await settle();
      expect(rows().map(r => r.dataset['findingKey'])).toEqual(['a']);
    });

    it('has no CVSS column, for which there is no backing data', async () => {
      await load([summary({ cve: 'CVE-2026-0001', source: 'finding' })]);
      expect(root.textContent).not.toMatch(/cvss/i);
      expect(root.textContent).toContain('CVE-2026-0001');
    });

    it('keeps the KEV badge on an exploited finding', async () => {
      await load([summary({ kev: true, cve: 'CVE-2026-0002', source: 'finding' })]);
      expect(root.textContent).toContain('CISA KEV EXPLOITED');
    });
  });

  describe('not re-assessed', () => {
    it('is visibly distinct from an open finding that was re-assessed, and gives the reason', async () => {
      await load([
        summary({ key: 'stale', title: 'Stale one', notReassessed: true, notReassessedReason: 'The DNS lookup failed on the latest run.' }),
        summary({ key: 'fresh', title: 'Fresh one', notReassessed: false })
      ]);

      const stale = root.querySelector<HTMLElement>('[data-finding-key="stale"]')!;
      const fresh = root.querySelector<HTMLElement>('[data-finding-key="fresh"]')!;

      expect(stale.querySelector('[data-testid="not-reassessed"]')?.textContent).toContain('Not re-assessed');
      expect(stale.querySelector('[data-testid="not-reassessed-reason"]')?.textContent).toContain('The DNS lookup failed on the latest run.');
      expect(stale.className).toContain('border-dashed');
      expect(stale.getAttribute('aria-label')).toContain('not re-assessed');

      expect(fresh.querySelector('[data-testid="not-reassessed"]')).toBeNull();
      expect(fresh.className).not.toContain('border-dashed');
      expect(fresh.getAttribute('aria-label')).not.toContain('not re-assessed');
    });

    it('says a reason was not recorded rather than leaving it out', async () => {
      await load([summary({ notReassessed: true, notReassessedReason: '' })]);
      expect(root.querySelector('[data-testid="not-reassessed-reason"]')?.textContent).toContain('No reason was recorded');
    });
  });

  describe('empty and failed states', () => {
    it('does not call the surface clear when nothing is recorded', async () => {
      await load([]);
      const text = root.textContent ?? '';
      expect(text).toContain('No findings have been recorded');
      expect(text).toContain('does not show what has not been checked');
      expect(text).not.toMatch(/\bclear\b|no known exposures|all good|secure/i);
    });

    it('states the coverage of the latest run beside an empty list', async () => {
      ipc.scanRuns.set([run({ coverageFraction: 0.4, coverage: { total: 10, ok: 4, notFound: 0, notChecked: 6, checkFailed: 0, assessedOnly: 4 } })]);
      await load([]);
      expect(root.textContent).toContain('40% of attempted checks concluded (4 of 10)');
    });

    it('says filters hide findings rather than resolve them', async () => {
      await load([summary({ severity: 'low' })]);
      component.setFilter('severity', 'critical');
      await settle();
      expect(root.textContent).toContain('No findings match these filters');
      expect(root.textContent).toContain('They do not resolve them');
    });

    it('warns that an empty list may be a failed load', async () => {
      ipc.findingIndexError.set('boom');
      await load([], 'error');
      const alert = root.querySelector('[role="alert"]');
      expect(alert?.textContent).toContain('could not be loaded');
      expect(alert?.textContent).toContain('not evidence that nothing was found');
    });

    it('shows loading rather than "no findings" while the first read is in flight', async () => {
      await load([], 'loading');
      expect(root.textContent).toContain('Loading findings');
      expect(root.textContent).not.toContain('No findings have been recorded');
    });
  });

  describe('keyboard and focus', () => {
    it('makes every row a real button with an accessible name', async () => {
      await load([summary({ key: 'k1', title: 'Something' }), summary({ key: 'k2', title: 'Else' })]);
      for (const row of rows()) {
        expect(row.tagName).toBe('BUTTON');
        expect(row.getAttribute('type')).toBe('button');
        expect(row.getAttribute('aria-label')).toMatch(/Open details\.$/);
        expect(row.tabIndex).toBeGreaterThanOrEqual(0);
        expect(row.className).toContain('focus-visible:outline');
      }
    });

    it('opens the detail from a row, giving it an address', async () => {
      await load([summary({ key: 'k1', title: 'Something' })]);

      rows()[0].click();
      await settle();

      expect(ipc.selectedFindingKey()).toBe('k1');
      expect(window.location.hash).toBe('#/findings/k1');
      expect(root.querySelector('app-finding-detail')).not.toBeNull();
      // The list is replaced, not covered.
      expect(rows()).toHaveLength(0);
    });

    it('returns focus to the row that opened the detail when it closes', async () => {
      await load([summary({ key: 'k1', title: 'One' }), summary({ key: 'k2', title: 'Two', severity: 'low' })]);

      const second = root.querySelector<HTMLButtonElement>('[data-finding-key="k2"]')!;
      second.focus();
      second.click();
      await settle();
      expect(ipc.selectedFindingKey()).toBe('k2');

      ipc.closeFinding();
      await settle();

      const returned = root.querySelector<HTMLButtonElement>('[data-finding-key="k2"]')!;
      expect(returned).not.toBeNull();
      expect(document.activeElement).toBe(returned);
    });

    it('closes the detail on Escape and lands back on the row', async () => {
      await load([summary({ key: 'k1', title: 'One' })]);
      rows()[0].click();
      await settle();
      expect(root.querySelector('app-finding-detail')).not.toBeNull();

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      await settle();

      expect(ipc.selectedFindingKey()).toBeNull();
      expect(document.activeElement).toBe(root.querySelector('[data-finding-key="k1"]'));
    });

    it('opens a deep-linked finding even though no row was clicked', async () => {
      await load([summary({ key: 'k1', title: 'One' })]);
      ipc.activeTab.set('findings');
      ipc.selectedFindingKey.set('k1');
      await settle();
      expect(root.querySelector('app-finding-detail')).not.toBeNull();
    });
  });

  describe('software candidates', () => {
    it('filters to candidates and renders an explicit unconfirmed text label', async () => {
      await load([
        summary({ key: 'sig-1', title: 'Weak DMARC', source: 'signal', severity: 'medium' }),
        summary({
          key: 'cand-1',
          title: 'CVE-2021-23017 (nginx 1.18.0, unconfirmed)',
          source: 'candidate',
          unconfirmed: true,
          cve: 'CVE-2021-23017',
          kev: true,
          severity: 'high',
          priority: 'critical'
        })
      ]);

      fixture.componentInstance.setFilter('source', 'candidate');
      await settle();

      expect(rows()).toHaveLength(1);
      const badge = root.querySelector('[data-testid="unconfirmed-badge"]');
      expect(badge?.textContent).toContain('Unconfirmed candidate');
      expect(rows()[0].textContent).toContain('Software candidate (unconfirmed)');
    });
  });

  describe('accessibility', () => {
    it('has no detectable WCAG 2.1 AA violations, apart from contrast which jsdom cannot compute', async () => {
      await load([
        summary({ key: 'k1', title: 'One', kev: true, cve: 'CVE-2026-0001', source: 'finding' }),
        summary({ key: 'k2', title: 'Two', notReassessed: true, notReassessedReason: 'check failed' })
      ]);
      const results = await axe.run(root, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] },
        rules: { 'color-contrast': { enabled: false } }
      });
      expect(results.violations.map(v => `${v.id}: ${v.nodes.length} node(s)`)).toEqual([]);
    });
  });
});
