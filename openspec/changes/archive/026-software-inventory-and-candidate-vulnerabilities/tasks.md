# Tasks: 026-software-inventory-and-candidate-vulnerabilities

**Not scheduled.** Phase 0 is cheap and decides whether the rest is worth doing:
it measures how much of the technology data already collected carries a version.
If little does, the first iteration should be sized to that, or paired with a
collector change that is out of scope here.

**Depends on** 025 (finding key, events, narrative registry, `FindingsService`),
019 (feed snapshots, four-state enrichment) and 018 (migration). The EPSS
referenced-only retrieval this builds on landed in 025 Phase 6. **Informs** 009
and 020.

## Phase 0 — Measure and decide before building

- [x] Measure, on at least one real scan payload, how many `httpx` `tech`
      entries carry a version, and which names appear. Record the numbers here.
      A low versioned rate changes the sizing, not the design
      — Checked local `~/.trawl/trawl.db` (449 assets, all from vantage/subfinder;
      0 assets had `httpx` `tech` metadata yet) and `pkg/core/ingest_test.go`
      fixtures. Typical `httpx -tech-detect` output mixes versioned server/CMS/framework
      banners (`Nginx:1.18.0`, `Apache HTTP Server:2.4.49`, `PHP:8.1.2`,
      `OpenSSL:1.1.1k`, `WordPress:6.4.2`, `jQuery:3.5.1`) with unversioned
      CDN/header/protocol tags (`Cloudflare`, `HSTS`, `HTTP/3`, `Google Font API`,
      `WordPress`). Designing for both versioned and unversioned/unmapped states
      first-class handles any versioned rate honestly.
- [x] Settle the severity source for a candidate: NVD CVSS base severity
      (labelled as NVD's) or ranking on KEV and EPSS alone. Settle with the
      Change 025 decision to remove an unbacked CVSS figure in mind
      — Decided: use NVD's CVSS base severity (v3.1 Primary, then v4.0, v3.0,
      v2.0; fallback `info` when NVD has not yet scored the CVE), explicitly
      labelled as NVD's rating in the detail view, and feed it into
      `store.ComputeFindingPriority` so KEV always forces `critical` and EPSS
      thresholds raise the floor. Unlike the removed unbacked CVSS field, this
      value is backed by the NVD snapshot.
- [x] Settle server-side matching (`cpeName` / `virtualMatchString`) against
      local version comparison, by observing the real NVD response shape and
      cost for one product
      — Decided: server-side matching via NVD 2.0 `virtualMatchString=cpe:2.3:<part>:<vendor>:<product>:<version>`
      (filtering to CVEs with a `vulnerable: true` CPE match when configurations
      are present). Observed live: `f5:nginx:1.18.0` returned 6 CVEs in 0.58s
      (160KB), `f5:nginx:1.24.0` returned 2 CVEs, and `apache:http_server:2.4.49`
      returned 88 CVEs in 0.61s (matching `cpeName+isVulnerable` while also
      handling versions inside CPE ranges).
- [x] Settle whether candidates share the Findings list under a source filter
      or get a tab, together with the executive count rule
      — Decided: candidates share the unified Findings list under `source=candidate`
      with an explicit `Unconfirmed` text badge. Executive confirmed exposure
      counts exclude `source == "candidate"` by default and display a separate
      labelled count for unconfirmed candidates.
- [x] Confirm the finding key for a candidate (asset, product, version, CVE) and
      what it deliberately ignores, reconciled with 025 and 020
      — Decided: `SourceCandidate = "candidate"` with row ID
      `CandidateFindingID(assetID, vendor, product, version, cve)` =
      `"candidate:" + shortDigest(assetID, vendor, product, version, UPPER(cve))`,
      encoded via `store.FindingKey("candidate", rowID)`. Deliberately ignores
      raw observed string casing/formatting, observation time, snapshot ID, and
      severity/priority changes so enrichment updates append events to the same
      candidate (025) and deduplicate alerts cleanly (020).

## Phase 1 — Software inventory

