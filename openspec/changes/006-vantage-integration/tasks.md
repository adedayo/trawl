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
- [ ] Deployment policy schema stated over egress classes and consented third-party endpoints — no check names in configuration
- [ ] Derive the requested check set by filtering the catalogue through the policy
- [ ] Derive transport permissions from the declared profiles of the requested checks
- [ ] Fail-closed exclusion path: unconsented checks recorded `not_checked` with the excluding reason named
- [ ] **Required CI check**: egress conformance — every rule declares a profile, every declared class is recognised by the policy schema
- [ ] Test: an added intrusive check is excluded under default policy
- [ ] Test: an added third-party dependency is excluded and the endpoint named to the operator
- [ ] Generate operator-facing egress documentation from the declared profiles

## Phase 8 — Discovery and inventory enrichment
- [ ] CT hostnames into `asset-discovery` as source `ct-log`, through existing dedup and allowlist
- [ ] Provider, region, jurisdiction and provenance onto `asset-inventory`
- [ ] Regression suppression for attribution changes caused by provider-data refresh

## Phase 9 — Supersede email-authentication internals
- [ ] Route existing `email-authentication` requirements through the adapter
- [ ] Gap analysis: any requirement vantage does not satisfy stays on the existing code path
- [ ] Remove superseded DNS-lookup code only after the gap analysis is closed

## Exit Criteria

Trawl assesses an authorized domain with no external binary present, from a single compiled artefact; every applicable check carries one of four distinct states; every finding carries a registry mapping or an explicit unmapped marker; an assessment coverage figure accompanies every aggregate; an out-of-scope target provably emits zero network queries even when assessment logic requests them; concurrent assessments under different scopes are isolated; deployment policy is expressed over egress classes rather than check names, so a vantage upgrade introducing an intrusive check or a new third-party dependency is excluded automatically and reported rather than run; and an upgrade that adds a rule, changes a consumed signature, or declares an unrecognised egress class fails CI rather than surfacing at runtime.

