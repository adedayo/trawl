import { ComponentFixture, TestBed } from '@angular/core/testing';
import axe from 'axe-core';
import { vi } from 'vitest';

import { FindingDetailComponent } from './finding-detail';
import { WailsIpcService } from '../../../../wails-ipc.service';
import { detail, event } from '../../../../dashboard/findings.fixtures';
import { unsupportedClaims } from '../../../../dashboard/findings';
import { FindingDetail } from '../../../../models/types';

describe('FindingDetail', () => {
  let fixture: ComponentFixture<FindingDetailComponent>;
  let ipc: WailsIpcService;
  let root: HTMLElement;
  let closed: ReturnType<typeof vi.fn<() => void>>;

  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  /** Mounts the detail for `key`, answering the read with `result`. */
  async function mount(result: FindingDetail | null | Error, key = 'k1'): Promise<void> {
    const spy = vi.spyOn(ipc.transport, 'getFindingDetail');
    if (result instanceof Error) {
      spy.mockRejectedValue(result);
    } else {
      spy.mockResolvedValue(result);
    }
    fixture = TestBed.createComponent(FindingDetailComponent);
    fixture.componentRef.setInput('findingKey', key);
    closed = vi.fn<() => void>();
    fixture.componentInstance.closed.subscribe(() => closed());
    root = fixture.nativeElement as HTMLElement;
    await settle();
  }

  const text = () => root.textContent ?? '';
  const headings = (selector: string) => Array.from(root.querySelectorAll(selector)).map(h => h.textContent?.trim());

  beforeEach(async () => {
    window.history.replaceState(null, '', window.location.pathname);
    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({ imports: [FindingDetailComponent] }).compileComponents();
    ipc = TestBed.inject(WailsIpcService);
    ipc.findingIndex.set([]);
  });

  describe('header', () => {
    it('shows severity, priority, status, first seen, last seen and how long it has been open', async () => {
      await mount(detail({ severity: 'high', priority: 'urgent', status: 'open', ageDays: 17 }));

      const header = text();
      expect(header).toContain('high');
      expect(header).toContain('Priority: urgent');
      expect(header).toContain('Open');
      expect(header).toContain('2026-11-01 10:00 UTC');
      expect(header).toContain('2026-11-18 10:00 UTC');
      expect(root.querySelector('[data-testid="age-line"]')?.textContent).toContain('Open for 17 days');
    });

    it('gives the resolution date for a resolved finding', async () => {
      await mount(detail({ status: 'resolved', resolvedAt: '2026-11-10T08:00:00Z', ageDays: 9 }));
      expect(root.querySelector('[data-testid="age-line"]')?.textContent).toContain('Resolved on 2026-11-10, 9 days after');
    });

    it('marks a finding that was not re-assessed, with the reason', async () => {
      await mount(detail({ notReassessed: true, notReassessedReason: 'The resolver timed out.' }));
      expect(root.querySelector('[data-testid="not-reassessed-note"]')?.textContent).toContain('The resolver timed out.');
      expect(text()).toContain('Not re-assessed');
    });

    it('names the finding in the heading', async () => {
      await mount(detail({ title: 'No SPF record is published' }));
      expect(root.querySelector('h2')?.textContent).toContain('No SPF record is published');
    });
  });

  describe('executive summary', () => {
    it('shows every section, in the fixed order', async () => {
      await mount(detail());
      expect(headings('[data-testid="executive-summary"] h4')).toEqual([
        'What we found',
        'Why it matters',
        'How it could be abused',
        'What good looks like',
        'What to do',
        'How a re-scan confirms the fix',
        'References and standards'
      ]);
      const summary = root.querySelector('[data-testid="executive-summary"]')!.textContent!;
      const e = detail().explanation;
      for (const part of [e.whatWeFound, e.whyItMatters, e.howItCouldBeAbused, e.whatGoodLooksLike, e.howRescanConfirms]) {
        expect(summary).toContain(part);
      }
    });

    it('renders "what to do" as an ordered list', async () => {
      await mount(detail());
      const steps = Array.from(root.querySelectorAll('[data-testid="executive-summary"] ol li')).map(li => li.textContent?.trim());
      expect(steps).toEqual(detail().explanation.whatToDo);
    });

    it('renders http references as links and the rest as text, with the standards', async () => {
      await mount(detail());
      const link = root.querySelector<HTMLAnchorElement>('[data-testid="executive-summary"] a')!;
      expect(link.href).toBe('https://www.rfc-editor.org/rfc/rfc7208');
      expect(link.rel).toContain('noopener');
      expect(text()).toContain('NIST SP 800-177');
    });

    it('shows the coverage note when the assessment did not conclude, and not otherwise', async () => {
      const note = 'The DKIM selectors could not be enumerated, so this control could not be assessed.';
      await mount(detail({ explanation: { ...detail().explanation, coverageNote: note } }));
      expect(root.querySelector('[data-testid="coverage-note"]')?.textContent).toContain(note);

      TestBed.resetTestingModule();
      await TestBed.configureTestingModule({ imports: [FindingDetailComponent] }).compileComponents();
      ipc = TestBed.inject(WailsIpcService);
      await mount(detail());
      expect(root.querySelector('[data-testid="coverage-note"]')).toBeNull();
    });

    it('shows the exploitation status in words when present, and not otherwise', async () => {
      const status = 'This weakness is known to be used in real attacks. Checked on 2026-11-18.';
      await mount(detail({ explanation: { ...detail().explanation, exploitationStatus: status } }));
      expect(root.querySelector('[data-testid="exploitation-status"]')?.textContent).toContain(status);

      TestBed.resetTestingModule();
      await TestBed.configureTestingModule({ imports: [FindingDetailComponent] }).compileComponents();
      ipc = TestBed.inject(WailsIpcService);
      await mount(detail());
      expect(root.querySelector('[data-testid="exploitation-status"]')).toBeNull();
    });

    it('states which narrative level supplied the wording when it is not the signal itself', async () => {
      await mount(detail({
        explanation: { ...detail().explanation, level: 'weakness-class', levelKey: 'email-spoofing', version: 'narratives-2026.11' }
      }));
      const line = root.querySelector('[data-testid="level-line"]')?.textContent ?? '';
      expect(line).toContain('weakness class');
      expect(line).toContain('email-spoofing');
    });

    it('says nothing about the level when the wording was written for the signal, but always gives the version', async () => {
      await mount(detail());
      expect(root.querySelector('[data-testid="level-line"]')).toBeNull();
      expect(root.querySelector('[data-testid="narrative-provenance"]')?.textContent).toContain('Narrative version narratives-2026.11');
    });

    it('does not render any AI annotation a legacy row may carry', async () => {
      const legacy = {
        ...detail(),
        aiAnnotation: { summary: 'AI-WRITTEN SUMMARY', remediation: 'AI-WRITTEN FIX' }
      } as unknown as FindingDetail;
      await mount(legacy);
      expect(text()).not.toContain('AI-WRITTEN');
      expect(text()).not.toMatch(/AI Triage|Advisory/);
    });
  });

  describe('evidence', () => {
    it('shows the evidence verbatim in a monospace block', async () => {
      await mount(detail({ evidence: 'v=spf1 include:_spf.example.com ~all' }));
      const pre = root.querySelector<HTMLElement>('[data-testid="evidence"]')!;
      expect(pre.tagName).toBe('PRE');
      expect(pre.textContent).toBe('v=spf1 include:_spf.example.com ~all');
      expect(pre.className).toContain('font-mono');
    });

    it('states the latest check outcome in words, including when it could not conclude', async () => {
      await mount(detail({ checkId: 'dkim', checkState: 'check_failed', checkReason: 'the resolver refused' }));
      const state = root.querySelector('[data-testid="check-state"]')?.textContent ?? '';
      expect(state).toContain('check failed');
      expect(state).toContain('the resolver refused');
    });

    it('says so when no evidence was recorded', async () => {
      await mount(detail({ evidence: '' }));
      expect(root.querySelector('[data-testid="evidence"]')).toBeNull();
      expect(text()).toContain('No evidence value was recorded');
    });
  });

  describe('timeline', () => {
    const events = () => Array.from(root.querySelectorAll('[data-testid="timeline-event"]'));

    it('shows a fixed-and-returned finding in order, each with its time and run', async () => {
      await mount(detail({
        timeline: [
          event({ id: 'e3', type: 'reopened', at: '2026-11-15T10:00:00Z', runId: 'run-9' }),
          event({ id: 'e1', type: 'first_seen', at: '2026-11-01T10:00:00Z', runId: 'run-1' }),
          event({ id: 'e2', type: 'resolved', at: '2026-11-08T10:00:00Z', runId: 'run-5' })
        ]
      }));

      const entries = events().map(e => e.textContent ?? '');
      expect(entries).toHaveLength(3);
      expect(entries[0]).toContain('First seen');
      expect(entries[0]).toContain('2026-11-01 10:00 UTC');
      expect(entries[0]).toContain('run run-1');
      expect(entries[1]).toContain('Resolved');
      expect(entries[1]).toContain('run run-5');
      expect(entries[2]).toContain('Reopened');
      expect(entries[2]).toContain('run run-9');
    });

    it('labels a library-caused change as the scanner changing, and says it is not the estate', async () => {
      await mount(detail({
        timeline: [event({ type: 'changed', cause: 'library', prevSeverity: 'medium', severity: 'high', libraryVersion: 'v1.9.0' })]
      }));
      const entry = events()[0].textContent ?? '';
      expect(entry).toContain('scanner library changed');
      expect(entry).toContain('not a change in the estate');
      expect(entry).toContain('Severity: medium to high');
    });

    it('labels a feed-caused priority change and cites the snapshot', async () => {
      await mount(detail({
        timeline: [event({ type: 'priority_changed', cause: 'feed', snapshotId: 'snap-42', prevPriority: 'medium', priority: 'urgent' })]
      }));
      const entry = events()[0].textContent ?? '';
      expect(entry).toContain('feed update');
      expect(entry).toContain('snap-42');
      expect(entry).toContain('Priority: medium to urgent');
    });

    it('labels a baseline event as recorded when history began, not as an observation', async () => {
      await mount(detail({ timeline: [event({ type: 'baseline', cause: 'baseline', runId: undefined })] }));
      const entry = events()[0].textContent ?? '';
      expect(entry).toContain('recorded when history began');
      expect(entry).toContain('Recorded at the start of history');
      expect(entry).not.toContain('not a change in the estate');
    });

    it('labels an ordinary change as a change in the estate and shows the engine note', async () => {
      await mount(detail({ timeline: [event({ type: 'resolved', cause: 'estate', note: 'No longer published.' })] }));
      const entry = events()[0].textContent ?? '';
      expect(entry).toContain('change in the estate');
      expect(entry).toContain('No longer published.');
    });

    it('says so when there are no events', async () => {
      await mount(detail({ timeline: [] }));
      expect(events()).toHaveLength(0);
      expect(text()).toContain('No events have been recorded');
    });
  });

  describe('provenance and regressions', () => {
    it('shows the run, library and registry versions and the feed snapshots', async () => {
      await mount(detail({
        provenance: {
          runId: 'run-77',
          libraryVersion: 'vantage v1.8.0',
          registryVersion: 'reg-12',
          feedSnapshots: [{ id: 'snap-9', feed: 'cisa-kev', sourceUrl: 'https://example.test/kev', retrievedAt: '2026-11-18T06:00:00Z', contentDigest: 'abc', recordCount: 1200 }]
        }
      }));
      const t = text();
      expect(t).toContain('run-77');
      expect(t).toContain('vantage v1.8.0');
      expect(t).toContain('reg-12');
      expect(t).toContain('cisa-kev');
      expect(t).toContain('snapshot snap-9');
    });

    it('says when nothing is recorded rather than leaving a blank', async () => {
      await mount(detail({ provenance: {} }));
      expect(text()).toContain('not recorded');
      expect(text()).toContain('No feed snapshot is cited');
    });

    it('lists related regressions', async () => {
      await mount(detail({
        regressions: [{ id: 'r1', assetId: 'a', attributeType: 'dmarc_policy', previousValue: 'reject', currentValue: 'none', consecutiveFails: 3, confirmedAt: '2026-11-12T00:00:00Z' }]
      }));
      expect(text()).toContain('dmarc_policy: reject to none');
    });

    it('omits the regressions section when there are none', async () => {
      await mount(detail({ regressions: [] }));
      expect(text()).not.toContain('Related regressions');
    });

    it('shows candidate raw string, matched product/version, NVD severity label, and lookup snapshot', async () => {
      await mount(detail({
        source: 'candidate',
        unconfirmed: true,
        severitySource: 'nvd',
        cve: 'CVE-2021-23017',
        observedString: 'Nginx:1.18.0',
        matchedVendor: 'f5',
        matchedProduct: 'nginx',
        matchedVersion: '1.18.0',
        lookupSnapshotId: 'nvd-abc123',
        lookupAt: '2026-11-20T12:00:00Z'
      }));
      expect(root.querySelector('[data-testid="detail-unconfirmed-badge"]')?.textContent).toContain('Unconfirmed candidate');
      expect(text()).toContain('Severity from NVD');
      const matchBox = root.querySelector('[data-testid="candidate-match-evidence"]')?.textContent ?? '';
      expect(matchBox).toContain('Nginx:1.18.0');
      expect(matchBox).toContain('f5:nginx 1.18.0');
      expect(matchBox).toContain('nvd-abc123');
    });
  });

  describe('states', () => {
    it('says a finding was not found, and that Back returns to the list', async () => {
      await mount(null, 'gone');
      expect(root.querySelector('[data-testid="notfound"]')?.textContent).toContain('was not found');
      expect(root.querySelector('[data-testid="executive-summary"]')).toBeNull();
    });

    it('says a failed load is not a resolution', async () => {
      await mount(new Error('503'));
      const message = root.querySelector('[data-testid="load-error"]')?.textContent ?? '';
      expect(message).toContain('could not be loaded');
      expect(message).toContain('does not mean it is resolved');
    });

    it('ignores a slow answer for a finding that is no longer the one shown', async () => {
      let release!: (d: FindingDetail) => void;
      const slow = new Promise<FindingDetail>(resolve => (release = resolve));
      const spy = vi.spyOn(ipc.transport, 'getFindingDetail');
      spy.mockReturnValueOnce(slow);
      spy.mockResolvedValueOnce(detail({ title: 'Second finding', key: 'second' }));

      fixture = TestBed.createComponent(FindingDetailComponent);
      fixture.componentRef.setInput('findingKey', 'first');
      root = fixture.nativeElement as HTMLElement;
      // Not settled between the two: the first read is deliberately still in flight.
      fixture.detectChanges();
      fixture.componentRef.setInput('findingKey', 'second');
      fixture.detectChanges();
      release(detail({ title: 'First finding', key: 'first' }));
      await settle();

      expect(root.querySelector('h2')?.textContent).toContain('Second finding');
    });
  });

  describe('closing and focus', () => {
    it('moves focus to the heading when it opens', async () => {
      await mount(detail());
      expect(document.activeElement).toBe(root.querySelector('h2'));
    });

    it('closes from the Back button', async () => {
      await mount(detail());
      root.querySelector<HTMLButtonElement>('[data-testid="back"]')!.click();
      expect(closed).toHaveBeenCalledTimes(1);
    });

    it('closes on Escape', async () => {
      await mount(detail());
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      expect(closed).toHaveBeenCalledTimes(1);
    });

    it('is an inline region, not a dialog, so there is no focus trap', async () => {
      await mount(detail());
      expect(root.querySelector('[role="dialog"]')).toBeNull();
      expect(root.querySelector('[aria-modal]')).toBeNull();
      expect(root.querySelector('section[aria-labelledby="finding-heading"]')).not.toBeNull();
    });
  });

  describe('the narrative boundary', () => {
    it('renders text with no currency amount and no probability of compromise', async () => {
      await mount(detail({
        explanation: {
          ...detail().explanation,
          exploitationStatus: 'This weakness is known to be used in real attacks. Checked on 2026-11-18.',
          coverageNote: 'This control could not be assessed on the latest run.'
        }
      }));
      expect(unsupportedClaims(text())).toEqual([]);
    });

    it('would catch a figure if a narrative ever rendered one', async () => {
      await mount(detail({
        explanation: { ...detail().explanation, whyItMatters: 'An incident could cost $250,000 and has a 30% chance of compromise.' }
      }));
      const claims = unsupportedClaims(text());
      expect(claims.length).toBeGreaterThanOrEqual(2);
    });

    it('does not describe a control that was not assessed as secure, working or fine', async () => {
      await mount(detail({
        checkState: 'not_checked',
        notReassessed: true,
        explanation: { ...detail().explanation, coverageNote: 'This control could not be assessed.' }
      }));
      expect(root.querySelector('[data-testid="coverage-note"]')?.textContent).not.toMatch(/\b(secure|working|fine|in place)\b/i);
      expect(root.querySelector('[data-testid="not-reassessed-note"]')?.textContent).not.toMatch(/\b(secure|working|fine)\b/i);
    });
  });

  describe('accessibility', () => {
    it('has no detectable WCAG 2.1 AA violations, apart from contrast which jsdom cannot compute', async () => {
      await mount(detail({
        timeline: [event({ type: 'first_seen' }), event({ id: 'e2', type: 'changed', cause: 'library', at: '2026-11-05T00:00:00Z' })],
        regressions: [{ id: 'r1', assetId: 'a', attributeType: 'dmarc_policy', previousValue: 'reject', currentValue: 'none', consecutiveFails: 3, confirmedAt: '2026-11-12T00:00:00Z' }],
        explanation: { ...detail().explanation, exploitationStatus: 'Checked on 2026-11-18.', coverageNote: 'Could not be assessed.' }
      }));
      const results = await axe.run(root, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] },
        rules: { 'color-contrast': { enabled: false } }
      });
      expect(results.violations.map(v => `${v.id}: ${v.nodes.length} node(s)`)).toEqual([]);
    });
  });
});
