# Change: 026-software-inventory-and-candidate-vulnerabilities

## Status

**Proposed, not scheduled.** Raised from a question about EPSS: scores are only
relevant for CVEs that bear on something the operator runs, so can the CVEs be
found from the nature of the asset rather than waiting for a scanner template to
name one? The EPSS half (fetch scores only for the CVEs on stored findings) landed
in Change 025 Phase 6. This change is the other half, and the larger one.

## Why

**1. A CVE enters Trawl only if a scanner template happens to match.**
`pkg/core/ingest.go` takes a CVE from a nuclei result's `classification.cve-id`.
Template coverage is a small subset of published CVEs, and a template match is
evidence that the flaw is reachable, not that the software is affected. An asset
running a version with known, even actively exploited, vulnerabilities for which
no template exists produces nothing. The product then reports quiet, and quiet is
read as safe. That is the failure the project's four-state coverage rule exists to
prevent, applied to software.

**2. The software evidence is already collected and then discarded.**
The scan worker runs `httpx -tech-detect` and ingest keeps the result as
`tech: [...]` inside the asset's free-text `metadata` JSON. It is unstructured,
unversioned in any reliable sense, not mapped to anything an advisory database
understands, and invisible to every query and view. Nothing reads it.

**3. The feeds are ready for it.**
NVD can be asked which CVEs apply to a product and version, and Change 019
already stores NVD, KEV and EPSS snapshots with provenance and four-state
coverage. EPSS is now fetched per CVE (Change 025 Phase 6), so the cost of
scoring a CVE set is proportional to the set, not to the catalogue.

## What Changes

- **Software inventory.** A structured, per-asset record of software the engine
  has seen: product, version (known or unknown, never guessed), the source and
  the raw string it came from, first and last seen. Parsed from the `tech` data
  already collected. No new traffic to any target.
- **Product identity.** A curated, versioned mapping from observed names to
  CPE vendor and product (`config/software/`), with a coverage test. Deterministic
  and reviewable. An observed name with no mapping is recorded as *unmapped*, which
  is visible, and is not looked up.
- **Candidate lookup.** For each mapped product and version, ask NVD which CVEs
  list it as affected. Results are stored as snapshots with provenance, cached by
  product and version, and re-queried only when the fingerprint changes or on
  explicit refresh.
- **Candidate vulnerabilities, kept apart from findings.** A CVE matched this way
  is a *candidate*: the software appears to be affected, nothing has shown the
  flaw is present or reachable. It is a distinct finding source with its own key,
  history (Change 025 events apply), status and narrative, and is always labelled
  *unconfirmed* with the evidence (the observed string and the match) beside it.
- **Enrichment follows.** Candidate CVEs join the set whose KEV status and EPSS
  scores are fetched, through the referenced-only paths that exist. A candidate
  that is KEV-listed is the case most worth an operator's attention and is
  ranked as such, still labelled unconfirmed.
- **Coverage is explicit.** An asset with no identified software is *not
  assessed for software vulnerabilities*, never "no known vulnerabilities". An
  asset whose software was identified but could not be mapped says so.

## Explicitly Out of Scope

- **No new probing.** No version-probing, banner grabbing, authenticated scan or
  agent. Evidence is what the existing scan worker already reports. A later change
  may add collectors; this one makes sure what is collected is used honestly.
- **No claim of vulnerability.** Candidate text never says an asset *is*
  vulnerable. Change 012's boundary applies: no probability of compromise and no
  monetary value, extended to this text by the same CI lint.
- **No feed into contact or exploit probability.** Candidate CVEs are not an
  observation of attackers and write no contact posterior (Change 007's guardrail
  applies unchanged). Whether they inform susceptibility inputs in Change 009 is
  for that change to decide, labelled as candidates.
- **No distribution backport reasoning.** Distribution-packaged builds routinely
  carry fixes without changing the upstream version string, so a version match on
  such a build over-reports. This change labels that risk; it does not model
  per-distribution advisories.
- **No SBOM or dependency analysis**, and no software the engine cannot see from
  outside.
- **No alert delivery.** Change 020 decides what is delivered and keys dedup on
  the finding key defined in Change 025.

## Impact

- **Schema.** Version 10 to 11 through the Change 018 mechanism: `asset_software`
  and candidate storage. Additive, so the frozen-schema migration test is
  extended and no existing table is retyped.
- **Egress.** Product and version strings leave the machine to reach NVD. They are
  facts about assets and are disclosed in `docs/feeds.md` and `docs/egress.md`.
  Lookups run on an explicit refresh (and the optional scheduler), never silently
  at startup, consistent with the desktop rule in `docs/feeds.md`.
- **Rate limits.** NVD allows 5 requests per 30 seconds without a key and 50 with
  one. Caching by product and version, and re-querying only on change, is what
  makes an unkeyed install workable. An optional API key setting is added.
- **Findings list.** Gains a source, `candidate`. Executive headline counts of
  confirmed exposure exclude candidates by default; candidates are counted and
  labelled separately, so the confirmed figure is not inflated by inference.
- **Transport.** New read endpoints and one refresh action on all three
  transports; Change 016's parity test applies.
- **Depends on** 025 (finding key, events, narrative registry, FindingsService),
  019 (feed snapshots and coverage state) and 018 (migration). **Informs** 009
  (a candidate-aware susceptibility input, if it wants one) and 020 (alerting on a
  KEV-listed candidate).
- **Org-neutral by rule.** Mapping and narrative text name no organisation,
  vendor contract or incident.

## Decisions Recorded Here

1. **Candidates are not findings.** The two answer different questions: "a scanner
   demonstrated this" and "the software list says this might apply". Merging them
   would make the second inherit the credibility of the first. They share the
   list, the key scheme and the timeline, and differ in label, ranking and
   headline counts.
2. **No new target traffic.** The change uses only data the scan worker already
   reports, so it needs no new scope authorisation and adds no contact with
   anything the operator has not already scanned.
3. **A curated product map, not a dictionary search.** Free-text search against
   the CPE dictionary is non-deterministic and invites wrong matches. A reviewed
   map is auditable, and an unmapped name is a visible gap with a test that keeps
   it so.
4. **Unknown version is a state, not a default.** A product with no version is
   recorded as such and is not matched against every CVE for the product. It is
   reported as *software seen, version unknown* and is not looked up.
5. **Lookups are cached and explicit.** The same (product, version) is asked
   once until it changes or the operator refreshes, which keeps an unkeyed NVD
   install inside its rate limit and keeps desktop mode quiet by default.

## Open Questions

- **Severity of a candidate.** Findings carry a severity and a priority computed
  by `ComputeFindingPriority`. A candidate has no scanner severity. Either record
  the CVSS base severity from the NVD response (a new column, which Change 025
  removed once for having no backing) or rank candidates on KEV and EPSS alone.
  Leaning the first, labelled as NVD's rating.
- **Match location.** Ask NVD per product and version (`cpeName` or
  `virtualMatchString`) and cache, or fetch the product's match data and compare
  versions locally. Server-side is simpler and keeps one source of truth;
  local is faster and works offline once cached. Leaning server-side first.
- **Where candidates appear.** Same list under a source filter, or a tab of their
  own. Leaning the same list, filtered out of the confirmed counts by default.
- **How much of `tech` carries a version.** The httpx technology data often has a
  name without a version. The rate of versioned entries on a real estate decides
  how useful the first iteration is, and should be measured on one before the
  mapping effort is sized.
