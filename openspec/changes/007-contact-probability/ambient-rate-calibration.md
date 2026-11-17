# Ambient Contact-Rate Calibration

Status: design approved; typed aggregation and SQLite evidence persistence are
implemented. No deployment or model parameter is authorized by this document.

## Objective

Estimate the rate of opportunistic contact against a deliberately instrumented
reachable service, using the same unit required by Change 007:

```text
contact attempts per reachable service-day
```

The experiment calibrates the ambient prior. It does not observe contact with
Trawl-managed assets, infer targeted activity, or produce a contact posterior.

## Experimental units

Each instrumented endpoint is one unit identified by a random experiment ID.
The endpoint must expose exactly one declared service class at a time. The
initial cohort should use the classes with the clearest protocol semantics:

- `web`: HTTP and HTTPS endpoints
- `ssh`: SSH
- `rdp`: RDP
- `mail`: SMTP, with other mail protocols recorded separately until evidence
  shows they can share a class
- `database`: one explicitly named protocol per endpoint

`remote-admin`, `dns`, and `other-tcp` remain in the taxonomy but should not
receive a calibrated rate until the experiment has a reliable event definition
for them.

Endpoints must be isolated, disposable, non-production systems. They must not
contain credentials, customer data, routable internal access, or a path to
other systems. The experiment is passive from the operator's perspective: it
accepts and records traffic but does not scan, exploit, authenticate against,
or follow up on sources.

## Raw observation contract

The collector stores append-only observations. A raw record has this shape:

```json
{
  "experimentId": "random-id",
  "endpointId": "random-id",
  "serviceClass": "ssh",
  "transport": "tcp",
  "port": 22,
  "observedAt": "2026-09-22T12:00:00Z",
  "eventType": "connection-attempt",
  "sourceKey": "one-way-hash",
  "accepted": true,
  "protocolIdentified": true
}
```

The raw record must not retain source IP addresses, payloads, credentials,
session contents, or other unnecessary personal data. `sourceKey` is only for
deduplication within the declared retention window and must use a keyed,
rotatable digest whose key is not stored with the observations.

`eventType` is the primary measurement. The initial event vocabulary is:

- `connection-attempt`: a new transport connection observed by the endpoint
- `protocol-interaction`: a valid protocol-level request after connection
- `application-request`: an application-layer request with a class-specific
  definition

These event types are reported separately. They must not be summed into one
rate because one connection can contain many requests.

## Exposure denominator

The collector emits one exposure interval per endpoint:

```text
reachableServiceDays = sum of wall-clock time the endpoint was reachable
```

Intervals are bounded by independent health checks. A health-check failure
does not silently count as reachable; the affected interval is excluded or
marked uncertain. Planned maintenance, deployment changes, and endpoint
identity changes close an interval and start a new one.

For class `c` and event type `e`, the raw rate is:

```text
lambda[c,e] = eventCount[c,e] / reachableServiceDays[c]
```

If endpoint exposure differs materially by network, geography, provider, or
address family, those strata remain separate until there is evidence that they
can be pooled.

## Uncertainty

For a count of events over exposure time, the initial analysis may use a
Poisson count model. The stored result must include the point estimate and a
documented interval, with the interval method and confidence level recorded.
Zero-count classes must retain their upper bound; zero observations do not
mean zero ambient attention.

The experiment report must include:

- event count and reachable service-days;
- point estimate and low/high interval;
- number of endpoints and observation duration;
- event definition and deduplication rule;
- censoring, outages and excluded intervals;
- class, transport, address-family and geography strata;
- collection software version and clock assumptions.

## Bias and comparison

The report must discuss at least:

- honeypot fingerprinting and attraction bias;
- provider and geography selection;
- IPv4 versus IPv6 exposure;
- service banners and default-port effects;
- health-check availability bias;
- repeated attempts from the same source;
- the difference between scanning, exploitation and ordinary protocol use.

External sources may be used as a reasonableness comparison, but not merged
with these observations unless their event definition and denominator match.
Rapid7 Sonar remains exposure evidence; Shadowserver reports remain candidate
comparison or calibration evidence until raw counts and terms are available.

## Promotion to a model parameter

No result enters `contactBaseRates` until all of these are true:

1. The event definition matches the canonical contact definition.
2. The denominator is reachable service-days or has a reviewed conversion.
3. The observation period and source vintage are recorded.
4. The interval and its calculation method are reproducible.
5. The reuse and retention terms are reviewed.
6. The service class has enough coverage to justify a distinct rate.
7. The result is labelled `sourced` or `calibrated`, never silently treated as
   a universal truth.

The signed model pack records the resulting parameter and provenance. Updating
it is a change in the ambient adversary parameter and must not rewrite the
historical observation or silently alter conditional attempt, success, or
impact estimates.

## Non-goals

This design does not authorize scanning the public Internet, contacting
third-party targets, collecting attacker identities, attributing activity to
organisations, or using honeypot observations as proof that a Trawl asset was
contacted. Those are separate questions and remain outside Change 007.
