import { ComponentFixture, TestBed } from '@angular/core/testing';
import axe from 'axe-core';
import { vi } from 'vitest';

import { TrendChartComponent } from './trend-chart';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { point } from '../../../../dashboard/findings.fixtures';

const DAY = 86_400_000;
const T0 = Date.parse('2026-11-01T00:00:00Z');
const at = (days: number) => new Date(T0 + days * DAY).toISOString();

describe('TrendChart', () => {
  let fixture: ComponentFixture<TrendChartComponent>;
  let ipc: WailsIpcService;
  let root: HTMLElement;

  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  async function show(history: ReturnType<typeof point>[]): Promise<void> {
    ipc.history.set(history);
    ipc.historyState.set('ready');
    await settle();
  }

  const text = () => root.textContent ?? '';

  beforeEach(async () => {
    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({ imports: [TrendChartComponent] }).compileComponents();
    ipc = TestBed.inject(WailsIpcService);
    fixture = TestBed.createComponent(TrendChartComponent);
    root = fixture.nativeElement as HTMLElement;
    await settle();
  });

  it('says there is no history rather than drawing an empty chart', async () => {
    await show([]);
    expect(root.querySelector('svg')).toBeNull();
    expect(root.querySelector('[data-testid="trend-empty"]')?.textContent).toContain('No scan summaries are recorded');
  });

  it('warns that a missing chart may be a failed load', async () => {
    ipc.historyState.set('error');
    await settle();
    expect(root.querySelector('[data-testid="trend-error"]')?.textContent).toContain('not evidence that nothing changed');
  });

  it('draws one marker per scan per severity, at real times', async () => {
    await show([point({ runId: 'a', at: at(0) }), point({ runId: 'b', at: at(1) }), point({ runId: 'c', at: at(2) })]);
    const groups = root.querySelectorAll('svg g[data-run-index]');
    expect(groups).toHaveLength(3);
    // Five severities per scan.
    expect(groups[0].querySelectorAll('path')).toHaveLength(5);
  });

  it('draws a ten-day silence as a labelled gap and nothing across it', async () => {
    await show([
      point({ runId: 'a', at: at(0) }), point({ runId: 'b', at: at(1) }), point({ runId: 'c', at: at(2) }), point({ runId: 'd', at: at(3) }),
      point({ runId: 'e', at: at(13) }), point({ runId: 'f', at: at(14) })
    ]);

    const gaps = root.querySelectorAll('[data-testid="trend-gap"]');
    expect(gaps).toHaveLength(1);
    expect(gaps[0].textContent).toContain('No scan recorded for 10 days');

    // Each severity is stroked in two pieces, one either side of the gap: the
    // run of four scans and the run of two, never a single line through both.
    const strokes = root.querySelectorAll('svg path[fill="none"]');
    expect(strokes).toHaveLength(5 * 2);
    for (const stroke of Array.from(strokes)) {
      expect(((stroke.getAttribute('d') ?? '').match(/M/g) ?? []).length).toBe(1);
    }
  });

  it('draws no connecting stroke between scans that are far apart and few', async () => {
    await show([point({ runId: 'a', at: at(0) }), point({ runId: 'b', at: at(40) })]);
    expect(root.querySelectorAll('svg path[fill="none"]')).toHaveLength(0);
    expect(root.querySelectorAll('[data-testid="trend-gap"]')).toHaveLength(1);
  });

  it('shows the coverage of every scan beside the counts, and marks a thin one', async () => {
    await show([
      point({ runId: 'a', at: at(0), coverageFraction: 0.9, high: 4 }),
      point({ runId: 'b', at: at(1), coverageFraction: 0.3, high: 0, medium: 0, low: 0, open: 0 }),
      point({ runId: 'c', at: at(2), coverageFraction: 0.9, high: 4 })
    ]);

    const bars = Array.from(root.querySelectorAll('[data-testid="coverage-bar"]'));
    expect(bars).toHaveLength(3);
    expect(bars.map(b => b.getAttribute('data-thin'))).toEqual(['false', 'true', 'false']);

    // The tooltip for the thin run carries its coverage and the warning.
    const tooltip = root.querySelector('svg g[data-run-index="1"] title')?.textContent ?? '';
    expect(tooltip).toContain('30%');
    expect(tooltip).toContain('less being checked');

    expect(root.querySelector('[data-testid="thin-note"]')?.textContent).toContain('not a better estate');
  });

  it('gives every run a coverage figure in the text table, and a gap row where there was a gap', async () => {
    await show([
      point({ runId: 'a', at: at(0), coverageFraction: 0.9 }), point({ runId: 'b', at: at(1), coverageFraction: 0.4 }),
      point({ runId: 'c', at: at(2), coverageFraction: 0.9 }), point({ runId: 'd', at: at(30), coverageFraction: 0.9 })
    ]);
    const table = root.querySelector('[data-testid="trend-table"]')!;
    const rows = Array.from(table.querySelectorAll('tbody tr'));
    expect(rows).toHaveLength(5);
    expect(rows[1].textContent).toContain('40% of attempted checks concluded');
    expect(rows[1].textContent).toContain('lower than the previous scan');
    expect(rows[3].getAttribute('data-testid')).toBe('trend-table-gap');
    expect(rows[3].textContent).toContain('No scan recorded for 28 days');
    expect(table.querySelector('caption')?.textContent).toContain('Open findings by severity');
  });

  it('describes the drawing in words for assistive technology', async () => {
    await show([point({ runId: 'a', at: at(0), open: 6 }), point({ runId: 'b', at: at(1), open: 5 })]);
    const svg = root.querySelector('svg')!;
    expect(svg.getAttribute('role')).toBe('img');
    expect(svg.querySelector('desc')?.textContent).toContain('2 recorded scans');
    expect(svg.querySelector('desc')?.textContent).toContain('A table with the same data follows');
  });

  it('reloads the history when the range changes', async () => {
    const spy = vi.spyOn(ipc.transport, 'getHistorySummary').mockResolvedValue([]);
    await fixture.componentInstance.setRange('30');
    expect(spy).toHaveBeenCalledTimes(1);
    const since = Date.parse(spy.mock.calls[0][0] as string);
    expect(Date.now() - since).toBeGreaterThan(29 * DAY);
    expect(Date.now() - since).toBeLessThan(31 * DAY);

    await fixture.componentInstance.setRange('all');
    expect(spy).toHaveBeenLastCalledWith('');
  });

  it('says a single scan is not yet a trend', async () => {
    await show([point({ runId: 'a', at: at(0) })]);
    expect(text()).toContain('Only one scan is recorded');
  });

  describe('accessibility', () => {
    it('has no detectable WCAG 2.1 AA violations, apart from contrast which jsdom cannot compute', async () => {
      await show([
        point({ runId: 'a', at: at(0) }), point({ runId: 'b', at: at(1), coverageFraction: 0.2 }),
        point({ runId: 'c', at: at(2) }), point({ runId: 'd', at: at(30) })
      ]);
      const results = await axe.run(root, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] },
        rules: { 'color-contrast': { enabled: false } }
      });
      expect(results.violations.map(v => `${v.id}: ${v.nodes.length} node(s)`)).toEqual([]);
    });
  });
});
