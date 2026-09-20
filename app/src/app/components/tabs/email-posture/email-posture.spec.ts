import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EmailPostureComponent, DomainRow, PostureControlView } from './email-posture';
import { WailsIpcService } from '../../../wails-ipc.service';
import { CoverageState, EmailPostureUI } from '../../../models/types';

describe('EmailPosture', () => {
  let component: EmailPostureComponent;
  let fixture: ComponentFixture<EmailPostureComponent>;
  let ipc: WailsIpcService;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [EmailPostureComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(EmailPostureComponent);
    component = fixture.componentInstance;
    ipc = TestBed.inject(WailsIpcService);
    ipc.assessments.set([]);
    ipc.emailPostures.set([]);
    ipc.regressions.set([]);
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

  const assessment = (controls: any[], outcome = 'completed', total = 10, ok = 10, domain = 'd.example') => ({
    assetId: 'a', domain, outcome,
    coverage: coverage(total, ok), coverageFraction: total ? ok / total : 0,
    controls, scenarios: [], unmapped: [], registryVersion: 'v1', libraryVersion: '1.0'
  }) as any;

  const ctrl = (state: CoverageState, over: Partial<any> = {}) =>
    ({ state, conclusive: state === 'ok' || state === 'not_found', ...over });

  const posture = (over: Partial<EmailPostureUI> = {}): EmailPostureUI => ({
    domain: 'd.example',
    spf: ctrl('ok', { detail: 'v=spf1 -all' }),
    dkim: ctrl('ok', { detail: 'selector1' }),
    dmarc: ctrl('ok', { detail: 'v=DMARC1; p=reject' }),
    mtaSts: ctrl('not_checked'),
    tlsRpt: ctrl('not_checked'),
    bimi: ctrl('not_found'),
    caa: ctrl('not_found'),
    dmarcPolicy: 'reject',
    dmarcPercent: 100,
    dmarcReporting: true,
    spfAllMechanism: '-',
    spfLookups: 4,
    priority: 'low',
    lastChecked: '2026-01-01T00:00:00Z',
    assessedControls: 5,
    totalControls: 7,
    ...over
  });

  /** The row the component assembles for the single fixture domain. */
  const rowFor = (domain = 'd.example'): DomainRow =>
    component.rows().find(r => r.domain === domain)!;

  const controlOf = (row: DomainRow, key: string): PostureControlView =>
    row.controls.find(c => c.key === key)!;

  describe('one account of a domain', () => {
    // Both pipelines describe SPF, DKIM and DMARC on the same domain. Showing
    // both would ask the operator to reconcile two states for one control, so
    // the posture record is the single source of state and the assessment
    // contributes only the advisories raised against it.
    it('takes each control state from the posture record, not the assessment', () => {
      ipc.assessments.set([assessment([control('spf', 'compliant', [{ signalId: 's1', severity: 'high' }])])]);
      ipc.emailPostures.set([posture({ spf: ctrl('check_failed', { reason: 'resolver timed out' }) })]);

      const spf = controlOf(rowFor(), 'spf');
      expect(spf.state).toBe('check_failed');
      expect(spf.signals.length).toBe(1);
    });

    it('does not repeat a posture control under the remaining surface', () => {
      const a = assessment([control('spf', 'compliant'), control('dnssec', 'deficient')]);
      expect(component.otherControls(a).map(c => c.control)).toEqual(['dnssec']);
    });

    // A domain assessed for its email posture but not yet by the measured-state
    // pipeline would otherwise vanish from the only view that shows it.
    it('shows a domain that has a posture record and no assessment', () => {
      ipc.emailPostures.set([posture({ domain: 'only-posture.example' })]);
      expect(component.rows().map(r => r.domain)).toContain('only-posture.example');
    });
  });

  describe('the seven controls', () => {
    it('renders every control, including those never assessed', () => {
      ipc.emailPostures.set([posture()]);
      expect(rowFor().controls.map(c => c.key))
        .toEqual(['spf', 'dkim', 'dmarc', 'mtaSts', 'tlsRpt', 'bimi', 'caa']);
    });

    // The whole point of the four-state model: no state may present as the
    // one that says a control was assessed and found in place.
    it('gives each state its own presentation, none shared with ok', () => {
      const states: CoverageState[] = ['ok', 'not_found', 'not_checked', 'check_failed'];
      const labels = states.map(s => component.stateLabel(s));
      const glyphs = states.map(s => component.stateGlyph(s));
      const badges = states.map(s => component.stateBadgeClasses(s, false));

      expect(new Set(labels).size).toBe(4);
      expect(new Set(glyphs).size).toBe(4);
      expect(new Set(badges).size).toBe(4);
    });

    // Colour is not the only carrier of state: strip the classes away and the
    // four are still distinguishable by word and by shape.
    it('remains legible without colour', () => {
      const states: CoverageState[] = ['ok', 'not_found', 'not_checked', 'check_failed'];
      for (const s of states) {
        expect(component.stateLabel(s).length).toBeGreaterThan(0);
        expect(component.stateGlyph(s).length).toBeGreaterThan(0);
      }
    });

    // A rendered label must not assert a fact the state cannot support. An
    // unassessed control that reads as "published" or "assessed" is the exact
    // defect the four-state model was introduced to prevent.
    it('never renders affirmative text for an unassessed control', () => {
      const affirmative = /\b(assessed|published|enforc\w*|present|configured|valid|in place)\b/i;
      ipc.emailPostures.set([posture({
        spf: ctrl('not_checked'),
        dkim: ctrl('check_failed'),
        dmarc: ctrl('not_checked', { reason: 'excluded by egress policy' })
      })]);

      for (const key of ['spf', 'dkim', 'dmarc']) {
        const c = controlOf(rowFor(), key);
        expect(component.controlHeadline(c)).not.toMatch(affirmative);
      }
    });
  });

  describe('an inconclusive state states why', () => {
    it('carries the recorded reason', () => {
      ipc.emailPostures.set([posture({
        mtaSts: ctrl('not_checked', { reason: 'excluded by egress policy' })
      })]);
      expect(component.reasonText(controlOf(rowFor(), 'mtaSts'))).toBe('excluded by egress policy');
    });

    // The gap is disclosed, not filled. A reason inferred from the state would
    // be evidence nobody observed.
    it('does not invent a reason where none was recorded', () => {
      ipc.emailPostures.set([posture({ tlsRpt: ctrl('check_failed') })]);
      const c = controlOf(rowFor(), 'tlsRpt');
      expect(component.needsReason(c)).toBe(true);
      expect(component.reasonText(c)).toContain('No reason was recorded');
    });

    it('says so when no posture record exists at all', () => {
      ipc.assessments.set([assessment([])]);
      const c = controlOf(rowFor(), 'spf');
      expect(c.recorded).toBe(false);
      expect(c.state).toBe('not_checked');
      expect(component.reasonText(c)).toContain('No posture record');
    });
  });

  describe('DKIM', () => {
    // Selectors cannot be enumerated from DNS, so finding nothing at the ones
    // we guessed establishes nothing. A reader told "DKIM absent" would
    // commission work that is very likely already done.
    it('reports a probed absence as inconclusive, never as absence', () => {
      ipc.emailPostures.set([posture({
        dkim: ctrl('not_found', { conclusive: false }),
        dkimSelectorsExamined: ['selector1', 'google', 'k1'],
        dkimSelectorsFound: []
      })]);
      const c = controlOf(rowFor(), 'dkim');

      expect(component.controlHeadline(c)).toBe('No key at the selectors examined');
      expect(component.controlHeadline(c)).not.toMatch(/absent|missing|not configured|not published/i);
      expect(c.notes.join(' ')).toContain('does not establish');
    });

    it('reports a conclusive absence as a fact about the domain', () => {
      ipc.emailPostures.set([posture({ dkim: ctrl('not_found', { conclusive: true }) })]);
      expect(component.controlHeadline(controlOf(rowFor(), 'dkim'))).toBe('Not published');
    });

    // "One of thirteen" and "a key exists" are different statements.
    it('discloses both the selectors found and those examined', () => {
      const evidence = component.dkimEvidence(posture({
        dkimSelectorsExamined: ['s1', 's2', 's3'],
        dkimSelectorsFound: ['s1']
      }));
      expect(evidence[0]).toBe('1 of 3 selector(s) examined yielded a key');
      expect(evidence.join(' ')).toContain('examined: s1, s2, s3');
    });
  });

  describe('coverage', () => {
    // The figure is the engine's. Computing it here would be a second
    // definition of "assessed", and two definitions of a coverage figure
    // eventually disagree.
    it('renders the coverage the record carries rather than deriving one', () => {
      ipc.emailPostures.set([posture({ assessedControls: 3, totalControls: 7 })]);
      expect(component.coverageLine(rowFor())).toBe('3/7 controls assessed');
    });

    it('says so when a record predates the coverage figure', () => {
      const p = posture();
      delete p.assessedControls;
      delete p.totalControls;
      ipc.emailPostures.set([p]);

      expect(component.hasCoverage(rowFor())).toBe(false);
      expect(component.coverageLine(rowFor())).toContain('not reported');
    });

    it('states coverage before any judgement on a collapsed row', () => {
      ipc.assessments.set([assessment([control('spf', 'compliant')], 'completed', 20, 18)]);
      ipc.emailPostures.set([posture({ assessedControls: 3 })]);

      const line = component.summaryLine(rowFor());
      expect(line.indexOf('3/7 controls assessed')).toBe(0);
      expect(line).toContain('18/20');
    });

    it('distinguishes never-assessed from assessed-clean', () => {
      ipc.assessments.set([assessment([], 'completed', 0, 0)]);
      expect(component.summaryLine(rowFor())).toContain('nothing has been assessed');
    });

    // A refusal is not a finding about the domain, it is a fact about the
    // scope. It must displace the advisory summary rather than sit beside it.
    it('reports the outcome instead of a tally when nothing was queried', () => {
      const a = assessment([], 'refused', 0, 0);
      a.error = 'outside the authorised scope';
      ipc.assessments.set([a]);
      expect(component.summaryLine(rowFor())).toContain('outside the authorised scope');
    });
  });

  describe('severity', () => {
    // The client renders the engine's rating and derives none of its own.
    it('renders the stored priority verbatim', () => {
      for (const p of ['critical', 'high', 'medium', 'low', 'info'] as const) {
        ipc.emailPostures.set([posture({ priority: p })]);
        expect(component.priority(rowFor())).toBe(p);
      }
    });

    // An unassessed control has no severity, and a floor substituted here
    // would let a resolver outage manufacture a finding.
    it('shows no severity where the engine recorded none', () => {
      ipc.emailPostures.set([posture({ priority: '', dmarc: ctrl('check_failed') })]);
      expect(component.priority(rowFor())).toBe('');
      expect(component.hasPriority(rowFor())).toBe(false);
    });

    it('does not count an unrated domain as a rated one', () => {
      ipc.emailPostures.set([
        posture({ domain: 'rated.example', priority: 'high' }),
        posture({ domain: 'unrated.example', priority: '' })
      ]);
      expect(component.ratedCount()).toBe(1);
    });
  });

  describe('DMARC evidence', () => {
    it('shows the tags the rating was derived from', () => {
      const evidence = component.dmarcEvidence(posture({
        dmarcPolicy: 'reject', dmarcPercent: 100, dmarcSubdomainPolicy: 'none',
        dmarcAlignmentSpf: 's', dmarcAlignmentDkim: 'r', dmarcReporting: true
      }));
      expect(evidence).toContain('p=reject');
      expect(evidence).toContain('pct=100');
      expect(evidence).toContain('sp=none');
      expect(evidence).toContain('aspf=s');
      expect(evidence).toContain('adkim=r');
      expect(evidence.join(' ')).toContain('aggregate reporting published');
    });

    // A badge reading "reject" would say the route is closed. At pct=40 it is
    // open three times in five.
    it('discloses partial enforcement as partial', () => {
      ipc.emailPostures.set([posture({ dmarcPolicy: 'reject', dmarcPercent: 40 })]);
      const notes = controlOf(rowFor(), 'dmarc').notes.join(' ');
      expect(notes).toContain('Partial enforcement');
      expect(notes).toContain('40%');
      expect(notes).toContain('60%');
    });

    it('describes a monitoring-only policy as delivering spoofed mail', () => {
      ipc.emailPostures.set([posture({ dmarcPolicy: 'none' })]);
      expect(controlOf(rowFor(), 'dmarc').notes.join(' ')).toContain('Monitoring only');
    });

    it('surfaces an unenforced subdomain policy', () => {
      ipc.emailPostures.set([posture({ dmarcPolicy: 'reject', dmarcSubdomainPolicy: 'none' })]);
      expect(controlOf(rowFor(), 'dmarc').notes.join(' ')).toContain('subdomains are not');
    });

    it('offers no evidence for a policy that was never observed', () => {
      expect(component.dmarcEvidence(posture({ dmarc: ctrl('check_failed') }))).toEqual([]);
    });
  });

  describe('SPF evidence', () => {
    it('discloses the all-mechanism qualifier and the lookup count', () => {
      const evidence = component.spfEvidence(posture({ spfAllMechanism: '+', spfLookups: 12 }));
      expect(evidence).toContain('+all');
      expect(evidence.join(' ')).toContain('12 DNS-querying mechanism(s)');
    });

    it('says what "+all" authorises', () => {
      ipc.emailPostures.set([posture({ spfAllMechanism: '+' })]);
      expect(controlOf(rowFor(), 'spf').notes.join(' ')).toContain('every host on the internet');
    });

    // Above ten lookups a receiver may return permerror and stop, so a
    // careful policy can be enforced nowhere.
    it('warns when the record exceeds the lookup limit', () => {
      ipc.emailPostures.set([posture({ spfLookups: 14 })]);
      expect(controlOf(rowFor(), 'spf').notes.join(' ')).toContain('may be enforced nowhere');
    });
  });

  describe('policy drift', () => {
    const regression = (over: Partial<any> = {}) => ({
      id: 'r1', assetId: 'a', attributeType: 'dmarc_policy',
      previousValue: 'p=reject;pct=100', currentValue: 'p=none;pct=100',
      consecutiveFails: 2, confirmedAt: '2026-02-01T00:00:00Z', ...over
    });

    it('surfaces a weakened policy against its domain', () => {
      ipc.assessments.set([assessment([])]);
      ipc.emailPostures.set([posture()]);
      ipc.regressions.set([regression()]);

      const drift = component.driftFor(rowFor());
      expect(drift.length).toBe(1);
      expect(component.driftLine(drift[0])).toContain('p=reject;pct=100');
      expect(component.driftLine(drift[0])).toContain('p=none;pct=100');
    });

    it('ignores drift recorded against another attribute', () => {
      ipc.assessments.set([assessment([])]);
      ipc.regressions.set([regression({ attributeType: 'tls_version' })]);
      expect(component.driftFor(rowFor()).length).toBe(0);
    });

    // No fingerprint is written for an assessment that did not conclude, so a
    // resolver outage cannot present itself here as a weakened policy.
    it('reports no drift for a domain whose assessment did not conclude', () => {
      ipc.assessments.set([assessment([], 'failed', 0, 0)]);
      ipc.emailPostures.set([posture({ dmarc: ctrl('check_failed'), priority: '' })]);
      expect(component.driftFor(rowFor())).toEqual([]);
    });
  });

  describe('boundaries', () => {
    // Change 012's boundary applies here as everywhere: this product states
    // what was observed and what it is rated, never what it would cost or how
    // likely a compromise is.
    it('states no monetary value and no probability of compromise', () => {
      ipc.assessments.set([assessment([control('spf', 'deficient', [{ signalId: 's', severity: 'high' }])])]);
      ipc.emailPostures.set([posture({
        dmarcPolicy: 'reject', dmarcPercent: 40, spfAllMechanism: '+', spfLookups: 14,
        dkim: ctrl('not_found', { conclusive: false })
      })]);
      ipc.regressions.set([{
        id: 'r1', assetId: 'a', attributeType: 'dmarc_policy',
        previousValue: 'p=reject;pct=100', currentValue: 'p=none;pct=100',
        consecutiveFails: 2, confirmedAt: '2026-02-01T00:00:00Z'
      }]);

      const row = rowFor();
      const rendered = [
        component.summaryLine(row),
        component.coverageLine(row),
        ...row.controls.flatMap(c => [component.controlHeadline(c), component.reasonText(c), ...c.evidence, ...c.notes]),
        ...component.driftFor(row).map(d => component.driftLine(d))
      ].join(' ');

      expect(rendered).not.toMatch(/[£$€]|\bUSD\b|\bGBP\b|\bEUR\b/);
      expect(rendered).not.toMatch(/likelihood|probability|expected loss|annualised loss|chance of (a )?(breach|compromise)/i);
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

    it('expands every row it knows about, from either pipeline', () => {
      ipc.assessments.set([assessment([], 'completed', 1, 1, 'assessed.example')]);
      ipc.emailPostures.set([posture({ domain: 'posture.example' })]);
      component.expandAll();

      expect(component.isOpen('assessed.example')).toBe(true);
      expect(component.isOpen('posture.example')).toBe(true);
    });
  });

  describe('advisory counts', () => {
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

  /**
   * An advisory that says an include target is broken, without saying which,
   * identifies a problem without pointing at it. The library supplies the
   * general explanation and the specific one separately, and both have to be
   * rendered: the general one alone leaves the operator to find the fault in
   * the evidence string, which for SPF is an entire record.
   *
   * These assert against the rendered DOM rather than the component's inputs,
   * because the failure being guarded is a template that drops a field.
   */
  describe('advisory prose', () => {
    const brokenInclude = {
      signalId: 'SURF-SPF-009',
      checkId: 'spf',
      condition: 'SPF include target publishes no usable record',
      severity: 'medium',
      state: 'ok',
      evidence: 'spf.term=include:dead.example',
      description: 'An include or redirect names a domain that does not resolve.',
      detail: 'Specifically, the term `include:dead.example` names `dead.example`, ' +
        'which does not resolve or publishes no SPF record.',
      remediation: 'Correct or remove the named term.',
    };

    const renderAdvisory = async (signal: Record<string, unknown>) => {
      ipc.assessments.set([assessment([control('spf', 'deficient', [signal])])]);
      ipc.emailPostures.set([posture()]);
      fixture.detectChanges();

      component.toggleDomain('d.example');
      component.toggle('d.example', 'spf');
      fixture.detectChanges();
      await fixture.whenStable();

      return fixture.nativeElement.textContent as string;
    };

    it('names the offending item, not only the identifier', async () => {
      const text = await renderAdvisory(brokenInclude);
      expect(text).toContain('include:dead.example');
    });

    it('keeps the general explanation alongside the specific one', async () => {
      const text = await renderAdvisory(brokenInclude);
      expect(text).toContain('An include or redirect names a domain that does not resolve.');
      expect(text).toContain('Correct or remove the named term.');
    });

    // Most signals name no particular item. Their advisories must render as
    // before rather than leaving a gap where the sentence would have been.
    it('renders normally when there is no specific detail', async () => {
      const { detail, ...withoutDetail } = brokenInclude;
      const text = await renderAdvisory(withoutDetail);
      expect(text).toContain('SPF include target publishes no usable record');
      expect(text).not.toContain('Specifically,');
    });
  });
});
