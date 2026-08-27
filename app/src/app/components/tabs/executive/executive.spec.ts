import { ComponentFixture, TestBed } from '@angular/core/testing';

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
});
