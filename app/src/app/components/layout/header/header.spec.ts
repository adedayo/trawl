import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { HeaderComponent } from './header';
import { WailsIpcService } from '../../../wails-ipc.service';

describe('Header', () => {
  let component: HeaderComponent;
  let fixture: ComponentFixture<HeaderComponent>;
  let ipc: WailsIpcService;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [HeaderComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(HeaderComponent);
    component = fixture.componentInstance;
    ipc = TestBed.inject(WailsIpcService);
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  /**
   * Fires the completion event the backend publishes for one finished target.
   *
   * Every deployment emits exactly one of these per scan request — the desktop
   * binding after its goroutine returns, a long-lived container after its
   * detached run, an inline container before it writes the response. A test
   * that resolves `triggerScan` without emitting is modelling a backend that
   * accepts work and never reports on it, which no deployment does.
   */
  const completeOneTarget = (): void => {
    const handlers = (ipc.transport as any).handlers as Map<string, Set<(p: any) => void>>;
    handlers.get('scan:complete')?.forEach(h => h(null));
  };

  /** Fires a completion event carrying the outcome the backend reports. */
  const completeWithOutcome = (payload: any): void => {
    const handlers = (ipc.transport as any).handlers as Map<string, Set<(p: any) => void>>;
    handlers.get('scan:complete')?.forEach(h => h(payload));
  };

  /** A triggerScan stand-in that reports completion, as the backend does. */
  const scanReporting = (record?: (domain: string) => void) =>
    vi.spyOn(ipc.transport, 'triggerScan').mockImplementation(async (domain: string) => {
      record?.(domain);
      completeOneTarget();
    });

  describe('scanning the authorised scope', () => {
    /**
     * Scanning must cover the whole authorised scope.
     *
     * This is a regression test for a bug where only `seedDomainsList()[0]`
     * was scanned. Because re-adding a domain moves it to the end of the list,
     * the target silently changed to whichever domain happened to sit at index
     * zero — so results looked order-dependent, a domain added later never
     * produced findings, and the UI still reported "Scan complete".
     */
    it('scans every authorised domain, not only the first', async () => {
      const scanned: string[] = [];
      scanReporting(domain => {
        if (domain) {
          scanned.push(domain);
        }
      });

      ipc.seedDomainsList.set(['glg.com', 'glgroup.com', 'glgresearch.com']);
      ipc.seedReposList.set([]);

      await ipc.scanAuthorisedScope();

      expect(scanned).toEqual(['glg.com', 'glgroup.com', 'glgresearch.com']);
    });

    /**
     * A target that fails must not prevent the others from being scanned, and
     * must be named afterwards. Results that are silently partial read as
     * clean, which is the more dangerous way to be wrong.
     */
    it('continues past a failing target and reports which failed', async () => {
      const scanned: string[] = [];
      vi.spyOn(ipc.transport, 'triggerScan').mockImplementation(async (domain: string) => {
        if (!domain) {
          return;
        }
        scanned.push(domain);
        if (domain === 'broken.example') {
          throw new Error('resolution failed');
        }
        completeOneTarget();
      });

      ipc.seedDomainsList.set(['first.example', 'broken.example', 'last.example']);
      ipc.seedReposList.set([]);

      const failures = await ipc.scanAuthorisedScope();

      expect(scanned).toEqual(['first.example', 'broken.example', 'last.example']);
      expect(failures).toEqual(['broken.example']);
    });

    /**
     * An empty scope must not fall back to a placeholder domain. The previous
     * behaviour assessed a domain the operator had never authorised while
     * presenting the button as having worked.
     */
    it('refuses to scan when nothing is in scope', () => {
      const spy = vi.spyOn(ipc.transport, 'triggerScan').mockResolvedValue(undefined);

      ipc.isAuthorized.set(true);
      ipc.seedDomainsList.set([]);
      ipc.seedReposList.set([]);

      component.triggerScan();

      expect(spy).not.toHaveBeenCalled();
      expect(ipc.activeTab()).toBe('scope');
    });
  });

  describe('scan status reporting', () => {
    /**
     * The status message must be rendered.
     *
     * It was previously written in six places and rendered in none, so every
     * message the application produced about scanning was invisible — the
     * scan button simply did nothing observable when the scope was empty or
     * unauthorised.
     */
    it('renders the status message in the banner', async () => {
      ipc.setScanStatus('Scanning glgroup.com (1 of 3)…');
      fixture.detectChanges();
      await fixture.whenStable();

      const banner = fixture.nativeElement.querySelector('[role="status"]');
      expect(banner).toBeTruthy();
      expect(banner.textContent).toContain('Scanning glgroup.com (1 of 3)…');
    });

    it('shows no banner when there is nothing to say', async () => {
      ipc.setScanStatus('');
      fixture.detectChanges();
      await fixture.whenStable();

      expect(fixture.nativeElement.querySelector('[role="status"]')).toBeNull();
    });

    it('surfaces a refusal to scan an unauthorised scope', async () => {
      ipc.isAuthorized.set(false);

      component.triggerScan();
      fixture.detectChanges();
      await fixture.whenStable();

      const banner = fixture.nativeElement.querySelector('[role="status"]');
      expect(banner.textContent).toContain('Sign the scope authorisation');
      expect(ipc.scanStatusTone()).toBe('warning');
    });

    /**
     * Each target's completion arrives as its own event. Without a guard, the
     * first domain to finish would clear the scanning state and announce that
     * the whole run was complete while later targets were still going.
     */
    it('does not announce completion while targets remain', async () => {
      const seen: string[] = [];
      scanReporting(domain => {
        if (domain) {
          seen.push(ipc.scanStatusMessage());
        }
      });

      ipc.seedDomainsList.set(['one.example', 'two.example']);
      ipc.seedReposList.set([]);

      await ipc.scanAuthorisedScope();

      // Mid-run the status must still name the target being scanned, not
      // claim the scan has finished.
      expect(seen[0]).toContain('one.example');
      expect(seen[0]).not.toContain('Scan complete');
    });

    /**
     * Requesting a scan is not the same as finishing one.
     *
     * The desktop binding runs the scan in a goroutine and a long-lived
     * container answers 202, so `triggerScan` resolves at acceptance. Treating
     * that as completion announced "Scan complete. Findings have been
     * ingested." while every target was still being assessed — and the
     * findings the message referred to had not been ingested at all.
     */
    it('waits for the work to report back, not for the request to be accepted', async () => {
      const report: Array<() => void> = [];
      // Accepts the request and detaches, exactly as the backend does.
      vi.spyOn(ipc.transport, 'triggerScan').mockImplementation(async (domain: string) => {
        if (domain) {
          report.push(() => completeOneTarget());
        }
      });

      ipc.seedDomainsList.set(['one.example', 'two.example']);
      ipc.seedReposList.set([]);

      const run = ipc.scanAuthorisedScope();
      // Let every request be issued and accepted.
      await new Promise(r => setTimeout(r, 0));

      expect(ipc.isScanning()).toBe(true);
      expect(ipc.scanStatusMessage()).not.toContain('Scan complete');

      report.forEach(fire => fire());
      await run;

      expect(ipc.isScanning()).toBe(false);
      expect(ipc.scanStatusMessage()).toContain('Scan complete');
    });

    /**
     * A target whose request was rejected will never report back, so it must
     * come off the outstanding count. Otherwise the run waits for a result
     * that cannot arrive and the banner spins until the timeout.
     */
    it('settles when the only target failed outright', async () => {
      vi.spyOn(ipc.transport, 'triggerScan').mockRejectedValue(new Error('refused'));

      ipc.seedDomainsList.set(['broken.example']);
      ipc.seedReposList.set([]);

      const failures = await ipc.scanAuthorisedScope();

      expect(failures).toEqual(['broken.example']);
      expect(ipc.isScanning()).toBe(false);
    });

    /**
     * A scan that ran but did not conclude must not be announced as complete.
     *
     * The desktop binding used to log the error and emit a bare completion
     * event, so a scan that failed halfway was indistinguishable from a clean
     * one. The UI settled on the reassuring reading — the dangerous direction
     * to be wrong in, because a partial assessment presented as a complete one
     * invites the conclusion that there is no exposure.
     */
    it('reports a target the backend says did not complete', async () => {
      vi.spyOn(ipc.transport, 'triggerScan').mockImplementation(async (domain: string) => {
        completeWithOutcome({
          phase: 'complete',
          domain,
          status: domain === 'broken.example' ? 'completed-with-failures' : 'completed',
          error: 'resolution failed'
        });
      });

      ipc.seedDomainsList.set(['ok.example', 'broken.example']);
      ipc.seedReposList.set([]);

      const failures = await ipc.scanAuthorisedScope();

      // The request succeeded, so it is not a request failure — but the scan
      // behind it did not conclude, and that must still be surfaced.
      expect(failures).toEqual([]);
      expect(ipc.scanStatusTone()).toBe('warning');
      expect(ipc.scanStatusMessage()).toContain('broken.example');
      expect(ipc.scanStatusMessage()).not.toContain('Scan complete');
      expect(ipc.lastPartialTargets()).toEqual(['broken.example']);
    });

    /**
     * An older backend emits a completion event with no status at all.
     * Inventing a failure from its absence would be worse than missing one.
     */
    it('treats a completion carrying no status as clean', async () => {
      scanReporting();

      ipc.seedDomainsList.set(['ok.example']);
      ipc.seedReposList.set([]);

      await ipc.scanAuthorisedScope();

      expect(ipc.scanStatusTone()).toBe('success');
      expect(ipc.lastPartialTargets()).toEqual([]);
    });

    /**
     * The header writes its own closing line when no request was rejected.
     * It must not overwrite a partial result with a success message, since
     * the whole point is that the estate is not fully assessed.
     */
    it('does not let the header overwrite a partial result with success', async () => {
      vi.spyOn(ipc.transport, 'triggerScan').mockImplementation(async (domain: string) => {
        completeWithOutcome({
          phase: 'complete', domain,
          status: 'completed-with-failures', error: 'resolution failed'
        });
      });

      ipc.isAuthorized.set(true);
      ipc.seedDomainsList.set(['broken.example']);
      ipc.seedReposList.set([]);

      component.triggerScan();
      await new Promise(r => setTimeout(r, 0));

      expect(ipc.scanStatusMessage()).toContain('broken.example');
      expect(ipc.scanStatusTone()).toBe('warning');
    });
  });
});