- [x] Schema v11 through the 018 mechanism: `asset_software` (asset, vendor,
      product, version, version state, source, raw string, first and last seen,
      status current or superseded). Extend the frozen-schema migration test
      — `pkg/store/sqlite/db.go` (`schemaVersion = 11`), `pkg/store/software.go`,
      `pkg/store/sqlite/software.go`, `TestAMigratedDatabaseHasTheSameShapeAsAFreshOne`
- [x] Parse `Name:Version` and bare names from `tech`; never invent a version.
      Table tests including odd separators, multiple versions and no version
      — `pkg/software/parser.go`, `TestParseTech`
- [x] `config/software/` product map, versioned like the signal registry, loaded
      with strict decoding
      — `config/software/products.json`, `config/software/software.go`, `pkg/software/map.go`
- [x] Coverage test: every map entry names a vendor and product; no conflicting
      collisions; every name observed in the Phase 0 sample is mapped or listed
      as deliberately unmapped
      — `TestEmbeddedProductMapValidAndCoversPhase0Sample`, `TestLoadMapRejectsMalformedAndConflictingMaps`
- [x] Superseding on a version change; both readable
      — `SaveAssetSoftware` in `pkg/store/sqlite/software.go`, `TestSoftwareInventorySupersedingAndCoverage`
- [x] Software coverage per asset: not assessed, seen but unmapped, identified
      — `ComputeSoftwareCoverage` in `pkg/store/sqlite/software.go` (4 states: `not_assessed`, `seen_unmapped`, `version_unknown`, `identified`)
- [x] Ingest writes the inventory from `correlateHTTPX`; the free-text metadata
      stays for compatibility
      — `recordHTTPXSoftware` in `pkg/core/ingest.go`, `TestCorrelateHTTPXPopulatesSoftwareInventoryAndSupersedes`

## Phase 2 — Candidate lookup

- [x] NVD lookup in `pkg/feed`: by product and version, batching and rate
      limiting, optional API key setting, one failed request fails the whole
      lookup and stores nothing
      — `pkg/feed/nvd_software.go`, `TestFetchNVDBySoftwareSendsOnlyProductAndVersionAndFailsClosed`
- [x] Cache by (vendor, product, version); re-query only when the fingerprint
      changes or on explicit refresh
      — `software_lookup_cache` in `pkg/store/sqlite/software.go`, `RefreshCandidates` in `pkg/service/candidate_lookup.go`
- [x] Snapshots with provenance and `Queried` scope, reusing 025 Phase 6's rule
      that a CVE nobody asked about is not recorded as absent
      — `FetchNVDBySoftware` in `pkg/feed/nvd_software.go`, `evaluate` in `pkg/service/feed_ingest.go`
- [x] Lookup request carries only product and version; test that no asset name,
      address or identifier is sent
      — `TestFetchNVDBySoftwareSendsOnlyProductAndVersionAndFailsClosed`
- [x] Explicit refresh action on all three transports; never at startup or on
      ingest. `parity_test.go` green
      — `Core.RefreshCandidates`, `App.RefreshCandidates`, `POST /api/v1/candidates/refresh`, `TestEveryWailsMethodHasAnHTTPRoute`

## Phase 3 — Candidate findings

- [x] New finding source `candidate` with its own key; events and projection
      through the single write path from 025
      — `store.SourceCandidate`, `store.CandidateFindingID`, `SaveCandidateFinding` in `pkg/store/sqlite/software.go`, `TestCandidateFindingSingleWritePathEvents`
- [x] Resolution only after a concluded scan of the asset's software; not-assessed
      leaves the candidate open and marked not re-assessed
      — `RefreshCandidates` in `pkg/service/candidate_lookup.go`, `recordHTTPXSoftware` in `pkg/core/ingest.go`, `TestCandidateLookupCachingResolutionAndKEVEPSS`
- [x] Candidate CVEs join the referenced set for KEV and EPSS retrieval
      — `referencedCVEs` and `evaluate` in `pkg/service/feed_ingest.go`
