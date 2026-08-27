# Proposal: Operator dashboard with an executive area

## Why

Trawl's engine now establishes considerably more than its interface reports.
Change 006 produced four-state coverage per check, per-asset and per-scenario
coverage figures, signal observations mapped through a versioned registry, and
an assessment-run record distinguishing a domain assessed cleanly from one
never assessed at all. Change 005 added correlation, so worker output becomes
typed assets, findings and posture observations. The dashboard shows an
overview tab computing one number: how many assets are pending.

That gap is the problem. A tool whose central argument is *"we did not look"
must never render as "it is fine"* currently renders both as an absence of rows
in a table.

There is also a stale spec. The `dashboard` capability from Change 001 predates
the SQLite engine and still specifies views driven by "a Convex live query".
Convex was removed from the stack in Change 005 Phase 5; the live channel is
Server-Sent Events at `/api/v1/events` for the server build and Wails IPC for
the desktop build. A requirement naming a datastore that no longer exists
cannot be verified against anything.

## What changes

One dashboard, two depths, over the same store:

- **Operator surface** — the existing tabs, brought up to what the engine now
  knows: coverage state on every posture claim, assessment-run status per
  asset, regressions as first-class rows rather than a derived count, and the
  fail-closed egress exclusions from Change 006 Phase 7 shown as *withheld
  assessment* rather than omitted silently.
- **Executive area** — a CISO-facing summary answering four questions: what is
  exposed, how much of it have we actually looked at, what changed for the
  worse since last time, and what should be done first. It is a view over the
  same rows, never a parallel computation.

## Scope

In scope: the Angular dashboard under `app/`, the read models backing it, and
the `dashboard` capability spec. Both transports — the desktop build over Wails
IPC and the server build over HTTP/SSE — because a view that exists in one and
not the other is how the two deployments start to disagree.

Out of scope: priced expected loss, calibrated probability, control ROI and the
board/practitioner/engineer split. Those are Changes 009–012, and the executive
area here is deliberately shaped so that Change 012 extends it rather than
replacing it. This change presents **observed exposure**; 012 presents
**priced risk**. Claiming the second before 009 and 010 exist would be the
confidence laundering RISK-ARC §4 warns against.

## The reference implementations, and where we depart

`checkmate-app` is the closest sibling and the right model for information
density, dark mode, tooltip-explained metrics and a headline card row. Two
deliberate departures:

**Implementation style.** `checkmate-app` uses inline templates (a 933-line
dashboard component), ngx-charts and Tailwind 3. Trawl's conventions are
Tailwind v4 with spartan/ui and separate template files. We adopt the
information design, not the file layout.

**The composite score.** `checkmate-app` leads with an "Overall Posture Score".
Trawl must not, in that form. A single number over an estate that is 40%
unassessed is not a worse measurement — it is a different kind of statement,
and rendering it identically to one over a fully assessed estate is exactly the
error guardrails 5 and 6 exist to prevent. Trawl's headline figure carries its
coverage inline and is refused outright below a floor. That is the honest
version of the same idea, not a weaker one.

## Risks

- **The executive area becomes a second computation.** Mitigated by requiring
  every figure to be a stored value read from the same rows the detail views
  read, verified by a cross-view consistency test.
- **Coverage becomes decorative.** A coverage badge nobody reads is no better
  than no badge. Mitigated by refusing the headline figure below a floor rather
  than annotating it, so low coverage changes what the interface will say
  rather than adding a caveat beside it.
- **Scope creep into 012.** Mitigated by an explicit prohibition: no currency
  and no probability in this change.
