import { describe, it, expect } from 'vitest';
import {
  classifyUpdate,
  isSecurityRelevant,
  requiredCooldownDays,
  isPreRelease,
  VERDICT,
  COOLDOWN_DAYS,
} from './classify-update.mjs';

/** A routine, eligible update. Individual tests override single fields. */
const base = {
  packageName: 'github.com/google/uuid',
  updateType: 'patch',
  currentVersion: 'v1.6.0',
  newVersion: 'v1.6.1',
  depType: 'require',
  releaseAgeDays: 10,
  testsPassed: true,
  isSecurityAdvisory: false,
};

describe('isSecurityRelevant', () => {
  it.each([
    'golang.org/x/crypto',
    'github.com/ProtonMail/go-crypto',
    'github.com/cloudflare/circl',
    'aead.dev/minisign',
    'github.com/projectdiscovery/subfinder/v2',
    'github.com/adedayo/checkmate',
    'github.com/adedayo/vantage',
    'modernc.org/sqlite',
    'github.com/wailsapp/wails/v2',
  ])('classifies %s as security-relevant', (pkg) => {
    expect(isSecurityRelevant(pkg)).toBe(true);
  });

  it.each(['github.com/google/uuid', 'concurrently', 'dario.cat/mergo'])(
    'does not over-classify %s',
    (pkg) => {
      expect(isSecurityRelevant(pkg)).toBe(false);
    },
  );

  it('handles a missing package name without throwing', () => {
    expect(isSecurityRelevant(undefined)).toBe(false);
  });
});

describe('requiredCooldownDays', () => {
  it('applies the high cooldown to security-relevant packages', () => {
    expect(requiredCooldownDays({ ...base, packageName: 'golang.org/x/crypto' })).toBe(
      COOLDOWN_DAYS.high,
    );
  });

  it('applies the high cooldown to major updates', () => {
    expect(requiredCooldownDays({ ...base, updateType: 'major' })).toBe(COOLDOWN_DAYS.high);
  });

  it('applies the elevated cooldown to indirect dependencies', () => {
    expect(requiredCooldownDays({ ...base, depType: 'indirect' })).toBe(COOLDOWN_DAYS.elevated);
  });

  it('never returns less than the routine floor', () => {
    expect(requiredCooldownDays(base)).toBe(COOLDOWN_DAYS.routine);
    expect(COOLDOWN_DAYS.routine).toBeGreaterThanOrEqual(3);
  });
});

describe('isPreRelease', () => {
  it.each(['0.1.0', 'v0.9.9', '1.0.0-beta.2', 'v2.0.0-rc1', '3.0.0-canary.4'])(
    'treats %s as lacking a semver contract',
    (v) => {
      expect(isPreRelease(v)).toBe(true);
    },
  );

  it.each(['1.0.0', 'v1.6.0', 'v2.14.0'])('treats %s as stable', (v) => {
    expect(isPreRelease(v)).toBe(false);
  });
});

describe('classifyUpdate — hard blocks', () => {
  it('blocks on failing tests, whatever else is true', () => {
    const r = classifyUpdate({ ...base, testsPassed: false });
    expect(r.verdict).toBe(VERDICT.BLOCKED);
    expect(r.automergeEligible).toBe(false);
  });

  it('blocks before the cooldown has elapsed', () => {
    const r = classifyUpdate({ ...base, releaseAgeDays: 1 });
    expect(r.verdict).toBe(VERDICT.BLOCKED);
    expect(r.reasons[0]).toMatch(/cooldown/i);
  });

  it('applies the cooldown to patch releases too — nothing is exempt', () => {
    const r = classifyUpdate({ ...base, updateType: 'patch', releaseAgeDays: 2 });
    expect(r.verdict).toBe(VERDICT.BLOCKED);
  });

  it('applies the longer cooldown to container image tags via the indirect/elevated path', () => {
    const r = classifyUpdate({
      ...base,
      packageName: 'alpine',
      depType: 'indirect',
      releaseAgeDays: 5,
    });
    expect(r.verdict).toBe(VERDICT.BLOCKED);
  });
});

describe('classifyUpdate — human-review routes', () => {
  it('routes major-version bumps to a human even when everything else is green', () => {
    const r = classifyUpdate({
      ...base,
      updateType: 'major',
      currentVersion: 'v1.6.0',
      newVersion: 'v2.0.0',
      releaseAgeDays: 30,
    });
    expect(r.verdict).toBe(VERDICT.HUMAN_REVIEW);
    expect(r.automergeEligible).toBe(false);
  });

  it('routes security-relevant packages to a human even for a patch bump', () => {
    const r = classifyUpdate({
      ...base,
      packageName: 'github.com/projectdiscovery/subfinder/v2',
      updateType: 'patch',
      releaseAgeDays: 30,
    });
    expect(r.verdict).toBe(VERDICT.HUMAN_REVIEW);
    expect(r.reasons.join(' ')).toMatch(/security-relevant/i);
  });

  it('routes pre-1.0 packages to a human', () => {
    const r = classifyUpdate({
      ...base,
      packageName: 'some/experimental',
      currentVersion: '0.3.1',
      newVersion: '0.3.2',
      releaseAgeDays: 30,
    });
    expect(r.verdict).toBe(VERDICT.HUMAN_REVIEW);
  });

  it('routes published security advisories to a human rather than fast-merging them', () => {
    const r = classifyUpdate({ ...base, isSecurityAdvisory: true, releaseAgeDays: 30 });
    expect(r.verdict).toBe(VERDICT.HUMAN_REVIEW);
    expect(r.automergeEligible).toBe(false);
  });

  it('routes lock-file maintenance to a human', () => {
    const r = classifyUpdate({ ...base, updateType: 'lockFileMaintenance', releaseAgeDays: 30 });
    expect(r.verdict).toBe(VERDICT.HUMAN_REVIEW);
  });

  it('accumulates every applicable reason rather than reporting only the first', () => {
    const r = classifyUpdate({
      ...base,
      packageName: 'golang.org/x/crypto',
      updateType: 'major',
      newVersion: 'v2.0.0',
      releaseAgeDays: 30,
    });
    expect(r.reasons.length).toBeGreaterThanOrEqual(2);
  });
});

describe('classifyUpdate — eligibility', () => {
  it('marks a routine patch update eligible once the cooldown has elapsed', () => {
    const r = classifyUpdate(base);
    expect(r.verdict).toBe(VERDICT.ELIGIBLE);
    expect(r.automergeEligible).toBe(true);
  });

  it('states that eligibility is not merge — the agent must still agree', () => {
    const r = classifyUpdate(base);
    expect(r.reasons.join(' ')).toMatch(/agentic triage check/i);
  });

  it('is a pure function: identical input yields identical output', () => {
    expect(classifyUpdate(base)).toEqual(classifyUpdate(base));
  });
});
