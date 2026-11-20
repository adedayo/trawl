# Vulnerability feed endpoints and air-gapped operation

Trawl enriches CVE-bearing findings from three public, unauthenticated
authorities. These requests are deliberate egress to named reference
authorities rather than contact with an assessed target, so they travel through
`pkg/feed` rather than the target scope guard. The feed client refuses any host
not explicitly configured in its endpoint map.

## Default outbound endpoints

| Feed | Identifier | Default URL | Format | Query pattern |
|---|---|---|---|---|
| CISA Known Exploited Vulnerabilities | `cisa-kev` | `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json` | JSON | Full catalogue, conditional `ETag` / `Last-Modified` |
| FIRST EPSS (per-CVE API) | `epss-api` | `https://api.first.org/data/v1/epss?cve=<CVE>,...` | JSON | Referenced CVEs only, in batches of 50. Used for refreshes |
| FIRST EPSS (daily file) | `epss` | `https://epss.empiricalsecurity.com/epss_scores-current.csv.gz` | Gzipped or plain CSV | Whole file. Used for offline loading, mirrors, and when `epss-api` is not configured |
| NIST NVD CVE 2.0 (by CVE) | `nvd` | `https://services.nvd.nist.gov/rest/json/cves/2.0?cveId=<CVE>` | JSON | Referenced CVEs only - never a full mirror |
| NIST NVD CVE 2.0 (by software) | `nvd` | `https://services.nvd.nist.gov/rest/json/cves/2.0?virtualMatchString=cpe:2.3:<part>:<vendor>:<product>:<version>` | JSON | Explicit candidate refresh only; cached per product and version |

## Licence and rate-limit constraints

- **CISA KEV**: US federal public-domain work (17 U.S.C. 105). Published as a
  single JSON document updated when entries change; conditional HTTP requests
  avoid re-downloading unchanged payloads.
- **FIRST EPSS**: Published publicly by FIRST for operational and research
  enrichment with source attribution. Updated once daily; polling more often
  than every 6-24 hours buys nothing. A refresh asks the per-CVE API for the
  CVEs on stored findings, not for the daily file of every scored CVE, so the
  request is proportional to the estate's findings rather than to the whole
  catalogue.
- **NIST NVD 2.0 API**: US public-domain data subject to NIST's unauthenticated
  rolling window (5 requests per 30 seconds without an API key). Trawl queries
  only distinct CVEs already present on stored findings rather than mirroring
  the full CVE corpus.

## Proxy and offline / air-gapped operation

Operators behind an egress proxy can permit only the three hosts above
(`www.cisa.gov`, `api.first.org`, `services.nvd.nist.gov`, plus
`epss.empiricalsecurity.com` if the daily file is wanted) or
override the URLs in `jobs/feed-ingest` (`KEV_URL`, `EPSS_URL`, `NVD_URL`) to
point at an internal mirror.

Air-gapped installations with no outbound access should supply snapshots from
local files:

```sh
go run ./cmd/feed-ingest --feed cisa-kev --file /path/to/known_exploited_vulnerabilities.json --db /path/to/trawl.db
go run ./cmd/feed-ingest --feed epss --file /path/to/epss_scores-current.csv.gz --db /path/to/trawl.db
```

