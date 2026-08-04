/**
 * Deterministic dependency-update classifier.
 *
 * This is THE GATE. It is a pure function of the pull request's facts, with no
 * network access, no AI, and no configuration it can be talked out of. The agentic
 * triage layer runs afterwards and may only WITHHOLD approval; it can never make an
 * update eligible that this function has ruled ineligible.
 *
 * That asymmetry is the whole design. An agent that can promote is an agent that can
 * be prompt-injected by a changelog. An agent that can only veto cannot.
 */

/** Package name patterns that always require human approval, regardless of update size. */
export const SECURITY_RELEVANT_PATTERNS = [
  // Cryptography, authentication, signing
  /^golang\.org\/x\/crypto/,
  /^golang\.org\/x\/net/,
  /^golang\.org\/x\/oauth2/,
  /^github\.com\/ProtonMail\/go-crypto/,
  /^github\.com\/cloudflare\/circl/,
  /^aead\.dev\/minisign/,
  /^jose$/,
  /^jsonwebtoken$/,
  /^node-forge$/,

  // The scanning and assessment tooling itself
  /^github\.com\/projectdiscovery\//,
  /^github\.com\/adedayo\/checkmate/,
  /^github\.com\/adedayo\/vantage/,
  /^github\.com\/owasp-amass\//,
  /^github\.com\/zricethezav\/gitleaks/,
  /^github\.com\/trufflesecurity\//,

  // Storage and desktop runtime — a compromise here is a compromise of the store
  /^modernc\.org\/sqlite/,
  /^github\.com\/wailsapp\/wails/,
];

/** Minimum release age in days, by risk category. */
export const COOLDOWN_DAYS = {
  routine: 3,
  elevated: 7,
  high: 14,
};

export const VERDICT = {
  ELIGIBLE: 'eligible-for-automerge',
  HUMAN_REVIEW: 'requires-human-review',
  BLOCKED: 'blocked',
};

export function isSecurityRelevant(packageName) {
  if (!packageName) return false;
  return SECURITY_RELEVANT_PATTERNS.some((p) => p.test(packageName));
}

export function requiredCooldownDays(update) {
  if (isSecurityRelevant(update.packageName)) return COOLDOWN_DAYS.high;
  if (update.updateType === 'major') return COOLDOWN_DAYS.high;
  if (update.depType === 'indirect') return COOLDOWN_DAYS.elevated;
  return COOLDOWN_DAYS.routine;
}

/** Pre-1.0 packages carry no semver contract: a patch bump may break anything. */
export function isPreRelease(version) {
  if (!version) return false;
  return /^v?0\./.test(version) || /-(alpha|beta|rc|next|canary|dev)/i.test(version);
}

/**
 * @param {object} update
 * @param {string}  update.packageName
 * @param {string}  update.updateType      'major' | 'minor' | 'patch' | 'digest' | 'pin' | 'lockFileMaintenance'
 * @param {string}  update.currentVersion
 * @param {string}  update.newVersion
 * @param {string}  update.depType
 * @param {number}  update.releaseAgeDays  age of the new release, in days
 * @param {boolean} update.testsPassed     deterministic CI result
 * @param {boolean} update.isSecurityAdvisory
 * @returns {{verdict: string, reasons: string[], automergeEligible: boolean}}
 */
export function classifyUpdate(update) {
  const reasons = [];

  // --- Hard blocks: no verdict other than "not now" is available. ---

  if (update.testsPassed === false) {
    return {
      verdict: VERDICT.BLOCKED,
      reasons: ['Required CI checks did not pass. No update merges on a red build.'],
      automergeEligible: false,
    };
  }

  const cooldown = requiredCooldownDays(update);
  if (typeof update.releaseAgeDays === 'number' && update.releaseAgeDays < cooldown) {
    return {
      verdict: VERDICT.BLOCKED,
      reasons: [
        `Release is ${update.releaseAgeDays} day(s) old; ${cooldown}-day cooldown has not elapsed. ` +
          'The cooldown exists so that a compromised-maintainer release has time to be detected and yanked ' +
          'before it reaches this repository.',
      ],
      automergeEligible: false,
    };
  }

  // --- Human-review routes. Each is unconditional: no agent opinion overrides them. ---

  if (isSecurityRelevant(update.packageName)) {
    reasons.push(
      `${update.packageName} is classified security-relevant (cryptography, authentication, ` +
        'scanning tooling, or the storage/desktop runtime). These always require human approval.',
    );
  }

  if (update.updateType === 'major') {
    reasons.push('Major-version bump. Always human-reviewed, regardless of test results.');
  }

  if (isPreRelease(update.currentVersion) || isPreRelease(update.newVersion)) {
    reasons.push(
      'Pre-1.0 or pre-release version. Semver gives no breaking-change guarantee here, ' +
        'so the update class cannot be trusted to describe the risk.',
    );
  }

  if (update.isSecurityAdvisory) {
    reasons.push(
      'Published security advisory. A human weighs the exposure being closed against the ' +
        'shortened supply-chain cooldown, with the advisory in front of them.',
    );
  }

  if (update.updateType === 'lockFileMaintenance') {
    reasons.push('Lock-file maintenance touches the full transitive graph. Human-reviewed.');
  }

  if (reasons.length > 0) {
    return { verdict: VERDICT.HUMAN_REVIEW, reasons, automergeEligible: false };
  }

  // --- Eligible. Note: eligible is not merged. The agentic triage check must also pass. ---

  return {
    verdict: VERDICT.ELIGIBLE,
    reasons: [
      `${update.updateType} update to ${update.packageName}, not security-relevant, ` +
        `${update.releaseAgeDays ?? '>='}${typeof update.releaseAgeDays === 'number' ? '' : cooldown} day(s) since release, required checks green.`,
      'Deterministic gate is satisfied. Auto-merge still requires the agentic triage check to agree.',
    ],
    automergeEligible: true,
  };
}
