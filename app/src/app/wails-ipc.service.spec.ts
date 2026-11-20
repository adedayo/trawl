import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { WailsIpcService } from './wails-ipc.service';
import { findingHash } from './dashboard/findings';
import { changeSet, detail, summary } from './dashboard/findings.fixtures';

/**
 * The finding address is `#/findings/<key>`, kept in step with
 * `selectedFindingKey`. These run against the real `window.location`, so each
 * test starts from a clean address and the service is created lazily, after
 * the address a test wants to arrive at has been set.
 */
function setAddress(hash: string): void {
  window.history.replaceState(null, '', `${window.location.pathname}${hash}`);
}

/**
 * Puts a known, finding-free entry under whatever the test does next. History
 * is shared by every test in the file, so without this "back" would return to
 * an address a previous test left behind.
 */
function startFromCleanEntry(): void {
  window.history.pushState(null, '', `${window.location.pathname}#/`);
}

/**
 * Waits for a condition rather than for the next `hashchange`: jsdom delivers
 * those asynchronously, so an event left over from an earlier test can arrive
 * during this one and be mistaken for its own.
 */
async function until(condition: () => boolean, timeoutMs = 1000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!condition() && Date.now() < deadline) {
    await new Promise(resolve => setTimeout(resolve, 5));
  }
}

describe('WailsIpcService finding address', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
    setAddress('');
  });

  afterEach(() => {
    setAddress('');
  });

  it('opens straight onto the finding an address names', () => {
    setAddress(findingHash('sig.example.com.spf-missing'));
    const ipc = TestBed.inject(WailsIpcService);

    expect(ipc.selectedFindingKey()).toBe('sig.example.com.spf-missing');
    expect(ipc.activeTab()).toBe('findings');
  });

  it('starts on the usual tab when the address names no finding', () => {
    setAddress('#/something-else');
    const ipc = TestBed.inject(WailsIpcService);

    expect(ipc.selectedFindingKey()).toBeNull();
    expect(ipc.activeTab()).toBe('executive');
  });

  it('gives an opened finding an address, and opens the Findings tab', () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.activeTab.set('email');

    ipc.openFinding('key-1');

    expect(ipc.selectedFindingKey()).toBe('key-1');
    expect(ipc.activeTab()).toBe('findings');
    expect(window.location.hash).toBe(findingHash('key-1'));
  });

  it('round-trips an address: what openFinding writes, a new session reads back', () => {
    const first = TestBed.inject(WailsIpcService);
    first.openFinding('a.b-c_1');
    const address = window.location.hash;

    TestBed.resetTestingModule();
    setAddress(address);
    const second = TestBed.inject(WailsIpcService);

    expect(second.selectedFindingKey()).toBe('a.b-c_1');
    expect(second.activeTab()).toBe('findings');
  });

  it('follows the address when it changes, as the back and forward buttons do', async () => {
    const ipc = TestBed.inject(WailsIpcService);

    window.location.hash = findingHash('typed-by-hand');
    await until(() => ipc.selectedFindingKey() === 'typed-by-hand');

    expect(ipc.selectedFindingKey()).toBe('typed-by-hand');
    expect(ipc.activeTab()).toBe('findings');
  });

  it('closes the detail when the browser goes back', async () => {
    startFromCleanEntry();
    const ipc = TestBed.inject(WailsIpcService);
    ipc.openFinding('key-1');
    expect(ipc.selectedFindingKey()).toBe('key-1');

    window.history.back();
    await until(() => ipc.selectedFindingKey() === null);

    expect(ipc.selectedFindingKey()).toBeNull();
    expect(window.location.hash).toBe('#/');
  });

  it('closing walks the history entry back instead of leaving it behind', async () => {
    startFromCleanEntry();
    const ipc = TestBed.inject(WailsIpcService);
    ipc.openFinding('key-1');

    ipc.closeFinding();
    expect(ipc.selectedFindingKey()).toBeNull();
    await until(() => window.location.hash === '#/');

    expect(window.location.hash).toBe('#/');
  });

  it('clears the address in place when the detail was opened by arriving at it', () => {
    setAddress(findingHash('shared'));
    const ipc = TestBed.inject(WailsIpcService);

    ipc.closeFinding();

    expect(ipc.selectedFindingKey()).toBeNull();
    expect(window.location.hash).toBe('');
  });

  it('returns to the tab the detail was opened from', () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.activeTab.set('secrets');
    ipc.openFinding('key-1');
    expect(ipc.activeTab()).toBe('findings');

    ipc.closeFinding();

    expect(ipc.activeTab()).toBe('secrets');
  });

  it('replaces the entry rather than stacking another when a second finding is opened', () => {
    const ipc = TestBed.inject(WailsIpcService);
    const before = window.history.length;
    ipc.openFinding('one');
    ipc.openFinding('two');

    expect(ipc.selectedFindingKey()).toBe('two');
    expect(window.history.length).toBe(before + 1);
  });

  it('closes the detail when the operator leaves the Findings tab', () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.openFinding('key-1');

    ipc.activeTab.set('overview');
    TestBed.tick();

    expect(ipc.selectedFindingKey()).toBeNull();
    expect(ipc.activeTab()).toBe('overview');
  });

  it('ignores an empty key', () => {
    const ipc = TestBed.inject(WailsIpcService);
    ipc.openFinding('');
    expect(ipc.selectedFindingKey()).toBeNull();
    expect(window.location.hash).toBe('');
  });
});

