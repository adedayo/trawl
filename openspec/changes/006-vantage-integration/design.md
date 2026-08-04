# Design: 006-vantage-integration

## Embedding, not invoking

Vantage's README states that `pkg/` is an implementation detail outside its version guarantees, and that the CLI is the supported surface. That statement is aimed at unknown third-party consumers who cannot absorb breakage. It does not describe this relationship: both projects are under common ownership, and vantage's embedding API is free to be shaped by what Trawl needs from it.

So Trawl imports vantage and runs assessment in-process. What this buys:

- **The single-binary property survives.** Changes 003, 004 and 005 all rest on Trawl compiling to one static binary — desktop app, container and server from the same artefact. A subprocess dependency would have required bundling an executable into the Wails app bundle, the distroless image and every distribution package, plus a discovery-and-diagnostic path for when it is missing. All of that disappears.
- **Type safety end to end.** No JSON round-trip, no envelope-version negotiation, no exit-code translation. Upstream API drift becomes a compile error rather than a field silently arriving as `null` in production.
- **Injectable transport, and therefore stronger scope enforcement.** See below. This is the substantive win.
- **Streaming progress.** Per-check completion flows into Trawl's event bus as it happens, rather than arriving as one lump when a process exits.
- **Context propagation.** Cancellation and deadlines pass through natively; a cancelled scan stops mid-flight instead of orphaning a process.

The cost is a real coupling, managed under *Contract discipline* below.

## The upstream API Trawl needs

Part of this change is defining that API in vantage. The shape Trawl requires:

```go
// package vantage (upstream)

type Assessor interface {
    Assess(ctx context.Context, req Request) (*Result, error)
    Catalogue(ctx context.Context) ([]RuleDescriptor, error)
}

type Request struct {
    Domain             string
    Hosts              []string      // supplied from Trawl's inventory
    Checks             []CheckID     // explicit; no profile names
    Enumerate          bool
    NoNetwork          bool
    ExpectJurisdiction []string
    Vantage            VantagePoint  // external only, today
}

type Options struct {
    Resolver   Resolver      // interface, not an address list
    HTTPClient *http.Client  // MTA-STS policy retrieval, CT, provider ranges
    Cache      Cache         // interface; Trawl supplies SQLite-backed
    Clock      Clock         // injectable, for deterministic tests
    Logger     *slog.Logger
}
```

Four requirements Trawl places on it, each of which makes vantage better as a general library rather than bending it to one consumer:

1. **No rendering concerns.** `Result` carries structured data only. Text, JSON, CSV and SARIF formatting stay in vantage's `cmd/` layer. A library returning pre-formatted strings cannot be embedded cleanly.
2. **No process-global state.** The existing `SetResolvers` / `SetQueryTimeout` package-level setters are unusable when several assessments run concurrently against different scopes. Configuration moves into `Options`, passed per call. A genuine upstream improvement, not a Trawl accommodation.
3. **Interfaces at every I/O boundary.** Resolver, HTTP client, cache and clock all injected. This is what makes both scope enforcement and hermetic testing possible.
4. **Typed sentinel errors.** Vantage already prefixes errors with `error:` and documents sentinels. Trawl needs them as typed values it can match on, to distinguish "the zone genuinely refused transfer" (a result) from "we could not reach any nameserver" (a coverage failure).
5. **Declared egress per rule.** Each check states, as structured data, what it touches. See *Egress profiles* below — this is what lets policy and mechanism share one source of truth.

Where these conflict with the CLI's needs, the CLI adapts — it is a consumer of the same API, and a library serving two consumers well is better designed than one serving a command-line front end only.

## Egress profiles

Vantage already documents that checks declare their egress, and `audit --list-checks` shows blast radius before anything is invoked. Making that declaration structured rather than prose is what allows Trawl's two safety mechanisms — the per-deployment policy gate and the scope-guarded transport — to derive from one definition instead of being maintained as two parallel lists.

```go
type RuleDescriptor struct {
    ID     CheckID
    Egress EgressProfile
    // ...
}

type EgressProfile struct {
    Resolver          bool     // ordinary recursive DNS
    TargetNameservers bool     // direct to authoritative: wild, ns, tko, axfr
    TargetHTTPS       bool     // MTA-STS policy retrieval
    ThirdParty        []string // certspotter, crt.sh, provider range endpoints
    Intrusive         bool     // beyond an ordinary query — axfr, and anything like it
}
```

Three consequences, in increasing order of importance.

**Documentation becomes generated.** `--list-checks` and Trawl's egress documentation are both derived from the same field, so they cannot drift from what the code does.

**Policy and mechanism share a source of truth.** Without this, Trawl has a configuration flag naming intrusive checks *and* a transport enforcing scope, maintained independently and free to disagree. With it, the deployment's policy is a predicate over egress profiles, the transport's permission set is derived from the profiles of the checks actually requested, and the check-name list disappears from configuration entirely.

