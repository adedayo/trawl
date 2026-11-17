# Ambient Contact-Rate Evidence Register

Status: taxonomy established; source review and rate selection outstanding.

This register is the working record for the `contactBaseRates` section that
Change 008 will eventually carry in a signed model pack. A rate is not ready
for the pack until its event definition, denominator, observation period,
source terms and uncertainty are recorded here.

## Canonical unit

Ambient attention is expressed as opportunistic contact attempts per reachable
service-day. A contact attempt must name the observable used by its source,
such as a TCP connection attempt, protocol handshake or application request.
Packets, unique source addresses and requests are not interchangeable events.

The model consumes a rate in this unit:

```text
lambda_ambient = comparable contact attempts / reachable service-day
```

Sources whose denominator is an address, network telescope or honeypot slot
must retain that denominator in the evidence record. They must not be silently
converted into a service-day rate.

## Initial classes

These classes are deliberately broad. They are split only where Trawl can
classify the service consistently and the evidence supports a meaningfully
different contact process.

| Class | Included labels and conventional ports | Evidence status |
| --- | --- | --- |
| `web` | HTTP/HTTPS, web and proxy labels; 80, 443, 8080, 8443 | source review outstanding |
| `ssh` | SSH; 22 | source review outstanding |
| `rdp` | RDP and Microsoft terminal services; 3389 | source review outstanding |
| `remote-admin` | VPN, IPsec, management and administrative labels | source review outstanding |
| `mail` | SMTP, submission, IMAP and POP labels; 25, 465, 587, 993, 995 | source review outstanding |
| `dns` | DNS and domain labels; 53 | source review outstanding |
| `database` | PostgreSQL, MySQL, MariaDB, Redis, MongoDB; 3306, 5432, 6379, 27017 | source review outstanding |
| `other-tcp` | Known responding TCP services without a supported class | no inherited rate; explicit fallback required |

The classifier is implemented in `pkg/contact/service_class.go`. An unknown
service remains `other-tcp`; it does not inherit the rate of the port that
happens to carry it.

## Source record

Each candidate source must be recorded with these fields before it can support
a model-pack parameter:

| Field | Required decision |
| --- | --- |
| Publisher and title | Who produced the measurement and what it is called |
| Observation period | When the underlying traffic was observed |
| Sensor population | Honeypots, telescope, public address space, cloud ranges, or other population |
| Service exposure | How the source identifies the service and whether it was reachable |
| Event definition | Packet, connection, handshake, request or unique source |
| Denominator | The exact unit used to normalize the observation |
| Statistic | Point estimate, interval and any distributional summary |
| Transformation | Reproducible conversion, if the source is not already service-day based |
| Bias and exclusions | Placement, fingerprinting, IPv4/IPv6, geography and filtering effects |
| Terms | Licence and permission to redistribute derived parameters |
| Decision | Accepted, supporting only, or rejected, with rationale |

## Acceptance rules

1. Do not combine incompatible observables merely because their numbers share
   a textual unit such as “events per day”.
2. Prefer the source with the closest event definition and population to the
   model class; use multiple sources to widen uncertainty, not to manufacture
   precision.
3. Preserve the source vintage. A later pack revision is a change in the
   adversary parameter, not a change in the assessed estate.
4. A class without comparable evidence remains unavailable or uses an
   explicitly wide fallback. It receives no unexplained engine constant.
5. KEV membership of a specific CVE is not part of the base service rate. Its
   consumption belongs to the attempt stage; any service-class attention
   adjustment must be separately named and deduplicated.

## Initial source review

Reviewed 2026-09-22. No source reviewed below provides a directly usable
`contacts per reachable service-day` rate for the initial pack. That is an
evidence result, not a reason to fill the pack with guessed values.

| Source | What it establishes | Decision |
| --- | --- | --- |
| [Rapid7 Project Sonar](https://www.rapid7.com/research/project-sonar/) | Internet-wide active surveys across many services; the public description says it scans public IPv4 space to identify open ports and then communicates with endpoints. | Supporting exposure evidence only. It measures reachability and service prevalence, not unsolicited contact received by a reachable service. |
| [Shadowserver network reporting](https://www.shadowserver.org/what-we-do/network-reporting/) | Public descriptions identify daily Internet-wide scans, sinkholes, honeypots and service-specific reports, including SSH, RDP, HTTP, SMTP and database services. | Supporting taxonomy and candidate calibration source. The public page does not expose the event counts, sensor denominator or licence terms needed to derive a rate. No numeric parameter selected. |
| [FIRST EPSS](https://www.first.org/epss/data) | Daily CSV and historical scores with `cve`, `epss` and `percentile`; the score estimates the probability that a published CVE will be exploited in the wild in the next 30 days. | Accepted for vulnerability-level enrichment in Changes 019/009. Rejected only for `lambda_ambient`: it is CVE exploitation likelihood, not service contact rate, and belongs to the attempt stage. |
| [CAIDA telescope catalogue](https://www.caida.org/catalog/datasets/) | Network-telescope datasets can describe unsolicited traffic at darknet address space. | Not selected in this pass. A telescope's packet or flow counts are not service-day contacts without a documented service exposure denominator and event mapping. |

The review therefore supports keeping the broad taxonomy, but it does not yet
support a sourced numeric ambient rate for any class. A usable rate requires a
source that publishes comparable event counts, exposed-service denominator,
observation interval, uncertainty and reuse terms, or a separately documented
calibration experiment using Trawl-compatible sensors. The proposed experiment
and raw observation contract are in
[ambient-rate-calibration.md](ambient-rate-calibration.md).

## Remaining Phase 0 work

- Review published mass-scan, background-radiation and honeypot studies for
  each class.
- Confirm reuse terms for the selected sources and record them above.
- Compare the taxonomy against stored `ServiceObservation` labels and report
  the percentage mapped to a named class versus `other-tcp`.
- Select provisional rates and intervals only after the source records are
  complete.
- Add the selected parameters to the signed pack under Change 008; do not add
  numeric rates to the classifier or contact engine.
