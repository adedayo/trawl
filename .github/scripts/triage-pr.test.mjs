import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  parseFacts,
  releaseAgeDays,
  toUpdate,
  checkPolicyConsistency,
  runAgent,
  buildSummary,
} from './triage-pr.mjs';
import { VERDICT } from './classify-update.mjs';

const NOW = new Date('2026-08-04T12:00:00Z');

const factsBlock = (overrides = {}) => {
  const facts = {
    packageName: 'github.com/miekg/dns',
    updateType: 'patch',
    currentVersion: '1.1.62',
    newVersion: '1.1.63',
    depType: 'require',
    manager: 'gomod',
    releaseTimestamp: '2026-07-01T00:00:00Z',
    isSecurityAdvisory: 'false',
    ...overrides,
  };
  const body = Object.entries(facts)
    .map(([k, v]) => `${k}=${v}`)
    .join('\n');
  return `This PR contains the following updates.\n\n<!-- trawl-update-facts\n${body}\n-->\n\nFooter text.`;
};

describe('parseFacts', () => {
  it('extracts every key from the facts block', () => {
    const facts = parseFacts(factsBlock());
    expect(facts.packageName).toBe('github.com/miekg/dns');
    expect(facts.updateType).toBe('patch');
    expect(facts.newVersion).toBe('1.1.63');
  });

  it('returns null when the block is absent, so the caller can fail closed', () => {
    expect(parseFacts('An ordinary pull request body.')).toBeNull();
    expect(parseFacts('')).toBeNull();
    expect(parseFacts(undefined)).toBeNull();
  });

  it('tolerates values containing "=" without truncating them', () => {
    const facts = parseFacts('<!-- trawl-update-facts\nnote=a=b=c\n-->');
    expect(facts.note).toBe('a=b=c');
  });

  it('ignores lines with no key', () => {
    const facts = parseFacts('<!-- trawl-update-facts\n\nfoo=bar\n   \n-->');
    expect(Object.keys(facts)).toEqual(['foo']);
  });
});

describe('releaseAgeDays', () => {
  it('floors the elapsed days rather than rounding up', () => {
    expect(releaseAgeDays('2026-08-01T13:00:00Z', NOW)).toBe(2);
  });

  it('returns undefined for missing, literal-undefined, and unparseable stamps', () => {
    // Renovate templates the string "undefined" when it has no timestamp. Treating
    // that as age zero would silently pass an update whose age is unknown.
    expect(releaseAgeDays(undefined, NOW)).toBeUndefined();
    expect(releaseAgeDays('undefined', NOW)).toBeUndefined();
    expect(releaseAgeDays('not-a-date', NOW)).toBeUndefined();
    expect(releaseAgeDays('', NOW)).toBeUndefined();
  });
});

describe('toUpdate', () => {
  it('maps the facts block onto the classifier input shape', () => {
    const update = toUpdate(parseFacts(factsBlock()), NOW);
    expect(update).toMatchObject({
      packageName: 'github.com/miekg/dns',
      updateType: 'patch',
      depType: 'require',
      releaseAgeDays: 34,
      isSecurityAdvisory: false,
    });
  });

  it('coerces only the exact string "true" to true', () => {
    expect(toUpdate({ isSecurityAdvisory: 'true' }, NOW).isSecurityAdvisory).toBe(true);
    expect(toUpdate({ isSecurityAdvisory: 'TRUE' }, NOW).isSecurityAdvisory).toBe(false);
    expect(toUpdate({ isSecurityAdvisory: '1' }, NOW).isSecurityAdvisory).toBe(false);
    expect(toUpdate({}, NOW).isSecurityAdvisory).toBe(false);
  });
});

describe('checkPolicyConsistency', () => {
  const humanReview = { verdict: VERDICT.HUMAN_REVIEW };
  const eligible = { verdict: VERDICT.ELIGIBLE };

  it('flags drift when the classifier wants a human and Renovate did not label for it', () => {
    const result = checkPolicyConsistency(humanReview, ['dependencies']);
    expect(result.consistent).toBe(false);
    expect(result.message).toMatch(/Policy drift/);
  });

  it.each(['needs-human-review', 'security-relevant', 'major-update', 'pre-1.0'])(
    'accepts %s as Renovate agreeing that a human is needed',
    (label) => {
      expect(checkPolicyConsistency(humanReview, ['dependencies', label]).consistent).toBe(true);
    },
  );

  it('does not flag drift when the classifier is content', () => {
    // Renovate being more cautious than the classifier is not an error. Only the
    // opposite direction — the classifier wanting review that Renovate did not
    // route — represents a gap through which something could auto-merge.
    expect(checkPolicyConsistency(eligible, ['dependencies']).consistent).toBe(true);
    expect(checkPolicyConsistency(eligible, ['security-relevant']).consistent).toBe(true);
  });
});

