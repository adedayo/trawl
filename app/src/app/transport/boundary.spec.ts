import { describe, expect, it } from 'vitest';

/**
 * One bundle, two transports.
 *
 * The seam is only worth having if nothing downstream knows which side of it
 * it is on. A component that imports `WailsTransport` directly still compiles,
 * still passes its own unit tests, and still renders — it simply stops working
 * in the container deployment, where the Wails runtime is absent. That failure
 * surfaces at runtime, in the deployment the developer is least likely to be
 * running, which is exactly the kind of defect a test has to catch instead.
 *
 * The rule: concrete transports are named inside `transport/` and nowhere
 * else. Everything downstream depends on the `TrawlTransport` interface and
 * receives its implementation from `createTransport`.
 *
 * Sources are read through `import.meta.glob` rather than the filesystem so
 * the check needs no Node typings and runs wherever the rest of the suite
 * does.
 */

const sources = (
  import.meta as unknown as {
    glob: (
      pattern: string,
      options: { query: string; import: string; eager: true }
    ) => Record<string, string>;
  }
).glob('../**/*.ts', { query: '?raw', import: 'default', eager: true });

const concreteTransports = ['WailsTransport', 'HttpTransport'];

describe('transport boundary', () => {
  it('names concrete transports only inside transport/', () => {
    const offenders = Object.entries(sources)
      // Glob keys are relative to this spec: the seam's own files resolve to
      // './…', everything outside it to '../…'.
      .filter(([path]) => path.startsWith('../'))
      .filter(([, source]) =>
        concreteTransports.some((name) => source.includes(name))
      )
      .map(([path]) => path);

    expect(
      offenders,
      'These files name a concrete transport. Depend on the TrawlTransport ' +
        'interface and let createTransport choose, or the file will only ' +
        'work in one of the two deployments.'
    ).toEqual([]);
  });

  it('scans a non-trivial number of files', () => {
    // A boundary test that silently matches nothing certifies the violation
    // it is failing to look for.
    expect(Object.keys(sources).length).toBeGreaterThan(10);
  });
});
