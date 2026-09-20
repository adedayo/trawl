# Change: 020-alerting-and-delivery

## Status

**Proposed, deliberately not scheduled.** Promoted from *Unclaimed work* in
`openspec/STATUS.md`, where it had no owner and therefore no record that it was
missing on purpose.

Specified now, built later. The capability matters most approaching production
use, when someone other than the developer is relying on the tool. Writing the
specification early is not premature: the constraints below — idempotency,
deduplication identity, the four-state coverage rule — are cheaper to honour
while the surrounding surfaces are still being built than to retrofit once
several of them emit notifications in their own way.

## Why

Trawl's argument is that continuous monitoring beats periodic assessment
because it notices change. Everything needed to notice change exists: the
regression detector, the event bus, the four-state coverage model, the
deterministic priority function.

Nothing tells anyone. There is no webhook delivery, no email, no deduplication,
and nothing fires when a new asset appears or a critical finding lands. Every
discovery is discovered a second time, by a human who happened to open the
dashboard. The detection is real and the notification is a person remembering
to look, which means the effective detection latency is however long it takes
someone to get curious.

The gap is stark because of what is already there. The regression detector
computes exactly the events worth sending, and publishes them to a bus with two
transports already attached. What is missing is not the hard part.

## What Changes

- **A delivery capability**: outbound webhooks first, since one webhook
  endpoint reaches everything an organisation already runs, and a small set of
  direct channels after.
- **Event identity and deduplication.** Every alertable event has a stable
  identity derived from what it asserts, so a rescan that observes the same
  condition does not re-alert. A monitoring tool that cries wolf daily is
  turned off within a fortnight, and the dedup key is the whole design.
- **At-least-once delivery with idempotency.** Retries are unavoidable;
  duplicate deliveries must therefore be recognisable as duplicates by the
  receiver. Delivery attempts, outcomes and the final state are recorded.
- **Routing by rule, evaluated deterministically.** Which events reach which
  destination is a pure function of the event and the configured rules. No AI
  in the delivery path — the component that decides whether a human is told is
  not one to make probabilistic.
- **Coverage travels into the alert.** An alert derived from an assessment with
  gaps says so. "No critical findings" delivered to an inbox is an assertion
  about the estate, and it is false if half of it was `not_checked`.
- **Silence is reported.** A configured destination that has received nothing
  is indistinguishable from a broken one, so the system states when it last
  successfully delivered and surfaces a destination failing to accept.
- **Destination configuration is external.** Webhook URLs are credentials and
  are org-specific; they live in configuration and never in the engine or the
  repository.

## Explicitly Out of Scope

- **No inbound integrations.** Creating tickets, closing them, or reading state
  back from a ticketing system is a bidirectional sync and a different change.
- **No AI-composed alert text** in the first iteration. Deterministic templates
  first. An annotation layer may enrich them later, subordinately, under the
  existing guardrail.
- **No on-call scheduling, escalation policies or acknowledgement workflow.**
  That is a paging product. Trawl delivers to one, or to a chat channel.
- **No alerting on absence of data as though it were a finding** — a fetch
  failure is an operational alert about Trawl, not a security alert about the
  estate, and conflating the two trains recipients to ignore both.

## Impact

- **Engine and schema.** Tables for destinations, routing rules, and a delivery
  log. Goes through the Change 018 mechanism.
- **Consumes the existing bus** rather than adding a parallel path. If a
  delivery subscriber cannot be written against `pkg/event`, that is a finding
  about the bus.
- **Introduces outbound requests to operator-supplied URLs**, which is a
  server-side request forgery surface pointed at whatever the operator
  configures. It needs its own treatment, distinct from the scope guard, which
  protects targets rather than the operator's own network.
- **Depends on 019** for anything that alerts on exploitation status: alerting
  that a CVE entered KEV requires something to notice that it did.
