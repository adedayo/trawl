# Change: 008-risk-model-packs

## Status

**Proposed, not started.** No dependency on 006/007 for the pack format itself; content for the contact and email domains depends on both.

## Why

Change 002 established the principle in miniature: the EPSS/KEV band table is versioned config data, so revisions of the published framework are a data change rather than an engine change. Everything downstream in this arc multiplies the number of such tables — weakness-class priors, contact-rate base rates by service class, variance shares, decay constants, scenario frequency anchors, magnitude percentiles, loss-form overlays. If any of them are compiled in, the model calcifies at its first publication and every source refresh becomes a release.

There is a second, sharper reason. The exploit-probability post's core objection to commercial platforms is that their numbers cannot be interrogated:

> Commercial posture platforms assign proprietary, black-box "risk scores" (0 to 1000) that engineers cannot inspect or challenge.

A pack is the antidote made concrete: every parameter that moves a number is a file a reader can open, with a named source, a verification status and a calibration label attached. If a parameter is not in a pack, it should not exist.

## What Changes

- New capability: `risk-model-packs`. A versioned, signed, self-describing parameter bundle, loaded at runtime and recorded by version on every computed estimate.
- **Generalises Change 002's `adjustmentBands`.** That table becomes one section of a pack rather than a standalone concept; Change 002's requirement that it be data, not code, is preserved and widened.
- **Carries the Layer 2.5 parameters.** Measured-state signals need more than a log-odds shift: a variance share, a dedup group, a decay constant, and a cap. These are the parameters that let a scan narrow an interval rather than only move a mean, and they are the novel content of this change. See `design.md`.
- **Carries scenario anchors** — per-scenario frequency priors and magnitude percentiles derived from published industry loss data, with the taxonomy mapping written down as data rather than assumed. The portfolio-ROI post requires that mapping be explicit: *"A documented mapping is auditable; an implicit one is where models quietly break."*
- **Carries judgment parameters** β₀, m₀ and w with their published sensitivity sweep points.
- **Every parameter carries provenance**: source citation, retrieval date, verification status (`needs-verification` | `verified` | `locally-overridden`), and calibration label (`illustrative` | `sourced` | `calibrated`).
- **Local overrides are first-class.** The Prior Estimator invites the reader to check the source and overwrite the value, whereupon the badge switches to verified. Packs support the same move: an operator override is recorded as a distinct layer above the shipped pack, never a silent edit of it.

## Explicitly Out of Scope

- **No computation.** Packs are inert data. Every consumer of a pack lives in 007, 009, 010 or 011.
- **No organisation-specific content in shipped packs.** Seed domains, sending domains, incident history and costed losses are deployment configuration under the existing `portability-config` discipline. A shipped pack contains only published, citable, general parameters.
- **No automatic pack updates.** Fetching a new pack is an operator action with a visible diff of what moved and which estimates change as a result. A risk model that silently re-prices itself overnight is not auditable.

## Impact

- **New config surface**: `config/packs/`, with a shipped default pack and an override layer.
- **Every estimate records its pack version.** Two estimates computed under different packs are not comparable, and trend reporting must know that.
- **A required check** verifies that no probability-affecting constant is compiled into engine code.
