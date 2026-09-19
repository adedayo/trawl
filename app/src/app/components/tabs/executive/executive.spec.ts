import { ComponentFixture, TestBed } from '@angular/core/testing';
import axe from 'axe-core';

import { ExecutiveComponent } from './executive';
import { WailsIpcService } from '../../../wails-ipc.service';
import { DomainAssessment } from '../../../models/types';

/**
 * The derivation is proven in dashboard/estate.spec.ts. What is proven here is
 * narrower and, for this component, more important: that the suppression
 * survives rendering.
 *
 * A pure function returning `available: false` is worth nothing if the
 * template goes on to print a score anyway from some other expression. That is
 * a plausible regression — someone adds a "just show it greyed out" branch —
 * and it is invisible to a unit test of the function alone. So this asserts on
 * the DOM.
 */
describe('ExecutiveComponent', () => {
  let fixture: ComponentFixture<ExecutiveComponent>;
  let ipc: WailsIpcService;

  function assessment(over: Partial<DomainAssessment>): DomainAssessment {
    return {
      domain: 'example.com',
      outcome: 'completed',
      controls: [],
      ...over
    } as unknown as DomainAssessment;
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ExecutiveComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ExecutiveComponent);
    ipc = TestBed.inject(WailsIpcService);
    ipc.isAuthorized.set(true);
    // An estate must exist, or the component correctly shows its empty state
    // instead of any figure at all.
    ipc.assets.set([
      { id: 'a-1', value: 'example.com', status: 'active' } as unknown as never
    ]);
    // The store has been read. Left at its default the component would
    // correctly show skeletons instead of any figure.
    ipc.loadState.set('ready');
    ipc.lastUpdatedAt.set(new Date().toISOString());
    ipc.streamHealthy.set(true);
  });

  it('renders no headline score when coverage is below the floor', async () => {
    ipc.assessments.set([
      assessment({
        coverage: { total: 100, ok: 10, notFound: 0, notChecked: 90, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);
    fixture.detectChanges();
    await fixture.whenStable();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

    expect(fixture.componentInstance.posture().available).toBe(false);
    // The refusal must be stated, and the reason with it.
    expect(text).toContain('Not enough coverage to report');
    // And the denominator must be present wherever the shortfall is quoted.
    expect(text).toContain('10/100');
  });

  it('shows the figure with its coverage once the floor is met', async () => {
    ipc.assessments.set([
      assessment({
        coverage: { total: 10, ok: 6, notFound: 2, notChecked: 2, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);
    fixture.detectChanges();
    await fixture.whenStable();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

    expect(fixture.componentInstance.posture().available).toBe(true);
    expect(text).toContain('75');
    // Coverage travels with the score rather than beside it.
    expect(text).toContain('80% assessed (8/10 checks)');
  });

  it('states no monetary value and no probability of compromise', async () => {
    ipc.assessments.set([
      assessment({
        coverage: { total: 10, ok: 9, notFound: 1, notChecked: 0, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);
    fixture.detectChanges();
    await fixture.whenStable();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

    // Guardrail 6: those figures need the loss model of Changes 008-011, and
    // inventing them here would be a number with no calibration behind it.
    // Asserting on the absence of a *quantity* rather than of the words,
    // because the boundary note has to be free to name what it is denying.
    expect(text).not.toMatch(/[£$€]\s?[\d,]+/);
    expect(text).not.toMatch(/\d+(\.\d+)?\s?%\s*(likelihood|chance|probability|risk of)/i);
    expect(text).toContain('It states no expected loss and no probability of compromise');
  });

  describe('four-state coverage', () => {
    /**
     * The interface counterpart to the store-level test in 006 Phase 5.
     *
     * "We tried and could not tell" is the state most likely to be quietly
     * rendered as a pass, because it produced no adverse result. It carries
     * the same styling vocabulary as a healthy control unless someone stops
     * it, and nobody notices, because the screen looks fine.
     */
    it('never renders check_failed in passing styling', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 7, notFound: 0, notChecked: 0, checkFailed: 3 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      const el = fixture.nativeElement as HTMLElement;
      const chip = Array.from(el.querySelectorAll('span')).find(s =>
        (s.textContent ?? '').includes('could not tell')
      );

      expect(chip).toBeTruthy();
      const cls = chip!.className;
      // Emerald is this view's vocabulary for "sound". An inconclusive check
      // must never borrow it.
      expect(cls).not.toContain('emerald');
      expect(cls).not.toContain('green');
      expect(cls).toContain('amber');
    });

    it('shows unattempted and inconclusive checks even when a score is displayed', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 6, notFound: 1, notChecked: 2, checkFailed: 1 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

      // A score is on screen — which is precisely when a gap left unstated
      // would flatter it.
      expect(fixture.componentInstance.posture().available).toBe(true);
      expect(text).toContain('1 could not tell');
      expect(text).toContain('2 not checked');
    });

    it('distinguishes coverage states by more than colour', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 6, notFound: 1, notChecked: 2, checkFailed: 1 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      const el = fixture.nativeElement as HTMLElement;
      const spans = Array.from(el.querySelectorAll('span'));
      const failed = spans.find(s => (s.textContent ?? '').includes('could not tell'));
      const unchecked = spans.find(s => (s.textContent ?? '').includes('not checked'));

      // Each state names itself in words, so it survives greyscale printing
      // and colour-blindness. A board pack is very often printed.
      expect(failed!.textContent).toContain('could not tell');
      expect(unchecked!.textContent).toContain('not checked');
      // And the two differ in border treatment as well as hue.
      expect(unchecked!.className).toContain('border-dashed');
      expect(failed!.className).not.toContain('border-dashed');
    });

    it('gives the coverage bar a text alternative, since a bar has no accessible value', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 8, notFound: 0, notChecked: 2, checkFailed: 0 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      const bar = (fixture.nativeElement as HTMLElement).querySelector('[role="img"]');
      expect(bar?.getAttribute('aria-label')).toContain('80% assessed (8/10 checks)');
    });
  });

  describe('freshness', () => {
    /**
     * A dashboard that stops updating without saying so is worse than one that
     * is obviously broken: it goes on looking authoritative while describing a
     * past that no longer holds.
     */
    it('says the figures may be stale once the stream drops', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 8, notFound: 1, notChecked: 1, checkFailed: 0 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      let text = (fixture.nativeElement as HTMLElement).textContent ?? '';
      expect(text).toContain('Live');
      expect(text).not.toContain('may be out of date');

      ipc.streamHealthy.set(false);
      fixture.detectChanges();
      await fixture.whenStable();

      text = (fixture.nativeElement as HTMLElement).textContent ?? '';
      expect(text).toContain('Not live');
      expect(text).toContain('Figures below may be out of date');
    });
  });

  describe('load states', () => {
    it('shows skeletons before the store has ever been read, not an empty estate', async () => {
      ipc.loadState.set('loading');
      ipc.lastUpdatedAt.set(null);
      fixture.detectChanges();
      await fixture.whenStable();

      const el = fixture.nativeElement as HTMLElement;
      const text = el.textContent ?? '';

      expect(el.querySelector('[aria-busy="true"]')).toBeTruthy();
      // "We have not looked yet" must not be dressed as "we looked and found
      // nothing".
      expect(text).not.toContain('No estate discovered yet');
      expect(text).not.toContain('Not enough coverage to report');
    });

    it('keeps figures on screen during a later refresh rather than reverting to skeletons', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 8, notFound: 1, notChecked: 1, checkFailed: 0 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      ipc.loadState.set('loading');
      fixture.detectChanges();
      await fixture.whenStable();

      // `lastUpdatedAt` is set, so this is a refresh, not a first load. A
      // slightly stale number tells the reader more than a shimmer does.
      expect(fixture.componentInstance.isLoading()).toBe(false);
      expect((fixture.nativeElement as HTMLElement).textContent).toContain('89');
    });

    it('reports a failed load distinctly from an empty one', async () => {
      ipc.loadState.set('error');
      ipc.loadError.set('Could not reach the engine.');
      fixture.detectChanges();
      await fixture.whenStable();

      const el = fixture.nativeElement as HTMLElement;
      const text = el.textContent ?? '';

      expect(el.querySelector('[role="alert"]')).toBeTruthy();
      expect(text).toContain('Some figures could not be loaded');
      // The distinction that matters: a failure must never read as an
      // all-clear.
      expect(text).toContain('incomplete rather than as an all-clear');
    });
  });

  it('renders completely with no AI provider configured', async () => {
    ipc.assessments.set([
      assessment({
        coverage: { total: 10, ok: 8, notFound: 1, notChecked: 1, checkFailed: 0 }
      } as unknown as Partial<DomainAssessment>)
    ]);
    fixture.detectChanges();
    await fixture.whenStable();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

    // No provider is configured in this fixture, and every section is still
    // present. AI annotation is advisory throughout; nothing load-bearing may
    // depend on it.
    expect(text).toContain('Assessed control posture');
    expect(text).toContain('What to deal with first');
    expect(text).toContain('What changed for the worse');
    expect(text).not.toMatch(/undefined|\[object Object\]/);
  });

  describe('accessibility', () => {
    /**
     * Automated WCAG 2.1 AA scanning.
     *
     * This catches the structural half of accessibility — unlabelled
     * controls, broken heading order, missing text alternatives — which is the
     * half that regresses silently as markup is edited. It does not catch the
     * judgement half, and passing it is not a claim that the view is
     * accessible, only that it has not acquired a defect a machine can see.
     *
     * Colour contrast is excluded because jsdom has no layout or paint: axe
     * cannot compute a contrast ratio here and would report false passes,
     * which is worse than reporting nothing. Contrast needs a real browser.
     */
    it('has no detectable WCAG 2.1 AA violations', async () => {
      ipc.assessments.set([
        assessment({
          coverage: { total: 10, ok: 6, notFound: 1, notChecked: 2, checkFailed: 1 }
        } as unknown as Partial<DomainAssessment>)
      ]);
      fixture.detectChanges();
      await fixture.whenStable();

      const results = await axe.run(fixture.nativeElement as HTMLElement, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] },
        rules: { 'color-contrast': { enabled: false } }
      });

      expect(
        results.violations.map(v => `${v.id}: ${v.nodes.length} node(s)`)
      ).toEqual([]);
    });
  });
});
