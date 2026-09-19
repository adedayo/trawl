# Tasks: 006-vantage-integration

**Do not start until Change 003 (Go/SQLite engine) is implemented. Phase 1 is upstream work in the `vantage` repository.**

## Phase 0 — Preconditions
- [x] Confirm `pkg/store` and `pkg/event` from Change 003 are in place
- [x] Agree the embedding API shape with the vantage repository (this change's Phase 1)

## Phase 1 — Upstream: vantage embedding API
- [x] `Assessor` interface: `Assess(ctx, Request) (*Result, error)`, `Catalogue(ctx)`
- [x] Move all configuration out of package-level setters into per-call `Options`
- [x] `Resolver` interface injected per client; `Cache` per assessment *(`Clock` and injectable `*http.Client` still outstanding — needed for Phase 3)*
- [x] `EgressProfile` on the check description: resolver, target nameservers, target HTTPS, named third-party endpoints, intrusive flag
- [x] Populate egress profiles for every existing check
- [x] Regenerate `--list-checks` blast-radius output from the declared profiles
- [x] `Result` carries structured data only — no rendering, no formatting
- [x] Typed sentinel errors distinguishing genuine negative results from assessment failures
- [x] Per-check completion callback or channel, for progress streaming
- [x] Refactor vantage's own `cmd/` layer onto the same API, proving it serves two consumers
- [x] Document the embedding API upstream *(package doc, `docs/embedding.md`, compiled examples, README)*
- [x] Tag a vantage release *(v1.2.0)*

## Phase 2 — Adapter
- [x] `pkg/scanner/vantage`: the only package in Trawl importing vantage types
- [x] Pin by module version in `go.mod`; **required check**: no local `replace` directive *(pinned to `v1.2.0`)*
- [x] Library version recorded on every observation
- [x] Result translation into Trawl's store types
- [x] Outcome handling: completed, partial, failed, refused, cancelled
- [x] Progress events onto the Change 003 event bus
- [x] **Contract tests** over the consumed API surface

## Phase 3 — Scope-guarded transport
- [x] `scopeGuardedResolver` wrapping the injected resolver
- [x] Scope-guarded HTTP client for MTA-STS retrieval and takeover assessment
- [x] Service-endpoint allowlist for third-party services, separate from target scope
- [x] **Required CI check**: out-of-scope assessment emits zero queries against an instrumented transport
- [x] Test: assessment following a reference out of scope is refused at the transport
- [x] Test: unlisted third-party endpoint refused; dependent check recorded `check_failed`

## Phase 4 — Signal registry
- [x] `config/signals/vantage-<major>.json` schema (id, condition, weaknessClass, scenario, stage, dedupGroup, control, direction)
- [x] Populate for all current `SURF-*` identifiers (81 mapped)
- [x] Registry loader with version recording on every observation
- [x] Unmapped-identifier retention path
- [x] **Required CI check**: registry completeness against `Catalogue`

## Phase 5 — Persistence and coverage
- [x] Store tables: `signalObservations`, `signalRegistry`, `assessmentCoverage`
- [x] Four-state persistence with no binary collapse anywhere in the store or API
- [x] Coverage computation per asset
- [x] Coverage computation per scenario (needs the registry loader, Phase 4)
- [x] Propagation test: `check_failed` never renders as a passing control downstream
- [x] Derived control posture: compliance is assessed silence, since the catalogue carries advisories only

## Phase 6 — Concurrency
- [x] Bounded worker pool, per-assessment transport and configuration
- [x] SQLite-backed shared cache honouring provider-range and CT freshness windows
- [x] Stale-on-unreachable with age disclosure preserved
- [x] Test: concurrent assessments under different scopes cannot query each other's targets

## Phase 7 — Egress policy
- [x] Deployment policy schema stated over egress classes and consented third-party endpoints — no check names in configuration *(`EgressPolicy` in `pkg/scanner/vantage/policy.go`; `egressPolicy` block in `config/example.json`, guarded by a test that fails if it grows a check-name key)*
- [x] Derive the requested check set by filtering the catalogue through the policy *(`SelectChecks`; the adapter overrides caller-supplied `Checks`/`Profile` when a policy is configured)*
- [x] Derive transport permissions from the declared profiles of the requested checks *(`Selection.EndpointAllowlist` / `Selection.NewScope` — derived from selected checks, not from the consent list)*
- [x] Fail-closed exclusion path: unconsented checks recorded `not_checked` with the excluding reason named *(`Selection.ExclusionCoverage`; a policy excluding everything returns `OutcomeRefused` rather than an empty clean result)*
- [x] **Required CI check**: egress conformance — every rule declares a profile, every declared class is recognised by the policy schema *(`TestEgressConformance`)*
- [x] Test: an added intrusive check is excluded under default policy *(`TestAddedIntrusiveCheckIsExcludedUnderDefaultPolicy`)*
- [x] Test: an added third-party dependency is excluded and the endpoint named to the operator *(`TestAddedThirdPartyDependencyIsExcludedAndEndpointNamed`)*
- [x] Generate operator-facing egress documentation from the declared profiles *(`docs/egress.md` via `go run ./cmd/egressdoc`; `TestEgressDocumentationIsUpToDate` fails the build if it drifts)*

## Phase 8 — Discovery and inventory enrichment

**Upstream work is done and released; this phase is unblocked.** The `net` and
`ct` checks computed everything this phase needs and then flattened it to
`[]string` before it left the library, so the only route available here was
parsing record lines. That was rejected: it fails silently and in the
reassuring direction, because reworded prose yields no match, which reads as
"no attribution" rather than as an error — an unattributed asset then being
indistinguishable from one known to be in no provider range. That is guardrail
5 inverted.

Vantage spec `015-structured-observations` is implemented: `pkg/observation`
carries the facts, `finding.CheckResult.Observation` carries them to the
result, and `observation.SameBasis` answers "did the host move, or did our data
refresh?" — the judgement being the library's to make, since it knows which
fields are material.

Trawl now pins `vantage v1.4.0` (`finding.SchemaVersion` 1.1, additive). The
required check forbidding a local replacement directive stayed in force
throughout: the pin moved only once the tag was public, so the build has never
depended on code that existed on one machine.

- [x] Bump the vantage pin to the release carrying spec `015` — `v1.4.0`, taken from the module proxy rather than a local path, so the build depends on code that exists for everyone
- [x] Extend the adapter's contract tests to cover the new consumed surface (`Observation`, `observation.Network`, `observation.CT`) so an incompatible upstream change fails the build — `pkg/scanner/vantage/contract_test.go`; verified by re-pinning to `v1.3.0`, which fails at compile time rather than during a scan
- [x] Map `FailedSources` and `StaleSources` onto assessment coverage, so an unattributed asset reports `check_failed` rather than reading as clean — `pkg/scanner/vantage/coverage.go`; stale sources annotate without degrading, since cached ranges remain usable and degrading them would suppress real findings on every publisher outage. Taken before inventory enrichment deliberately: enriching first would leave a window in which attribution is displayed confidently for assets whose provider ranges never loaded
- [x] Provider, region, jurisdiction and provenance onto `asset-inventory` — `asset_attribution` and `attribution_provenance` tables, replaced per assessment rather than merged; one row per address so a name spread across jurisdictions is not reduced to a winner; a `Hosting` column that shows "could not be established" rather than an empty provider list when the ranges failed to load
- [ ] CT hostnames into `asset-discovery` as source `ct-log`, through existing dedup and allowlist — preserve the three-state resolution; `observation.CTHost.Undetermined()` exists so a failed lookup is not read as absence
- [x] Regression suppression for attribution changes caused by provider-data refresh, via `observation.SameBasis` — hosting tracked as posture attribute `network_attribution`; a change on an unchanged basis is raised, a change across a changed basis records a new baseline and raises nothing. The suppressed run still writes its snapshot, or the baseline would stay at the pre-refresh value and the next run would raise the same change — suppression that defers rather than suppresses

## Phase 9 — Supersede email-authentication internals
- [ ] Route existing `email-authentication` requirements through the adapter
- [ ] Gap analysis: any requirement vantage does not satisfy stays on the existing code path
- [ ] Remove superseded DNS-lookup code only after the gap analysis is closed

## Exit Criteria

Trawl assesses an authorized domain with no external binary present, from a single compiled artefact; every applicable check carries one of four distinct states; every finding carries a registry mapping or an explicit unmapped marker; an assessment coverage figure accompanies every aggregate; an out-of-scope target provably emits zero network queries even when assessment logic requests them; concurrent assessments under different scopes are isolated; deployment policy is expressed over egress classes rather than check names, so a vantage upgrade introducing an intrusive check or a new third-party dependency is excluded automatically and reported rather than run; and an upgrade that adds a rule, changes a consumed signature, or declares an unrecognised egress class fails CI rather than surfacing at runtime.

