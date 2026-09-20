# Tasks: 020-alerting-and-delivery

**Deliberately unscheduled.** The capability earns its keep approaching
production use, when someone other than the developer depends on the tool. The
specification exists now because the constraints it imposes — dedup identity,
idempotent delivery, coverage travelling into the alert — are cheap to honour
while surfaces are being built and expensive to retrofit once several of them
have grown their own notification paths.

**Depends on 019** for anything alerting on exploitation status. Alerting that
a CVE entered KEV requires something that notices it did.

## Phase 0 — Decide the identity before building anything

- [ ] Define the dedup key for each alertable event class, and write down what
      each key deliberately ignores. This is the whole design: a key too broad
      hides a real change, a key too narrow re-alerts daily and the tool is
      muted within a fortnight
- [ ] Decide whether resolution is an alertable event in its own right. A tool
      that only ever reports bad news gets read as noise; one that reports
      recovery has to be right about recovery
- [ ] Confirm that the regression detector already emits every event class the
      rules will need. If a rule needs an event that is not published, that is
      work in the detector, not here

## Phase 1 — Destinations and the delivery log

- [ ] Destination records, read from external configuration. No URL,
      credential or org name in the repository
- [ ] Destination URL validation refusing loopback, link-local and private
      ranges unless explicitly permitted, **re-checked at delivery time** so a
      DNS change cannot convert an accepted destination into an internal one.
      This is an SSRF surface pointed at the operator's own network, and the
      scope guard does not cover it — that guard protects targets
- [ ] A delivery log: event identity, destination, attempt count, outcome,
      final state, last success per destination
- [ ] Migration through the 018 mechanism

## Phase 2 — Delivery

- [ ] A bus subscriber in `pkg/event` terms. If delivery cannot be written as
      a subscriber, that is a finding about the bus, not a reason for a
      parallel path
- [ ] Webhook delivery with a stable event identifier across retries
- [ ] Bounded retry with backoff, terminating in a recorded failure rather
      than an unbounded queue
- [ ] Delivery failure never blocks or fails a scan. Detection that depends on
      notification succeeding is worse than notification that is missing

## Phase 3 — Routing and composition

- [ ] Rule evaluation as a pure function of event and rules, with a test that
      the same event routes identically across repeated evaluation
- [ ] A test asserting no AI provider is consulted in the delivery path. The
      component deciding whether a human is told is not one to make
      probabilistic, and a grep will not stop the next change
- [ ] Deterministic templates carrying the finding's own evidence and the check
      that produced it, so a recipient can act without opening the dashboard
- [ ] Any alert asserting something about a set carries that set's coverage.
      "No critical findings" is false when half the estate was `not_checked`,
      and it is the most reassuring sentence the system can emit
- [ ] Operational alerts (fetch failures, scan errors) classified separately
      from estate alerts and routable separately

## Phase 4 — Visibility

- [ ] A destinations view distinguishing failing from configured-and-idle. A
      destination that has received nothing looks exactly like a broken one
- [ ] Last successful delivery shown per destination
- [ ] Documentation: payload shape, the event identifier's stability guarantee,
      and what a receiver must do to deduplicate

## Exit Criteria

A new critical finding reaches a configured webhook without anyone opening the
dashboard. Rescanning an unchanged estate delivers nothing. A destination that
has been failing for a day is visibly failing rather than quietly silent. No
alert asserts that an estate is clear without stating how much of it was
assessed.