Or post snapshots into a running `trawl server` via `jobs/feed-ingest` with
`KEV_FILE` and `EPSS_FILE` set. (`jobs/feed-ingest` and `cmd/feed-ingest`
ingest the whole EPSS file; only the server's own refresh uses the per-CVE API.) When no snapshot has been supplied, Trawl
leaves every finding in `not_checked` and displays *no CISA KEV catalogue
retrieved* rather than reporting zero exploited vulnerabilities.

## What is stored, and when it was last refreshed

Every ingested catalogue is kept in the database, not just its effect on
findings.

- **Every snapshot**: one row recording the feed, source, when its content
  arrived, a digest of the content and the entry count. These are never
  deleted; findings cite them as provenance.
- **CISA KEV entries**: the entries of the latest KEV snapshot are stored with
  it, in the same transaction. A new snapshot retires the previous one's
  entries. This is what lets a finding recorded after the last refresh be
  evaluated against the catalogue without the network. EPSS (a few hundred
  thousand rows a day) and NVD (referenced CVEs only) are not kept entry by
  entry; their snapshot rows alone describe them.
- **Last refreshed**: the last time each source was successfully consulted,
  recorded whether or not the content had changed. It is separate from when the
  content arrived, because an unchanged catalogue produces no new snapshot and
  its arrival time stops moving. Both are shown. Staleness (48 hours by
  default) is judged on the last successful consultation, so a feed that was
  reconfirmed this morning is current however old its content is, and a feed
  whose refreshes keep failing goes stale and says why. A failure never moves
  the last-refreshed time.

Refreshing is safe to repeat. Unchanged content is not stored twice, and a
refresh still evaluates any CVE finding recorded since the catalogue was last
applied, so pressing it after a new scan picks those findings up. Reconfirming
an unchanged catalogue with nothing new to evaluate is not entered in the run
history.

Databases that predate this behaviour have no last-refreshed record and fall
back to the snapshot's own time until the next refresh. Their KEV snapshot has
no stored entries until the next refresh finds the same content, which stores
them.

## EPSS is looked up for the CVEs you have

A refresh sends FIRST the CVE identifiers on stored findings (and nothing
else), so those identifiers leave the machine; the daily file does not do that.
Operators who object can configure only the `epss` endpoint (or load the file
offline) and get the whole-file behaviour.

- A CVE the API returns no score for has no EPSS score yet, which is common for
  recently published CVEs. That is recorded as *not found*, an answer.
- A CVE recorded after the request was made was never asked about. The snapshot
  remembers which CVEs it asked about, so such a finding stays *not checked*
  until the next refresh rather than being recorded as unscored.
- The snapshot is identified by what was learned (the CVEs, their scores and
  the score date), so the same answers are the same snapshot and a new finding
  or a changed score is a new one. With no CVE findings there is nothing to
  look up, no request is made, and the snapshot is empty.
- A failed batch fails the whole refresh and stores nothing, as for NVD, so a
  partly retrieved set is never mistaken for a complete one.
- Scores are fetched when you refresh; a finding added afterwards shows EPSS as
  *not checked* until you do.

## Refreshing one feed

The desktop application's **Refresh CISA KEV** action, and `POST
/api/v1/feeds/refresh` with `{"feeds": ["cisa-kev"]}`, refresh only the named
feeds. An unknown name is a `400`. With no `feeds` the three are refreshed
together. Refreshes are serialised, so the scheduler and an operator pressing
the button together cannot fetch and apply the same catalogue twice.

## Software candidate lookups (NVD)

When an operator explicitly runs **Check software candidates** (or calls
`POST /api/v1/candidates/refresh`), Trawl queries NVD 2.0 for each distinct
mapped product and known version observed in the software inventory:

- **What leaves the machine**: only the CPE `virtualMatchString`
  (`cpe:2.3:<part>:<vendor>:<product>:<version>`), `resultsPerPage`, and
  `startIndex`. No asset name, hostname, IP address, or internal identifier is
  ever sent.
- **Unmapped and unversioned software**: products absent from the curated map
  (`config/software/products.json`) or observed without a version are recorded
  as such and are never sent to NVD.
- **Caching and explicit refresh**: lookup answers are cached by
  `(cpePart, vendor, product, version)` with their NVD snapshot provenance. A
  subsequent refresh reuses the cache unless the observed software changes or
  the operator passes `force: true`. Lookups never run silently at startup or
  during scan ingest.
- **Rate limits and optional API key**: NVD permits 5 requests per 30 seconds
  without a key (Trawl paces unkeyed requests at 6 seconds) and 50 requests per
  30 seconds with a key (paced at 650ms). Operators may store an optional NVD
  API key in the `nvd_api_key` setting; it is sent in the `apiKey` HTTP header
  only to the configured NVD endpoint.
- **Failure handling**: if any single product or page request fails, the whole
  lookup fails and stores no partial answer; previously stored candidates are
  retained and marked as not re-assessed.

## Desktop mode

The desktop application never fetches third-party feeds silently at startup: an
operator opening a local database has not asked Trawl to make background
connections. Instead, the executive view discloses snapshot age (or the absence
of any retrieved catalogue) and provides explicit **Refresh** actions, per
catalogue or for all of them, to pull on demand.
