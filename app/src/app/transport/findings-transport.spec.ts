import { vi } from 'vitest';

import { HttpTransport } from './http.transport';
import { WailsTransport } from './wails.transport';
import { detail, summary } from '../dashboard/findings.fixtures';

/**
 * The Change 025 read paths, on both transports.
 *
 * What these pin down is the one property the older read paths deliberately do
 * not have: they reject when they cannot answer. An empty findings list and a
 * failed request render the same, and only one is a statement about the estate.
 */

function respond(status: number, body: unknown = null) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  });
}

describe('HttpTransport findings read model', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('asks for the index with only the filters that were given, encoded', async () => {
    const fetchMock = respond(200, [summary()]);
    vi.stubGlobal('fetch', fetchMock);

    await new HttpTransport().getFindingIndex({ status: 'active', assetId: 'a b', changedSince: '2026-11-01T00:00:00Z' });

    expect(fetchMock.mock.calls[0][0]).toBe(
      '/api/v1/findings/index?status=active&assetId=a%20b&changedSince=2026-11-01T00%3A00%3A00Z'
    );
  });

  it('asks for the whole index when no filter is given', async () => {
    const fetchMock = respond(200, []);
    vi.stubGlobal('fetch', fetchMock);
    await new HttpTransport().getFindingIndex();
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/findings/index');
  });

  it('rejects when the index cannot be read, instead of returning an empty list', async () => {
    vi.stubGlobal('fetch', respond(500));
    await expect(new HttpTransport().getFindingIndex()).rejects.toThrow(/500/);

    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network down')));
    await expect(new HttpTransport().getFindingIndex()).rejects.toThrow('network down');
  });

  it('fetches a finding by its encoded key', async () => {
    const fetchMock = respond(200, detail());
    vi.stubGlobal('fetch', fetchMock);

    const result = await new HttpTransport().getFindingDetail('a/b c');

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/findings/a%2Fb%20c');
    expect(result?.key).toBe(detail().key);
  });

  it('reports an unknown key as absent, and any other failure as a rejection', async () => {
    vi.stubGlobal('fetch', respond(404));
    expect(await new HttpTransport().getFindingDetail('gone')).toBeNull();

    vi.stubGlobal('fetch', respond(503));
    await expect(new HttpTransport().getFindingDetail('k')).rejects.toThrow(/503/);
  });

  it('omits the run bounds the caller left out', async () => {
    const fetchMock = respond(200, { new: [], resolved: [], worsened: [], improved: [] });
    vi.stubGlobal('fetch', fetchMock);
    const transport = new HttpTransport();

    await transport.getFindingChanges();
    await transport.getFindingChanges('run-1', 'run-2');
    await transport.getFindingChanges('', 'run-2');

    expect(fetchMock.mock.calls.map(c => c[0])).toEqual([
      '/api/v1/findings/changes',
      '/api/v1/findings/changes?from=run-1&to=run-2',
      '/api/v1/findings/changes?to=run-2'
    ]);
  });

  it('reads the scan ledger and the history summary', async () => {
    const fetchMock = respond(200, []);
    vi.stubGlobal('fetch', fetchMock);
    const transport = new HttpTransport();

    await transport.getScanRuns();
    await transport.getHistorySummary();
    await transport.getHistorySummary('2026-10-01T00:00:00Z');

    expect(fetchMock.mock.calls.map(c => c[0])).toEqual([
      '/api/v1/scans',
      '/api/v1/history/summary',
      '/api/v1/history/summary?since=2026-10-01T00%3A00%3A00Z'
    ]);
  });

  it('rejects the ledger and the history when they cannot be read', async () => {
    vi.stubGlobal('fetch', respond(500));
    await expect(new HttpTransport().getScanRuns()).rejects.toThrow();
    await expect(new HttpTransport().getHistorySummary()).rejects.toThrow();
    await expect(new HttpTransport().getFindingChanges()).rejects.toThrow();
  });
});

describe('WailsTransport findings read model', () => {
  const g = globalThis as unknown as { go?: unknown };

  afterEach(() => {
    delete g.go;
  });

  function bind(app: Record<string, unknown>): void {
    g.go = { main: { App: app } };
  }

  it('calls the bindings named in the contract with the arguments in order', async () => {
    const app = {
      GetFindingIndex: vi.fn().mockResolvedValue([summary()]),
      GetFindingDetail: vi.fn().mockResolvedValue(detail()),
      GetFindingChanges: vi.fn().mockResolvedValue({ new: [], resolved: [], worsened: [], improved: [] }),
      GetScanRuns: vi.fn().mockResolvedValue([]),
      GetHistorySummary: vi.fn().mockResolvedValue([])
    };
    bind(app);
    const transport = new WailsTransport();

    await transport.getFindingIndex({ status: 'open' });
    await transport.getFindingDetail('k');
    await transport.getFindingChanges('r1', 'r2');
    await transport.getFindingChanges();
    await transport.getScanRuns();
    await transport.getHistorySummary('2026-10-01T00:00:00Z');

    expect(app.GetFindingIndex).toHaveBeenCalledWith({ status: 'open' });
    expect(app.GetFindingDetail).toHaveBeenCalledWith('k');
    expect(app.GetFindingChanges).toHaveBeenNthCalledWith(1, 'r1', 'r2');
    expect(app.GetFindingChanges).toHaveBeenNthCalledWith(2, '', '');
    expect(app.GetHistorySummary).toHaveBeenCalledWith('2026-10-01T00:00:00Z');
  });

  it('rejects when the binding is missing, because an older desktop binary has no answer', async () => {
    bind({});
    const transport = new WailsTransport();
    await expect(transport.getFindingIndex()).rejects.toThrow(/unavailable/);
    await expect(transport.getScanRuns()).rejects.toThrow(/unavailable/);
    await expect(transport.getHistorySummary()).rejects.toThrow(/unavailable/);
    await expect(transport.getFindingChanges()).rejects.toThrow(/unavailable/);
    await expect(transport.getFindingDetail('k')).rejects.toThrow(/unavailable/);
  });

  it('reads an unknown key as absent but not a missing binding', async () => {
    bind({ GetFindingDetail: vi.fn().mockRejectedValue(new Error('finding not found')) });
    expect(await new WailsTransport().getFindingDetail('gone')).toBeNull();

    bind({ GetFindingDetail: vi.fn().mockRejectedValue(new Error('database is locked')) });
    await expect(new WailsTransport().getFindingDetail('k')).rejects.toThrow('database is locked');
  });
});
