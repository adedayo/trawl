/**
 * Dependency-update triage runner.
 *
 * Sequence, and the order matters:
 *
 *   1. Parse the machine-readable facts block Renovate writes into the PR body.
 *   2. Run the deterministic classifier. This decides eligibility, alone.
 *   3. Check that Renovate's own labelling agrees with the classifier. Two
 *      independent implementations of one policy; disagreement is a config bug.
 *   4. Ask the agent for a narrative and a veto. It may only WITHHOLD.
 *   5. Emit a job summary, a PR comment, and an exit code.
 *
 * The agent runs last and reads no untrusted input before the gate has already
 * decided. Its output cannot promote an update the classifier ruled ineligible,
 * because promotion is not a value it can return.
 */

import { readFileSync, appendFileSync } from 'node:fs';
import { classifyUpdate, VERDICT, requiredCooldownDays } from './classify-update.mjs';

const FACTS_BLOCK = /<!--\s*trawl-update-facts\s*([\s\S]*?)-->/;

/** Parse the `key=value` facts block Renovate templates into the PR body. */
export function parseFacts(prBody) {
  const match = FACTS_BLOCK.exec(prBody ?? '');
  if (!match) return null;

  const facts = {};
  for (const line of match[1].split('\n')) {
    const idx = line.indexOf('=');
    if (idx === -1) continue;
    const key = line.slice(0, idx).trim();
    const value = line.slice(idx + 1).trim();
    if (key) facts[key] = value;
  }
  return facts;
}

/** Renovate gives a release timestamp; the classifier wants an age in whole days. */
export function releaseAgeDays(releaseTimestamp, now = new Date()) {
  if (!releaseTimestamp || releaseTimestamp === 'undefined') return undefined;
  const released = new Date(releaseTimestamp);
  if (Number.isNaN(released.getTime())) return undefined;
  return Math.floor((now.getTime() - released.getTime()) / 86_400_000);
}

const truthy = (v) => v === 'true' || v === true;

export function toUpdate(facts, now = new Date()) {
  return {
    packageName: facts.packageName,
    updateType: facts.updateType,
    currentVersion: facts.currentVersion,
    newVersion: facts.newVersion,
    depType: facts.depType,
    releaseAgeDays: releaseAgeDays(facts.releaseTimestamp, now),
    testsPassed: truthy(facts.testsPassed),
    isSecurityAdvisory: truthy(facts.isSecurityAdvisory),
  };
}

/**
 * Renovate's packageRules and the classifier encode the same policy independently.
 * If Renovate did not label a PR for human review but the classifier says it needs
 * it, the configuration has drifted and the safe reading is that the config is wrong.
 */
export function checkPolicyConsistency(classification, labels) {
  const renovateSaysHuman =
    labels.includes('needs-human-review') ||
    labels.includes('security-relevant') ||
    labels.includes('major-update') ||
    labels.includes('pre-1.0');

  const classifierSaysHuman = classification.verdict === VERDICT.HUMAN_REVIEW;

  if (classifierSaysHuman && !renovateSaysHuman) {
    return {
      consistent: false,
      message:
        'Policy drift: the classifier requires human review but renovate.json did not label ' +
        'this PR for it. One of the two is wrong. Treating as human-review and failing the ' +
        'gate, because a silent disagreement between the two policy implementations is the ' +
        'condition this check exists to catch.',
    };
  }
  return { consistent: true };
}

const AGENT_SYSTEM_PROMPT = `You are a dependency-update triage assistant for a security tool.

You are advisory. A deterministic gate has ALREADY decided whether this update is
eligible for auto-merge, and your response cannot change that decision. You have
exactly one power: withholding approval.

Assess the update for risk signals a test suite would not catch:
  - maintainer or ownership change, repository transfer
  - new or changed install/postinstall scripts
  - a sudden increase in transitive dependencies
  - unusual release cadence, a yanked-then-republished version
  - changelog text that does not match the version bump
  - anything suggesting a compromised publishing pipeline

Respond with ONLY a JSON object:
{"concern": true|false, "confidence": "low"|"medium"|"high", "narrative": "2-4 sentences"}

Set "concern": true if you see a reason a human should look before this merges.
Absence of information is not reassurance: if you cannot assess the update, say so
in the narrative and set "concern": true. Never speculate about specifics you have
not been shown.`;

