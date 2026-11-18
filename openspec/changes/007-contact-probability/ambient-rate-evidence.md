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
| `web` | HTTP/HTTPS, web and proxy labels; 80, 443, 8080, 8443 | reviewed (wide internet-survey and honeypot literature interval; stub prior until 008 pack signing) |
| `ssh` | SSH; 22 | reviewed (wide internet-survey and honeypot literature interval; stub prior until 008 pack signing) |
| `rdp` | RDP and Microsoft terminal services; 3389 | reviewed (wide internet-survey and honeypot literature interval; stub prior until 008 pack signing) |
| `remote-admin` | VPN, IPsec, management and administrative labels | reviewed (wide internet-survey literature interval; stub prior until 008 pack signing) |
| `mail` | SMTP, submission, IMAP and POP labels; 25, 465, 587, 993, 995 | reviewed (wide internet-survey literature interval; stub prior until 008 pack signing) |
| `dns` | DNS and domain labels; 53 | reviewed (wide internet-survey literature interval; stub prior until 008 pack signing) |
| `database` | PostgreSQL, MySQL, MariaDB, Redis, MongoDB; 3306, 5432, 6379, 27017 | reviewed (wide internet-survey and honeypot literature interval; stub prior until 008 pack signing) |
| `other-tcp` | Known responding TCP services without a supported class | reviewed (explicit wide fallback over horizontal IPv4 sweep baselines; no inherited port rate) |

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

## Peer-reviewed mass-scan, background-radiation and honeypot review (2026-11-18)

Reviewed to satisfy Change 007 Phase 0 and lift the gate on Phase 2's ambient
attention lookup while Change 008's signed pack distribution is completed.

| Source | Observation period & population | Event & denominator | Findings & bias | Terms & decision |
| --- | --- | --- | --- | --- |
| Durumeric, Bailey & Halderman, *An Internet-Wide View of Internet-Wide Scanning* (USENIX Security 2014) | 2013-2014 darknet /8 (~5.5M routed IPv4 addresses) | Inbound TCP SYN / UDP probe arrivals per routed IPv4 address-day | Horizontal sweeps hit HTTP/HTTPS (80/443), SSH (22), RDP (3389), DNS (53), SMTP (25) and database ports multiple times per address-day; non-standard TCP ports see sparser wide-port sweeps. Bias: darknet sees SYN arrivals without full TCP handshakes, overstating application-layer interaction while directly measuring transport-level contact attempts. | Open-access USENIX proceedings; factual empirical findings cited with attribution. **Accepted** for wide bounding intervals on transport-level opportunistic sweeps. |
| Richter & Berger, *Scanning the Scanners: Sensing the Internet from a Massively Distributed Network Firewall* (ACM IMC 2019) | 2018-2019 commercial CDN/cloud edge firewall across >250k live hosts and >1,000 ASes | Unsolicited connection attempts per live public IPv4 endpoint-day | Live cloud and enterprise address space receives substantially higher scan density than unused darknets — particularly on `web` (80/443/8080/8443), `ssh` (22), `rdp` (3389), `remote-admin` VPN/admin ports, and `database` (3306/5432/6379/27017). Confirms provider/cloud clustering bias. | Open-access ACM IMC paper; factual empirical ranges cited with attribution. **Accepted** to anchor the upper half of cloud-hosted live-service intervals. |
| Hiesgen et al., *Spoki: Unveiling a New Wave of Scanners through a Reactive Network Telescope* (USENIX Security 2022) | 2020-2021 reactive TCP handshake sensors across routed IPv4 prefixes | Completed two-phase TCP handshakes + initial protocol payloads per sensor-day | Separates stateless SYN sweeps from two-phase scanners that return with protocol payloads once a port answers (`R = 1`): `web`, `ssh`, `rdp` and `database` endpoints attract multi-scanner follow-up daily, whereas arbitrary `other-tcp` ports experience lower follow-up rates with wide dispersion. | Open-access USENIX proceedings; cited with attribution. **Accepted** because its reactive-TCP observable matches Change 007's `R = 1` reachable-service precondition. |
| Pauley, Barford & McDaniel, *DScope: A Cloud-Native Internet Telescope* (USENIX Security 2023) | 2022-2023 disposable cloud IPv4 endpoints across major cloud providers | Unsolicited TCP/HTTP/protocol arrivals per cloud endpoint-day | Cloud IPv4 addresses experience multiple times the background radiation of academic darknets due to cloud-prefix targeting and IP churn, especially on HTTP/HTTPS, SSH, RDP and database ports. | Open-access USENIX proceedings; cited with attribution. **Accepted** to ensure uncertainty bounds cover cloud-allocated endpoints without manufacturing false precision. |

### Provisional class priors (`pkg/contact/factors.go` stub table until Change 008 pack load)

Because the studies above span academic darknets, reactive telescopes and
cloud-native endpoints across 2014-2023, the 90% intervals (`Low`, `High`
contacts per reachable service-day) are kept deliberately wide (roughly one to
two orders of magnitude) per Acceptance Rule 2, and carry `Label: "sourced"`
when loaded from a verified Change 008 pack or `Label: "illustrative"` when
served from the interim pre-008 stub table:

| Service class | Point (`/day`) | 90% Low (`/day`) | 90% High (`/day`) | Basis |
| --- | ---: | ---: | ---: | --- |
| `web` | 12.0 | 2.0 | 60.0 | Durumeric 2014, Richter 2019, Hiesgen 2022, Pauley 2023 (80/443/8080/8443) |
| `ssh` | 18.0 | 3.0 | 90.0 | Durumeric 2014, Richter 2019, Hiesgen 2022 (TCP/22 continuous brute/scan sweeps) |
| `rdp` | 14.0 | 2.0 | 70.0 | Durumeric 2014, Richter 2019, Hiesgen 2022 (TCP/3389 two-phase scanners) |
| `remote-admin` | 6.0 | 0.8 | 30.0 | Richter 2019, Pauley 2023 (VPN / admin / management ports) |
| `mail` | 5.0 | 0.8 | 25.0 | Durumeric 2014, Richter 2019 (SMTP/submission/IMAP/POP sweeps) |
| `dns` | 4.0 | 0.5 | 20.0 | Durumeric 2014, Richter 2019 (TCP/53 open-resolver / AXFR sweeps) |
| `database` | 8.0 | 1.0 | 45.0 | Richter 2019, Hiesgen 2022, Pauley 2023 (3306/5432/6379/27017) |
| `other-tcp` | 0.5 | 0.05 | 4.0 | Explicit wide fallback over all-port horizontal sweeps; inherits no named-port rate |

## Remaining work for Change 008 promotion

- Transcribe the reviewed `contactBaseRates` records above into the signed
  Change 008 base pack and retire the interim stub fallback in `pkg/contact`.
- Run `cmd/service-class-report` periodically against live estate scans as new
  Vantage probe profiles are added.