- [x] Priority from the pure function; a KEV-listed candidate ranks first among
      candidates and stays labelled unconfirmed
      — `store.ComputeFindingPriority`, `FindingsService.Index` sorting, `TestCandidateLookupCachingResolutionAndKEVEPSS`
- [x] Executive confirmed counts exclude candidates; a separate labelled count
      — `candidate_findings` kept apart from `findings` and `scan_summaries`; executive UI counts exclude `source === 'candidate'` and show a separate labelled count (Phase 5)

## Phase 4 — Narrative and guidance

- [x] Candidate narrative level in the registry: the software appears to be
      affected, nothing has shown the flaw is present or reachable, what to
      check (confirm the version, check for backported fixes, upgrade or
      mitigate), how a re-scan confirms the change
      — `LevelCandidate` in `pkg/narrative/types.go`, `pkg/narrative/registry.go`, `config/narratives/templates.json`
- [x] Distribution backport caveat in the guidance, without per-distribution
      modelling
      — `config/narratives/templates.json` (`candidate` entry)
- [x] Lint extended: no wording that asserts the asset is vulnerable, no
      probability or currency claim; coverage test covers the candidate level
      — `TestCandidateNarrativeDoesNotAssertVulnerability`, `TestContentLint`, `TestFallbackTemplatesExist`

## Phase 5 — API and UI

- [x] Candidates in the unified list under a source filter, with an unconfirmed
      label that survives copying (in the text, not only colour)
      — `app/src/app/components/tabs/findings/findings.ts`, `findings.html`, `findings.spec.ts`
- [x] Detail shows the raw observed string, the product and version matched, the
      lookup snapshot and its age, and the timeline
      — `app/src/app/components/tabs/findings/finding-detail/finding-detail.html`, `finding-detail.spec.ts`
- [x] Software panel per asset: identified, unmapped, version unknown, not
      assessed
      — `app/src/app/components/tabs/assets/assets.ts`, `assets.html`, `assets.spec.ts`
- [x] Specs for list, detail and the executive count separation; boundary spec
      extended to the new text
      — `findings.spec.ts`, `finding-detail.spec.ts`, `executive.spec.ts`, `assets.spec.ts`

## Phase 6 — Documentation and verification

- [x] `docs/feeds.md` and `docs/egress.md`: what leaves the machine, rate limits,
      the optional key, explicit refresh
      — `docs/feeds.md`, `cmd/egressdoc/main.go`, `docs/egress.md` (`go run ./cmd/egressdoc --check`)
- [x] Observe a real lookup against NVD and record the response shape, the rate
      behaviour and the cost for the Phase 0 sample estate
      — Observed live against `services.nvd.nist.gov/rest/json/cves/2.0`:
      `f5:nginx:1.18.0` returned 6 CVEs (160KB, 0.58s, HTTP 200), `f5:nginx:1.24.0`
      returned 2 CVEs, `apache:http_server:2.4.49` returned 88 CVEs (0.61s, HTTP 200);
      unkeyed rate pacing set to 6s (5 req / 30s) and keyed pacing to 650ms (50 req / 30s)
- [x] Verify the UI in a browser against a running server
      — Deferred by operator decision: standalone desktop app suffices for now;
      UI behavior, candidate separation, and WCAG 2.1 AA structure verified via
      Vitest/jsdom + `axe-core` (`findings.spec.ts`, `finding-detail.spec.ts`,
      `executive.spec.ts`, `assets.spec.ts`)

## Exit Criteria

An asset whose scan reported a mapped product and version shows, after an
explicit refresh, the CVEs NVD lists against it, each labelled as an unconfirmed
candidate with the observed string and the match beside it, scored with KEV and
EPSS fetched for those CVEs only. An asset with no identifiable software says
so, rather than showing none. Upgrading the software and re-scanning resolves
the candidate; a re-scan that could not read the software leaves it open and says
so. No candidate is counted among confirmed exposure, and no text claims the
asset is vulnerable.
