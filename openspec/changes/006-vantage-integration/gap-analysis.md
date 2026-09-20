# Phase 9 gap analysis: `email-authentication`

**Status: complete. Findings below gate the removal of `pkg/scanner/email.go`.**

The rule this document exists to enforce is stated in `openspec/project.md`: the
capability's requirements survive, its implementation is replaced, and any
requirement vantage cannot satisfy stays on the existing code path. A capability
is not permitted to shrink because a dependency changed.

Each requirement of `openspec/changes/001-initial-build/specs/email-authentication/spec.md`
is taken in turn, against what the code on both sides actually does rather than
against what either was intended to do.

## Summary

The interim implementation does not satisfy four of its own six requirements.
That inverts the framing of the phase. This is not "replace working code with
better code"; it is "a placeholder has been reporting its own failures as
answers, and the replacement is also the fix".

Vantage satisfies the DNS retrieval and judgement for every requirement. Two
gaps are real and are being closed upstream rather than worked around here:
the DKIM selector list is not configurable, and the DMARC tags a deterministic
priority must be a function of are not exposed as structured data.

## Requirement by requirement

### SPF record check — **vantage supersedes**

| | |
|---|---|
| Interim | `lookupSPF` returns the raw TXT string. No syntactic validation, no `+all` detection. `SPFValid` in the store is set to `true` whenever the string does not begin with `"error"`. |
| Vantage | `analyse.SPFRecursive` evaluates the record: `+all`, syntax, the RFC 7208 ten-lookup limit, redirect and include chains, and whether the domain sends mail at all. |

The scenario "overly permissive SPF" is **not implemented** in the interim path.
Nothing looks for `+all`. The requirement has been unmet since Change 001.

### DKIM selector check — **gap: selector list not configurable**

| | |
|---|---|
| Interim | One hard-coded selector, `"default"`, with a comment claiming callers can extend it. They cannot; it is not a parameter. |
| Vantage | Probes thirteen selectors used by major providers, parses each key against RFC 6376/8301 — undersized RSA, revoked, test mode, malformed — and returns `StateNotChecked` when none hit. |

Vantage already honours the requirement's sharpest clause. The spec forbids ever
recording "domain has no DKIM", because selectors are not enumerable from DNS;
`dkimCheck` returns `StateNotChecked` rather than `StateNotFound` for exactly
that reason, and `analyse.DKIM` raises its confidence only when the caller named
the selector. That is the requirement implemented more carefully than we asked.

**The gap is `commonDKIMSelectors` being a package-level `var`.** The requirement
says *configurable*, and it matters to a CISO: an organisation knows its own
selectors, and a tenant-specific one nobody can guess is precisely the case that
turns a real DKIM deployment into a `not_checked`. Closed upstream by
`Request.DKIMSelectors`.

### DMARC policy check and deterministic priority — **gap: tags not exposed**

| | |
|---|---|
| Interim | Substring-scans for `p=` and stores the value. `Priority` is set to `""`, with a comment saying priority is not part of scanner output. |
| Vantage | `analyse.ParseDMARC` parses `p`, `sp`, `pct`, `adkim`, `aspf`, `rua`, `ruf`, and `analyse.DMARCFull` walks the organisational domain and validates report destinations. |

Deterministic priority is **unimplemented** in the interim path — the field is
literally empty.

Vantage parses every tag the requirement names, but `DMARCPolicy` is internal to
`analyse` and reaches a consumer only as rendered records and as
`ComputedEvidence` strings on individual findings. Deriving priority from those
means parsing prose, which is the failure mode Phase 8 rejected: reworded text
yields no match, which reads as "no policy" rather than as an error — silence in
the reassuring direction. **Closed upstream by `observation.Email`.**

### Adjacent passive checks, informational priority — **vantage supersedes; tiering is ours**

Vantage covers BIMI, MTA-STS (with policy retrieval, which the interim
existence-check does not do), TLS-RPT and CAA. The interim path has no BIMI,
TLS-RPT or CAA at all — three checks the requirement names are simply absent.

The *tiering* rule — adjacent findings never elevated above an SPF/DKIM/DMARC
finding on the same domain — is Trawl's, since vantage's catalogue carries
advisories and deliberately holds no opinion about our severity ladder. It
belongs in the signal registry, which already carries weakness class and
direction.

### No new job container; runs in-process — **satisfied, and a live defect fixed**

Both paths run in-process. But the requirement's second clause is not met today:

> The resolver SHALL be the one the deployment's egress policy permits, so that
> an operator who has restricted the engine to a nominated resolver does not find
> this capability quietly reaching a different one.

`pkg/scanner/email.go` calls `net.LookupTXT` and constructs `miekg/dns` clients
against the system resolver. It touches neither `Scope` nor the egress policy.
Everything Phase 3 built is, for this capability, not in the path — an
out-of-scope domain passed to `ScanAndSave` is queried. Routing through the
adapter closes it.

### Scope-limited, scheduled re-check — **vantage supersedes; drift is ours**

Scope limiting arrives with the adapter. Policy drift (`p=reject` → `p=none`
recorded as a transition rather than an overwrite) is `posture-regression`
machinery, and the Phase 8 work on `network_attribution` is the pattern: track
the tags as posture attributes and the transition is raised with history.

## Verdict

| Requirement | Disposition |
|---|---|
| SPF | Vantage supersedes; fixes an unmet scenario |
| DKIM | Vantage supersedes **after** `Request.DKIMSelectors` |
| DMARC + deterministic priority | Vantage supersedes **after** `observation.Email`; priority derived in Trawl |
| Adjacent records | Vantage supersedes; tiering in the signal registry |
| In-process, egress-permitted resolver | Vantage supersedes; fixes a live scope-guard bypass |
| Scope-limited, scheduled re-check | Vantage supersedes; drift via posture attributes |

**Nothing is retained on the existing code path.** Every requirement is met or
exceeded once the two upstream gaps close, so `pkg/scanner/email.go` and
`pkg/service/email_scanner.go` may be removed — but only after the upstream
release is tagged and pinned, and after the store model below is widened.

## Consequent work not in the original task list

- **`store.EmailPosture` collapses three states into booleans.** `SPFValid`,
  `DKIMFound`, `MTAStsFound`, `DNSSECValid`, `DANEValid` cannot distinguish "the
  control is absent" from "we could not look". That is the binary collapse
  Phase 5 forbids everywhere else in the store, and it is the difference between
  a CISO reading a gap and reading an outage. The model is widened to the same
  four states the rest of the assessment path carries.
- **The API surface is pinned** by `cmd/trawl/parity_test.go`, and the Angular
  client reads `emailPostures`. The capability keeps its view; the view gains
  the states.
