# Tasks: 012-executive-workbench

**Do not start until Change 011 is implemented.**

## Phase 0 — Preconditions
- [ ] Control rankings, scenario estimates and instrumentation ranking available from Change 011
- [ ] Confirm all estimate types carry label, interval, coverage and ledger reference

## Phase 1 — View architecture
- [ ] Three view modes reading identical stored estimates; no per-view recomputation
- [ ] Cross-view consistency test
- [ ] Estimate rendering component that cannot be constructed without an interval

## Phase 2 — Drill-down
- [ ] Navigation: aggregate → scenario → control → asset → finding → ledger row → observation
- [ ] Observation detail: timestamp, source, evidence, coverage state, tool version, pack version
- [ ] **Required check**: every board-level currency figure has a complete drill path

## Phase 3 — Board decisions
- [ ] `boardDecisions` with kind, value, rationale, decider, date, minutes reference, status
- [ ] Loss threshold recorded as inherited, referencing the existing decision
- [ ] Tail price with unset distinct from zero; standing notice while unset
- [ ] Cadence configuration

## Phase 4 — Snapshots and trends
- [ ] Immutable `quarterlySnapshots` with pack version, tool versions, seed, decisions in force
- [ ] Interval-narrowing trend read
- [ ] Ranking-stability trend read, reporting instability as a finding
- [ ] Movement attribution: decompose every snapshot-to-snapshot delta into world change, belief correction and parameter change; refuse to render a combined trend line
- [ ] Belief-correction labelling in the UI — an increase caused by new instrumentation reads as a correction to the previous report, not a deterioration
- [ ] Per-sensor realised value: belief corrections produced and their magnitude; flag sensors that have produced none
- [ ] Pack-change marking with re-baseline or declared discontinuity
- [ ] Immutability test against a later pack update

## Phase 5 — Export gating
- [ ] `exportGate` for board and external audiences
- [ ] Refusal reasons: unverified parameters (named individually), illustrative figures, coverage below floor, unset tail price
- [ ] Assumption-propagation statements attached to sourced figures
- [ ] `exportRecords` with snapshot, audience, gate result, timestamp

## Phase 6 — Challenge flow
- [ ] Dispute, withdraw and recompute from the interface
- [ ] `challengeRecords` retained; withdrawn rows never deleted
- [ ] Test: withdrawing a variance-bearing row widens the displayed interval

## Phase 7 — Presentation guardrails
- [ ] **Required check**: no composite score anywhere in any interface or export
- [ ] **Required check**: no figure rendered without label, interval and coverage
- [ ] **Required check**: no board or CISO headline figure is a mean-time-to-remediate or similar closed-finding time average; where remediation duration appears at engineer-level detail, detection coverage for the same class is shown alongside it
- [ ] **Required check**: every board-level figure shows freshness (elapsed time since its newest contributing observation, or its stalest material input if aggregated) at headline level, not only via drill-down
- [ ] Simplification disclosures bound to the figures relying on them
- [ ] AI narrative attribution; test that figures are identical with commentary disabled
- [ ] Playwright accessibility scanning across all three views

## Phase 8 — Close the arc
- [ ] Update `project.md`: Trawl as continuous exposure measurement feeding a quantified risk and investment decision engine
- [ ] Archive Change 002 as superseded by Change 009, if not already done
- [ ] Publish `openspec/RISK-ARC.md` reasoning document alongside the specs

## Exit Criteria

A board pack refuses to export while a single underlying parameter remains unverified or the tail price is unset; every currency figure on it walks down to a timestamped observation in a bounded number of steps; an engineer can dispute one ledger row and watch the board figure move and its interval widen; the quarterly trend reports whether intervals are narrowing and whether the ranking is stable; and nowhere in the product does a unitless composite risk score exist.
