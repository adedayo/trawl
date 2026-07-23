# Change: 002-susceptibility-scoring-integration

## Status

**Proposed, not started.** Blocked by Change 001 (initial build) reaching Phase 4 (AI triage layer) implemented and archived. This is a future capability, scoped now so the design decision is recorded while it's fresh, not scheduled for immediate work.

## Why

Trawl's findings (KEV/EPSS/CVSS-correlated, tied to a known-internet-facing asset) are exactly the inputs the "Detect, Estimate, Decide" posture-series argument says a continuous sensor should supply: contact status (is this asset reachable), applicability (is the weakness confirmed present, not just theoretically matched), and an EPSS/KEV band for the attempt stage. Today, using a finding in the exploit-probability framework's Prior Estimator or the weakness-removal Pathway Remediation Calculator requires a human to read the finding off the dashboard and hand-transcribe those facts into the calculator's inputs. This change closes that gap: Trawl computes and surfaces the same structured stage estimate the calculators expect, sourced directly from data it already has, so a finding can be copied into either calculator instead of manually re-derived.

## What Changes

- New capability: `susceptibility-scoring`. Extends `ai-triage` (does not replace it): the existing narrative-annotation action gains an additional, deterministic computation that emits structured Applicability/Attempt/Success stage inputs per finding, formatted to match the exploit-probability framework's Prior Estimator input shape.
- Contact status becomes an explicit, surfaced field per finding: Trawl's own asset-inventory/asset-discovery data (internet-facing vs. internal/unknown) is precisely the "network exposure and attack-surface analysis" that the exploit-probability framework names as external to itself.
- The EPSS/KEV-band-to-adjustment mapping from "The Probability Behind the Finding" (§Where EPSS Fits In) is pulled into a versioned config table inside the engine, not hardcoded inline, so it can be updated as the published framework's bands are revised without an engine code change.
- An evidence-ledger row is stored per adjustment applied (named source, direction, weight), mirroring the framework's "chain of custody" requirement — any adjustment can be traced back to the finding/asset field that produced it and removed for recomputation.

## Explicitly Out of Scope

- **No Monte Carlo simulation inside Trawl.** The Pathway Remediation Calculator and Prior Estimator remain the blog's Angular components; Trawl emits the Layer 1/2 deterministic inputs and evidence ledger, never runs the simulation itself.
- **No Layer 3 Bayesian conjugate updates inside Trawl.** Canary-credential firings, IdP authentication logs, incident records, and **DMARC aggregate/forensic reports (`rua`/`ruf`)** are all Layer 3 telemetry that lives outside Trawl's scope — Trawl observes email-authentication *posture* (SPF/DKIM/DMARC records as configured) from the outside via the `email-authentication` capability, but it does not receive mail, so it cannot see actual spoofing attempts the way a domain owner's own DMARC report pipeline does. That report data belongs in the same "own environment telemetry" bucket as IdP logs: a real observation that sharpens an attempt/success-stage estimate, recommended as a datapoint source in the exploit-probability framework's evidence ledger, not something Trawl ingests or computes. A future change could wire Trawl's own scan-history as one Layer 3 observation source (e.g., "a nuclei hit reconfirming an already-known finding" as an applicability-stage observation), but that is not this change.
- **No change to the deterministic-severity/AI-narrative-only boundary.** The stage-estimate computation is a pure function of finding/asset fields plus the versioned adjustment config, exactly like the existing priority-scoring function; AI's role stays limited to the existing narrative annotation.
- **No automatic feed into the firm-level risk model.** Reconciliation with the canonical enterprise risk model (per the Posture-Informed-Risk-Exposure framework's §8) stays a human/quarterly step; this change does not automate that reconciliation.

## Impact

- **No new infrastructure.** Extends the existing Convex schema (`findings`) and the existing `ai-triage` action; no new services.
- **Dashboard addition.** The Angular dashboard gains a per-finding "susceptibility inputs" panel with a copy/export action, alongside the existing severity/priority/AI-annotation display.
- **Portability preserved.** The adjustment-band config table is versioned data, not organization-specific, and lives alongside the other engine config per the `portability-config` capability's existing discipline.