**New checks fail closed.** This is the one that matters. A vantage upgrade introducing a check with `ThirdParty: ["some-new-service"]` is refused by the service-endpoint allowlist until an operator consents — automatically, without anyone having noticed the check was added. Likewise a new `Intrusive` check under a deployment that has not enabled intrusive assessment. For a tool whose first non-negotiable is non-destructiveness, silent widening of behaviour on dependency upgrade is the failure mode to design against, and prose documentation cannot prevent it.

This is the same principle as the registry completeness check: an upgrade may add capability, but it may never quietly add *behaviour* the operator did not agree to. The two checks cover the two ways that could happen — an unmapped signal, and an unconsented egress class.

The profile also earns its place in vantage independently of Trawl: a CI consumer gating on SARIF output can programmatically assert that a pipeline run touched nothing but DNS, which is not currently expressible.

## Scope enforcement in the transport

This is the strongest reason to embed, and it upgrades an existing guardrail rather than merely preserving it.

Trawl's `scope-authorization` guardrail is defence-in-depth: the worker independently validates targets before touching them, assuming upstream data may be wrong. With a subprocess, that check happens when the argument vector is built — after which Trawl has no further control over what the process does.

Embedded, Trawl supplies the `Resolver` and the `*http.Client`:

```go
type scopeGuardedResolver struct {
    inner vantage.Resolver
    scope *authz.Scope
}

func (r *scopeGuardedResolver) Query(ctx context.Context, name string, t dns.Type) (*dns.Msg, error) {
    if !r.scope.Permits(name) {
        return nil, authz.ErrOutOfScope   // recorded, and the query never leaves the process
    }
    return r.inner.Query(ctx, name, t)
}
```

An out-of-scope target is now unreachable at the point of egress. Even a hypothetical upstream bug — a check following a CNAME chain out of scope, enumeration returning a neighbouring organisation's name — cannot produce a packet. The guardrail moves from "we do not ask" to "we cannot ask", which is what the project's non-negotiables were reaching for.

The same guard wraps the HTTP client, covering MTA-STS policy retrieval and takeover assessment. Third-party endpoints (Cert Spotter, crt.sh, provider range files) pass through an explicit, separately configured allowlist of *service* endpoints, distinct from target scope — a distinction invisible from outside a subprocess and explicit here.

## Contract discipline

Embedding trades a versioned CLI contract for a Go API contract. The discipline that replaces it:

- **Pin by module version** in `go.mod`. No `replace` directives to local paths in committed code; a required check enforces this, because a local replace is how "both projects are mine" quietly becomes an unbuildable repository for anyone else.
- **Contract tests in Trawl** exercising every part of the vantage API surface Trawl uses, run in CI. An upstream change breaking a Trawl assumption fails Trawl's build at upgrade time, deliberately.
- **The registry completeness check** (below) already fails when upstream adds a rule. Together, a vantage upgrade cannot land silently.
- **Upstream changes are made upstream.** No fork, no vendored patches. If Trawl needs something, vantage gains it as a general capability, or Trawl handles it in its own adapter layer.

Trawl's adapter (`pkg/scanner/vantage`) stays thin but real: it owns scope guarding, registry mapping, coverage translation and store persistence. No Trawl code outside that package imports vantage types, so an upstream API change has exactly one blast radius.

## Result handling

Without exit codes, the CLI's five-state contract is expressed directly in the result:

| Situation | Embedded handling |
|---|---|
| Assessment completed | `Result` with per-check outcomes; findings persisted |
| Some checks inconclusive | Same, with those checks carrying `check_failed` |
| Whole assessment failed | Typed error; **no** findings persisted; all requested checks recorded `check_failed` |
| Out-of-scope target | `authz.ErrOutOfScope` from the guarded transport; scope-violation record; no findings |
| Cancelled or deadline exceeded | `ctx.Err()`; partial results retained, unreached checks `not_checked` |

The critical invariant is unchanged: **a failed assessment never produces the reassurance of a passing one.**

## Concurrency

Removing process-global configuration makes concurrent assessment across different scopes safe. Trawl runs a bounded worker pool over domains, each with its own scope-guarded transport, sharing one SQLite-backed cache for provider ranges and CT results with the 7-day and 24-hour freshness semantics vantage documents. Stale-on-unreachable behaviour is preserved, including disclosure of the entry's age — an old answer that discloses its age is more useful than no answer, but only if the age travels with it.

## The signal registry

Versioned data (`config/signals/vantage-<major>.json`), not code:

```json
{
  "registryVersion": "2026-08-01",
  "vantageMajor": 1,
  "signals": [
    {
      "id": "SURF-DMARC-001",
      "condition": "No DMARC record published",
      "weaknessClass": "email-spoofing",
      "scenario": "business-email-compromise",
      "stage": "success",
      "dedupGroup": "dmarc-enforcement",
      "control": "dmarc-enforcement",
      "direction": "adverse"
    },
    {
      "id": "SURF-SPF-005",
      "condition": "SPF terminates in ~all rather than -all",
      "weaknessClass": "email-spoofing",
      "scenario": "business-email-compromise",
      "stage": "success",
      "dedupGroup": "sender-authorisation",
      "control": "spf-hardening",
      "direction": "adverse"
    }
  ]
}
```