describe('runAgent', () => {
  afterEach(() => vi.unstubAllGlobals());

  const update = {
    packageName: 'left-pad',
    currentVersion: '1.0.0',
    newVersion: '1.0.1',
    updateType: 'patch',
    depType: 'dependencies',
    releaseAgeDays: 5,
    isSecurityAdvisory: false,
  };
  const classification = { verdict: VERDICT.ELIGIBLE };
  const env = { AI_BASE_URL: 'https://api.test/v1', AI_API_KEY: 'k', AI_MODEL: 'm' };

  const respondWith = (content) =>
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({ choices: [{ message: { content } }] }),
      })),
    );

  it('reports unavailable when credentials are absent', async () => {
    const result = await runAgent(update, classification, {});
    expect(result.available).toBe(false);
    expect(result.concern).toBe(false);
  });

  it.each([
    ['AI_BASE_URL', { ...env, AI_BASE_URL: undefined }],
    ['AI_API_KEY', { ...env, AI_API_KEY: undefined }],
    ['AI_MODEL', { ...env, AI_MODEL: undefined }],
  ])('reports unavailable when %s is missing', async (_name, partial) => {
    expect((await runAgent(update, classification, partial)).available).toBe(false);
  });

  it('parses a well-formed verdict', async () => {
    respondWith('{"concern": true, "confidence": "high", "narrative": "Maintainer changed."}');
    const result = await runAgent(update, classification, env);
    expect(result).toMatchObject({ available: true, concern: true, confidence: 'high' });
    expect(result.narrative).toBe('Maintainer changed.');
  });

  it('parses a verdict wrapped in a markdown fence', async () => {
    respondWith('```json\n{"concern": false, "confidence": "medium", "narrative": "Fine."}\n```');
    expect(await runAgent(update, classification, env)).toMatchObject({
      available: true,
      concern: false,
    });
  });

  it('treats anything other than boolean true as no concern raised', async () => {
    respondWith('{"concern": "true", "narrative": "n"}');
    expect((await runAgent(update, classification, env)).concern).toBe(false);
  });

  it('sends temperature 0 so the same PR produces the same narrative', async () => {
    respondWith('{"concern": false, "narrative": "n"}');
    await runAgent(update, classification, env);
    const body = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
    expect(body.temperature).toBe(0);
    expect(body.model).toBe('m');
  });

  it('does not double the slash when the base URL has a trailing one', async () => {
    respondWith('{"concern": false, "narrative": "n"}');
    await runAgent(update, classification, { ...env, AI_BASE_URL: 'https://api.test/v1/' });
    expect(globalThis.fetch.mock.calls[0][0]).toBe('https://api.test/v1/chat/completions');
  });

  it('is unavailable, not approving, on a non-2xx response', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 503 })));
    const result = await runAgent(update, classification, env);
    expect(result.available).toBe(false);
    expect(result.narrative).toMatch(/503/);
  });

  it('is unavailable, not approving, on unparseable output', async () => {
    respondWith('I think this update is probably fine.');
    expect((await runAgent(update, classification, env)).available).toBe(false);
  });

  it('is unavailable, not approving, when the request throws', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('network down');
      }),
    );
    const result = await runAgent(update, classification, env);
    expect(result.available).toBe(false);
    expect(result.narrative).toMatch(/network down/);
  });

  it('never returns a shape that could promote an ineligible update', async () => {
    // The agent's contract has no field meaning "approve". The only thing it can
    // assert is concern. This test exists to fail if that contract is widened.
    respondWith('{"concern": false, "automergeEligible": true, "verdict": "ELIGIBLE"}');
    const result = await runAgent(update, classification, env);
    expect(result.automergeEligible).toBeUndefined();
    expect(result.verdict).toBeUndefined();
  });
});

describe('buildSummary', () => {
  const update = {
    packageName: 'modernc.org/sqlite',
    currentVersion: '1.34.0',
    newVersion: '1.34.1',
    updateType: 'patch',
    depType: 'require',
    releaseAgeDays: 20,
    isSecurityAdvisory: false,
  };
  const classification = { verdict: VERDICT.ELIGIBLE, reasons: ['Cooldown elapsed.'] };

  it('records the verdict, the reasons, and the outcome', () => {
    const summary = buildSummary(
      update,
      classification,
      { available: true, concern: false, confidence: 'high', narrative: 'Nothing unusual.' },
      true,
    );
    expect(summary).toContain('modernc.org/sqlite');
    expect(summary).toContain('Cooldown elapsed.');
    expect(summary).toContain('Auto-merge permitted');
    expect(summary).toContain('Nothing unusual.');
  });

  it('states plainly when auto-merge was withheld', () => {
    const summary = buildSummary(
      update,
      classification,
      { available: false, concern: false, narrative: 'Agent unavailable: timeout.' },
      false,
    );
    expect(summary).toContain('Auto-merge withheld');
    expect(summary).toContain('_Not available._');
  });

  it('always restates the asymmetry, on every PR, whatever the outcome', () => {
    for (const permitted of [true, false]) {
      const summary = buildSummary(
        update,
        classification,
        { available: true, concern: false, confidence: 'low', narrative: 'n' },
        permitted,
      );
      expect(summary).toContain('It can withhold auto-merge; it can never grant it.');
    }
  });
});