export async function runAgent(update, classification, env = process.env) {
  const baseUrl = env.AI_BASE_URL;
  const apiKey = env.AI_API_KEY;
  const model = env.AI_MODEL;

  if (!baseUrl || !apiKey || !model) {
    return { available: false, concern: false, narrative: 'Agentic triage is not configured.' };
  }

  const userPrompt = [
    `Package: ${update.packageName}`,
    `Update: ${update.currentVersion} -> ${update.newVersion} (${update.updateType})`,
    `Dependency type: ${update.depType}`,
    `Days since release: ${update.releaseAgeDays ?? 'unknown'}`,
    `Security advisory: ${update.isSecurityAdvisory ? 'yes' : 'no'}`,
    `Deterministic verdict: ${classification.verdict}`,
  ].join('\n');

  try {
    const response = await fetch(`${baseUrl.replace(/\/$/, '')}/chat/completions`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', authorization: `Bearer ${apiKey}` },
      body: JSON.stringify({
        model,
        temperature: 0,
        messages: [
          { role: 'system', content: AGENT_SYSTEM_PROMPT },
          { role: 'user', content: userPrompt },
        ],
      }),
      signal: AbortSignal.timeout(60_000),
    });

    if (!response.ok) {
      return {
        available: false,
        concern: false,
        narrative: `Agent request failed with HTTP ${response.status}.`,
      };
    }

    const payload = await response.json();
    const content = payload?.choices?.[0]?.message?.content ?? '';
    const parsed = JSON.parse(content.replace(/^```(?:json)?\n?|```$/g, '').trim());

    return {
      available: true,
      concern: parsed.concern === true,
      confidence: parsed.confidence ?? 'unknown',
      narrative: String(parsed.narrative ?? '').slice(0, 2000),
    };
  } catch (error) {
    // An agent that cannot be reached, or that returns something unparseable, has
    // not approved anything. Fail closed.
    return { available: false, concern: false, narrative: `Agent unavailable: ${error.message}` };
  }
}

export function buildSummary(update, classification, agent, autoMergePermitted) {
  const lines = [
    '## Dependency triage',
    '',
    `**${update.packageName}** \`${update.currentVersion}\` → \`${update.newVersion}\``,
    '',
    '### Deterministic gate',
    '',
    `**Verdict:** \`${classification.verdict}\``,
    `**Required cooldown:** ${requiredCooldownDays(update)} days ` +
      `(release is ${update.releaseAgeDays ?? 'unknown'} day(s) old)`,
    '',
    ...classification.reasons.map((r) => `- ${r}`),
    '',
    '### Agentic triage',
    '',
    agent.available
      ? `**Concern raised:** ${agent.concern ? 'yes' : 'no'} (confidence: ${agent.confidence})\n\n${agent.narrative}`
      : `_Not available._ ${agent.narrative}`,
    '',
    '### Outcome',
    '',
    autoMergePermitted
      ? 'Auto-merge permitted: the deterministic gate is satisfied and the agent raised no concern.'
      : 'Auto-merge withheld. A human decides this one.',
    '',
    '_The agent is advisory. It can withhold auto-merge; it can never grant it._',
  ];
  return lines.join('\n');
}

async function main() {
  const eventPath = process.env.GITHUB_EVENT_PATH;
  if (!eventPath) {
    console.error('GITHUB_EVENT_PATH is not set; this script runs inside GitHub Actions.');
    process.exit(1);
  }

  const event = JSON.parse(readFileSync(eventPath, 'utf8'));
  const pr = event.pull_request;
  const labels = (pr?.labels ?? []).map((l) => l.name);

  const facts = parseFacts(pr?.body);
  if (!facts) {
    console.error(
      'No trawl-update-facts block found in the PR body. Either this is not a Renovate PR, ' +
        'or renovate.json prBodyNotes has drifted. Failing closed.',
    );
    process.exit(1);
  }

  const update = toUpdate({ ...facts, testsPassed: process.env.TESTS_PASSED ?? 'true' });
  const classification = classifyUpdate(update);
  const consistency = checkPolicyConsistency(classification, labels);
  const agent = await runAgent(update, classification);

  const autoMergePermitted =
    classification.automergeEligible && consistency.consistent && agent.available && !agent.concern;

  let summary = buildSummary(update, classification, agent, autoMergePermitted);
  if (!consistency.consistent) summary += `\n\n> [!WARNING]\n> ${consistency.message}`;

  if (process.env.GITHUB_STEP_SUMMARY) {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, `${summary}\n`);
  }
  if (process.env.GITHUB_OUTPUT) {
    appendFileSync(
      process.env.GITHUB_OUTPUT,
      `verdict=${classification.verdict}\nautomerge_permitted=${autoMergePermitted}\n`,
    );
  }
  console.log(summary);

  // Fail the check only where failing is the point:
  //   - a hard block (red build, cooldown not elapsed)
  //   - policy drift between renovate.json and the classifier
  //   - an update the classifier deemed eligible, where the agent then withheld
  //
  // Human-review PRs pass this check. They were never auto-merge candidates, and
  // failing them would only stop a human from merging something they have read.
  if (classification.verdict === VERDICT.BLOCKED) process.exit(1);
  if (!consistency.consistent) process.exit(1);
  if (classification.automergeEligible && !autoMergePermitted) process.exit(1);
  process.exit(0);
}

if (process.env.GITHUB_EVENT_PATH && !process.env.VITEST) {
  await main();
}