describe('WailsIpcService findings read model', () => {
  let ipc: WailsIpcService;

  beforeEach(() => {
    TestBed.resetTestingModule();
    setAddress('');
    ipc = TestBed.inject(WailsIpcService);
  });

  it('loads the unified index and marks it ready', async () => {
    vi.spyOn(ipc.transport, 'getFindingIndex').mockResolvedValue([summary({ key: 'k1' })]);

    await ipc.refreshFindingIndex();

    expect(ipc.findingIndex().map(f => f.key)).toEqual(['k1']);
    expect(ipc.findingIndexState()).toBe('ready');
  });

  it('keeps the last list and reports the failure when a refresh fails', async () => {
    const spy = vi.spyOn(ipc.transport, 'getFindingIndex');
    spy.mockResolvedValueOnce([summary({ key: 'k1' })]);
    await ipc.refreshFindingIndex();

    spy.mockRejectedValueOnce(new Error('boom'));
    await ipc.refreshFindingIndex();

    // An empty list is a claim about the estate; a failed request must not make it.
    expect(ipc.findingIndex().map(f => f.key)).toEqual(['k1']);
    expect(ipc.findingIndexState()).toBe('error');
    expect(ipc.findingIndexError()).toBe('boom');
  });

  it('refreshes the index when a scan completes', async () => {
    const index = vi.spyOn(ipc.transport, 'getFindingIndex').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getScanRuns').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getHistorySummary').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getFindingChanges').mockResolvedValue(changeSet());
    vi.spyOn(ipc.transport, 'getFindings').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getAssets').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getSecretFindings').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getEmailPostures').mockResolvedValue([]);
    vi.spyOn(ipc.transport, 'getAssessments').mockResolvedValue([]);

    await (ipc as unknown as { onScanComplete(): Promise<void> }).onScanComplete();

    expect(index).toHaveBeenCalled();
    expect(ipc.changeSetState()).toBe('ready');
    expect(ipc.scanRunsState()).toBe('ready');
    expect(ipc.historyState()).toBe('ready');
  });

  it('turns the null lists a Go service serialises into empty ones', async () => {
    vi.spyOn(ipc.transport, 'getFindingChanges').mockResolvedValue({ new: null, resolved: null, worsened: null, improved: null } as never);
    await ipc.refreshChanges();
    expect(ipc.changeSet()?.new).toEqual([]);

    vi.spyOn(ipc.transport, 'getFindingDetail').mockResolvedValue({ ...detail(), timeline: null } as never);
    expect((await ipc.getFindingDetail('x'))?.timeline).toEqual([]);
  });

  it('remembers the history range across refreshes', async () => {
    const spy = vi.spyOn(ipc.transport, 'getHistorySummary').mockResolvedValue([]);
    await ipc.loadHistorySince('2026-10-01T00:00:00.000Z');
    await ipc.refreshFindingViews().catch(() => undefined);
    expect(spy).toHaveBeenLastCalledWith('2026-10-01T00:00:00.000Z');
  });
});
