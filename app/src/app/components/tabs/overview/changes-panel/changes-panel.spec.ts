import { ComponentFixture, TestBed } from '@angular/core/testing';
import axe from 'axe-core';
import { vi } from 'vitest';

import { ChangesPanelComponent } from './changes-panel';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { change, changeSet, point, run, summary } from '../../../../dashboard/findings.fixtures';

describe('ChangesPanel', () => {
  let fixture: ComponentFixture<ChangesPanelComponent>;
  let ipc: WailsIpcService;
  let root: HTMLElement;

  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  const text = () => root.textContent ?? '';
  const bucket = (name: string) => root.querySelector<HTMLElement>(`[data-testid="changes-panel"] [data-bucket="${name}"]`)!;

  const earlier = point({ runId: 'r1', at: '2026-11-01T10:00:00Z', coverageFraction: 0.9 });
  const later = (fraction: number) => point({ runId: 'r2', at: '2026-11-08T10:00:00Z', coverageFraction: fraction });

  beforeEach(async () => {
    window.history.replaceState(null, '', window.location.pathname);
    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({ imports: [ChangesPanelComponent] }).compileComponents();
    ipc = TestBed.inject(WailsIpcService);
    ipc.scanRuns.set([]);
    ipc.scanRunsState.set('ready');
    fixture = TestBed.createComponent(ChangesPanelComponent);
    root = fixture.nativeElement as HTMLElement;
    await settle();
  });

  afterEach(() => {
    window.history.replaceState(null, '', window.location.pathname);
  });

  describe('since the last scan', () => {
    it('counts and lists new, resolved, worsened and improved findings, each labelled', async () => {
      ipc.changeSet.set(changeSet({
        from: earlier,
        to: later(0.9),
        new: [change({ finding: summary({ key: 'n1', title: 'New thing' }) }), change({ finding: summary({ key: 'n2', title: 'Another new thing' }) })],
        resolved: [change({ finding: summary({ key: 'r1', title: 'Fixed thing' }) })],
        worsened: [],
        improved: [change({ finding: summary({ key: 'i1', title: 'Better thing' }) })]
      }));
      ipc.changeSetState.set('ready');
      await settle();

      const counts = (name: string) => bucket(name).querySelector('[data-testid="bucket-count"]')?.textContent?.trim();
      expect(counts('new')).toBe('2 findings');
      expect(counts('resolved')).toBe('1 finding');
      expect(counts('worsened')).toBe('0 findings');
      expect(counts('improved')).toBe('1 finding');
      expect(bucket('new').textContent).toContain('New thing');
      expect(bucket('worsened').textContent).toContain('None recorded between these scans');
    });

    it('opens a finding from its entry', async () => {
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.9), new: [change({ finding: summary({ key: 'n1', title: 'New thing' }) })] }));
      ipc.changeSetState.set('ready');
      await settle();

      const button = bucket('new').querySelector<HTMLButtonElement>('button')!;
      expect(button.getAttribute('aria-label')).toContain('New thing');
      button.click();

      expect(ipc.selectedFindingKey()).toBe('n1');
      expect(ipc.activeTab()).toBe('findings');
    });

    it('shows five entries and offers the rest', async () => {
      const many = Array.from({ length: 8 }, (_, i) => change({ finding: summary({ key: `k${i}`, title: `Finding ${i}` }) }));
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.9), new: many }));
      ipc.changeSetState.set('ready');
      await settle();

      expect(bucket('new').querySelectorAll('ul button')).toHaveLength(5);
      const toggle = Array.from(bucket('new').querySelectorAll('button')).find(b => b.textContent?.includes('Show all 8'))!;
      toggle.click();
      await settle();
      expect(bucket('new').querySelectorAll('ul button')).toHaveLength(8);
      expect(bucket('new').textContent).toContain('Show fewer');
    });

    it('states the runs compared and the coverage of each', async () => {
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.6) }));
      ipc.changeSetState.set('ready');
      await settle();

      expect(root.querySelector('[data-testid="compared-line"]')?.textContent).toContain('2026-11-01 10:00 UTC compared with 2026-11-08 10:00 UTC');
      expect(text()).toContain('Earlier scan: 90% of attempted checks concluded');
      expect(text()).toContain('Later scan: 60% of attempted checks concluded');
    });

    it('says so beside every count when coverage fell', async () => {
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.5), resolved: [change(), change()] }));
      ipc.changeSetState.set('ready');
      await settle();

      expect(root.querySelector('[data-testid="coverage-drop"]')?.textContent).toContain('Coverage fell from 90% to 50%');
      const chips = root.querySelectorAll('[data-testid="bucket-coverage-drop"]');
      expect(chips).toHaveLength(4);
      // The chip sits in the same header row as the count it qualifies.
      const header = bucket('resolved').querySelector('[data-testid="bucket-count"]')!.parentElement!;
      expect(header.querySelector('[data-testid="bucket-coverage-drop"]')).not.toBeNull();
    });

    it('says nothing about coverage when it held', async () => {
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.9) }));
      ipc.changeSetState.set('ready');
      await settle();
      expect(root.querySelector('[data-testid="coverage-drop"]')).toBeNull();
      expect(root.querySelector('[data-testid="bucket-coverage-drop"]')).toBeNull();
    });

    it("shows the engine's note verbatim", async () => {
      const note = 'The later run attempted 12 fewer checks than the earlier one; counts are not like for like.';
      ipc.changeSet.set(changeSet({ from: earlier, to: later(0.9), note }));
      ipc.changeSetState.set('ready');
      await settle();
      expect(root.querySelector('[data-testid="change-note"]')?.textContent?.trim()).toBe(note);
    });

    it('does not present the first scan as a change', async () => {
      ipc.changeSet.set(changeSet({ to: later(0.9), new: [change()] }));
      ipc.changeSetState.set('ready');
      await settle();
      expect(root.querySelector('[data-testid="no-baseline"]')?.textContent).toContain('no earlier scan');
    });

    it('says when no scan has been recorded', async () => {
      ipc.changeSet.set(changeSet());
      ipc.changeSetState.set('ready');
      await settle();
      expect(root.querySelector('[data-testid="no-runs"]')).not.toBeNull();
    });

    it('warns that empty lists may be a failed load', async () => {
      ipc.changeSetState.set('error');
      await settle();
      const message = root.querySelector('[data-testid="changes-error"]')?.textContent ?? '';
      expect(message).toContain('could not be loaded');
      expect(message).toContain('not evidence that nothing changed');
    });
  });

  describe('compare scans', () => {
    const runs = [
      run({ id: 'r1', startedAt: '2026-11-01T09:00:00Z', finishedAt: '2026-11-01T10:00:00Z', coverageFraction: 0.9 }),
      run({ id: 'r2', startedAt: '2026-11-08T09:00:00Z', finishedAt: '2026-11-08T10:00:00Z', coverageFraction: 0.6 }),
      run({ id: 'r3', startedAt: '2026-11-15T09:00:00Z', finishedAt: '2026-11-15T10:00:00Z', coverageFraction: 0.7 })
    ];

    const choose = async (from: string, to: string) => {
      const select = (id: string) => root.querySelector<HTMLSelectElement>(`#${id}`)!;
      select('compare-from').value = from;
      select('compare-from').dispatchEvent(new Event('change'));
      select('compare-to').value = to;
      select('compare-to').dispatchEvent(new Event('change'));
      await settle();
    };

    beforeEach(async () => {
      ipc.scanRuns.set(runs);
      await settle();
    });

    it('offers the runs from the ledger, newest first, with their coverage', () => {
      const options = Array.from(root.querySelectorAll<HTMLOptionElement>('#compare-from option')).map(o => o.textContent?.trim());
      expect(options[0]).toBe('Choose a scan');
      expect(options[1]).toContain('2026-11-15');
      expect(options[1]).toContain('70% coverage');
      expect(options).toHaveLength(4);
    });

    it('is not available until two different scans are chosen, and says why', async () => {
      const button = root.querySelector<HTMLButtonElement>('[data-testid="compare-button"]')!;
      expect(button.disabled).toBe(true);
      expect(root.querySelector('[data-testid="compare-hint"]')?.textContent).toContain('Choose two scans');

      await choose('r1', 'r1');
      expect(button.disabled).toBe(true);
      expect(root.querySelector('[data-testid="compare-hint"]')?.textContent).toContain('two different scans');

      await choose('r3', 'r1');
      expect(button.disabled).toBe(true);
      expect(root.querySelector('[data-testid="compare-hint"]')?.textContent).toContain('earlier scan first');
    });

    it('compares the two chosen runs and shows the result with its coverage', async () => {
      const spy = vi.spyOn(ipc.transport, 'getFindingChanges').mockResolvedValue(changeSet({
        from: point({ runId: 'r1', at: '2026-11-01T10:00:00Z', coverageFraction: 0.9 }),
        to: point({ runId: 'r2', at: '2026-11-08T10:00:00Z', coverageFraction: 0.6 }),
        worsened: [change({ finding: summary({ key: 'w1', title: 'Got worse' }) })]
      }));
      await choose('r1', 'r2');

      root.querySelector<HTMLButtonElement>('[data-testid="compare-button"]')!.click();
      await settle();

      expect(spy).toHaveBeenCalledWith('r1', 'r2');
      const result = root.querySelector('[data-testid="comparison"]')!;
      expect(result.textContent).toContain('Got worse');
      expect(result.textContent).toContain('Coverage fell from 90% to 60%');
    });

    it('reports a failed comparison instead of an empty one', async () => {
      vi.spyOn(ipc.transport, 'getFindingChanges').mockRejectedValue(new Error('503'));
      await choose('r1', 'r2');
      root.querySelector<HTMLButtonElement>('[data-testid="compare-button"]')!.click();
      await settle();

      expect(root.querySelector('[data-testid="comparison-error"]')?.textContent).toContain('could not be loaded');
      expect(root.querySelector('[data-testid="comparison"]')).toBeNull();
    });
  });

  describe('accessibility', () => {
    it('has no detectable WCAG 2.1 AA violations, apart from contrast which jsdom cannot compute', async () => {
      ipc.scanRuns.set([run({ id: 'r1' }), run({ id: 'r2', startedAt: '2026-11-08T09:00:00Z' })]);
      ipc.changeSet.set(changeSet({
        from: earlier,
        to: later(0.5),
        note: 'A note.',
        new: [change()],
        improved: [change({ finding: summary({ key: 'i1' }) })]
      }));
      ipc.changeSetState.set('ready');
      await settle();

      const results = await axe.run(root, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] },
        rules: { 'color-contrast': { enabled: false } }
      });
      expect(results.violations.map(v => `${v.id}: ${v.nodes.length} node(s)`)).toEqual([]);
    });
  });
});