This change defines and populates the registry with `stage`, `dedupGroup` and `control`. It deliberately does **not** carry `deltaLogit`, `varianceShare` or `tau` — those are risk-model parameters and belong to Change 008's packs, which reference signals by id. Keeping observation separate from valuation is what allows the ledger to show "this fact" and "what we did with this fact" as distinct, separately challengeable rows.

Completeness is checked by calling `Catalogue` in-process — no subprocess, no output parsing — asserting every returned rule is either mapped or explicitly recorded as intentionally unmapped. Identifiers encountered at runtime but absent from the registry are stored `unmapped` rather than dropped: a signal Trawl cannot yet interpret is still evidence a human should see.

## Coverage states are load-bearing

Vantage distinguishes four states, and the README explains why:

> "No record published" and "we could not tell" are different statements, and conflating them would let a reader conclude a control is missing when in truth it was never assessed.

Trawl inherits this verbatim, and the inverse error matters more for a risk model: a check that never ran must never contribute the *reassurance* of a passing check.

| State | Meaning | Risk-model consequence (defined in 008/009) |
|---|---|---|
| `ok` | Assessed, control present and sound | Favourable measured-state evidence |
| `not_found` | Assessed, no record published | Adverse measured-state evidence — a strong positive observation |
| `not_checked` | Not attempted | No evidence; class prior stands unmodified |
| `check_failed` | Attempted, could not conclude | No evidence; raised as an assessment gap |

Every scenario and asset therefore carries an **assessment coverage** figure — checks concluded ÷ checks applicable — reported alongside every number derived from it.

## Intrusive and third-party checks

Four checks contact the target's nameservers directly rather than going through a resolver: `wild`, `ns`, `tko`, `axfr`. Vantage documents `axfr` as more intrusive and confines it to its `surface` and `deep` profiles.

Trawl's configuration does not name these checks. It states a **policy over egress profiles**: whether intrusive assessment is permitted (default: no), and which third-party service endpoints are consented to (default: none). The set of checks Trawl requests is then derived by filtering the catalogue through that policy, and the transport's permission set is derived from the profiles of the checks actually requested.

The practical difference from a check-name list is what happens on upgrade. A new intrusive check, or a check reaching a third party nobody has consented to, is excluded automatically and recorded as `not_checked` with the reason — rather than running because no one thought to add its name to a deny list.

## Discovery and inventory enrichment

CT-enumerated hostnames enter `asset-discovery` with source `ct-log`, then flow through the existing dedup and allowlist path — identical treatment to subfinder/amass output, no privileged trust. This finds names that were certificated but never linked or resolved by other means.

Provider attribution enriches assets with provider, region, inferred jurisdiction, and vantage's `provenance` record (endpoint used, data date). The provenance matters for regression detection: an attribution can change because Trawl's provider data refreshed rather than because the estate moved, and `posture-regression` must not raise the former as a change. Azure is reported unattributed by design upstream; Trawl surfaces `unattributed` rather than guessing.

## Relationship to `email-authentication`

The capability's requirements are unchanged and still hold. Its implementation is replaced. Any requirement vantage cannot satisfy is retained on the existing code path rather than dropped — a capability is not permitted to shrink because a dependency changed.

## Testing

Embedding makes the test suite better, because everything is injectable:

- **Hermetic assessment tests** with a fake `Resolver` returning fixture DNS responses. No network, no fixture-JSON replay, no subprocess — actual assessment logic under test.
- **Scope-guard test (required check)**: an out-of-scope target produces zero queries, asserted by a resolver that fails the test if called at all. Strictly stronger than the subprocess equivalent, which could only assert that a process was not started.
- **Contract tests** over the vantage API surface Trawl consumes, so upstream drift fails Trawl's build at upgrade time.
- **Coverage-state propagation**: a `check_failed` result never produces a passing control state anywhere downstream.
- **Registry completeness** against `Catalogue`, as a required check.
- **Egress conformance (required check)**: every rule in the catalogue declares an egress profile, and every profile's declared classes are represented in the deployment policy schema — so an upgrade introducing an unrecognised egress class fails the build rather than being silently permitted or silently dropped.
- **Fail-closed test**: a catalogue entry declaring an unconsented third-party endpoint or an intrusive profile is excluded from the requested set and recorded `not_checked` with its reason.
- **Concurrency test**: simultaneous assessments under different scopes leak neither configuration nor transport between each other — the regression test for the removed package-level setters.
- **No-replace-directive check**: committed `go.mod` contains no local `replace` for vantage.
- **Failure-mode tests**: assessment error persists no findings; cancellation retains partial results with unreached checks as `not_checked`.
