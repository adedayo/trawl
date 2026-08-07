import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EmailPostureComponent } from './email-posture';

describe('EmailPosture', () => {
  let component: EmailPostureComponent;
  let fixture: ComponentFixture<EmailPostureComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [EmailPostureComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(EmailPostureComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  /**
   * Fixtures are built by hand rather than borrowed from a factory so that the
   * shape each assertion depends on is visible at the assertion.
   */
  const coverage = (total: number, ok: number) => ({
    total, ok, assessedOnly: ok, notFound: 0, notChecked: total - ok, checkFailed: 0
  });

  const control = (name: string, posture: any, signals: any[] = []) => ({
    control: name, posture, coverage: coverage(2, 2), checks: [], signals
  });

  const assessment = (controls: any[], outcome = 'completed', total = 10, ok = 10) => ({
    assetId: 'a', domain: 'd.example', outcome,
    coverage: coverage(total, ok), coverageFraction: total ? ok / total : 0,
    controls, scenarios: [], unmapped: [], registryVersion: 'v1', libraryVersion: '1.0'
  }) as any;

  describe('collapsed row summary', () => {
    // A domain that was never assessed for DKIM and one assessed clean must
    // not look alike. Omitting the missing control would make them identical
    // at a glance, which is the confusion the four-state model exists to stop.
    it('shows every headline control, marking absent ones unknown', () => {
      const a = assessment([control('spf', 'compliant')]);
      const summary = component.summaryControls(a);

      expect(summary.map(c => c.control)).toEqual(['spf', 'dkim', 'dmarc', 'mta-sts']);
      expect(summary.find(c => c.control === 'spf')!.posture).toBe('compliant');
      expect(summary.find(c => c.control === 'dkim')!.posture).toBe('unknown');
    });

    // The row's badge draws the eye, so it must count only what warrants it.
    // Counting low and informational advisories there would make every domain
    // look urgent and the badge would stop meaning anything.
    it('counts only medium-and-above advisories as needing review', () => {
      const a = assessment([
        control('mx', 'deficient', [{ severity: 'low' }, { severity: 'info' }]),
        control('spf', 'deficient', [{ severity: 'high' }])
      ]);

      expect(component.advisoryCount(a)).toBe(3);
      expect(component.significantCount(a)).toBe(1);
    });

    // "No advisory raised" over two checks is a far weaker claim than the same
    // words over twenty. The summary states the coverage so the reader can
    // tell which one they are being given.
    it('states coverage alongside a clean result', () => {
      const line = component.summaryLine(assessment([control('spf', 'compliant')], 'completed', 20, 18));
      expect(line).toContain('18/20');
    });

    it('distinguishes never-assessed from assessed-clean', () => {
      const line = component.summaryLine(assessment([], 'completed', 0, 0));
      expect(line).toContain('Nothing has been assessed');
    });

    // A refusal is not a finding about the domain, it is a fact about the
    // scope. It must displace the advisory summary rather than sit beside it.
    it('reports the outcome instead of a tally when nothing was queried', () => {
      const a = assessment([], 'refused', 0, 0);
      a.error = 'outside the authorised scope';
      expect(component.summaryLine(a)).toContain('outside the authorised scope');
    });
  });

  describe('expanding rows', () => {
    it('opens and closes a single domain', () => {
      expect(component.isOpen('d.example')).toBe(false);
      component.toggleDomain('d.example');
      expect(component.isOpen('d.example')).toBe(true);
      component.toggleDomain('d.example');
      expect(component.isOpen('d.example')).toBe(false);
    });
  });

  describe('scenario chips', () => {
    const scenario = (over: Partial<any> = {}) => ({
      scenario: 'email-interception', coverage: coverage(9, 9),
      aggravating: 2, significant: 1, mitigating: 3, supported: true, ...over
    });

    // Two numbers on one chip are ambiguous without being spelled out, and
    // the minor findings must remain visible rather than being filtered away
    // by the headline figure.
    it('accounts for both the weighty and the minor groups', () => {
      const hint = component.scenarioHint(scenario());
      expect(hint).toContain('1 weakness group(s) at medium severity or above');
      expect(hint).toContain('1 raising only low or informational');
      expect(hint).toContain('3 assessed clean');
    });

    // A count with no coverage behind it is unquantified, not reassuring.
    it('states the coverage the counts are drawn from', () => {
      expect(component.scenarioHint(scenario())).toContain('9/9 checks concluded');
    });

    it('omits the minor clause when every group is weighty', () => {
      const hint = component.scenarioHint(scenario({ aggravating: 1, significant: 1 }));
      expect(hint).not.toContain('low or informational');
    });

    // An unassessed scenario must never read as a low-risk one.
    it('says so when nothing bears on the scenario', () => {
      expect(component.scenarioHint(scenario({ supported: false })))
        .toContain('No assessment bears on this scenario');
    });
  });
});
