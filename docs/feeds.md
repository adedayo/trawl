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
| FIRST EPSS | `epss` | `https://epss.empiricalsecurity.com/epss_scores-current.csv.gz` | Gzipped or plain CSV | Daily snapshot, conditional `ETag` / `Last-Modified` |
| NIST NVD CVE 2.0 | `nvd` | `https://services.nvd.nist.gov/rest/json/cves/2.0?cveId=<CVE>` | JSON | Referenced CVEs only - never a full mirror |

## Licence and rate-limit constraints

- **CISA KEV**: US federal public-domain work (17 U.S.C. 105). Published as a
  single JSON document updated when entries change; conditional HTTP requests
  avoid re-downloading unchanged payloads.
- **FIRST EPSS**: Published publicly by FIRST for operational and research
  enrichment with source attribution. Updated once daily; polling more often
  than every 6-24 hours buys nothing.
- **NIST NVD 2.0 API**: US public-domain data subject to NIST's unauthenticated
  rolling window (5 requests per 30 seconds without an API key). Trawl queries
  only distinct CVEs already present on stored findings rather than mirroring
  the full CVE corpus.

## Proxy and offline / air-gapped operation

Operators behind an egress proxy can permit only the three hosts above
(`www.cisa.gov`, `epss.empiricalsecurity.com`, `services.nvd.nist.gov`) or
override the URLs in `jobs/feed-ingest` (`KEV_URL`, `EPSS_URL`, `NVD_URL`) to
point at an internal mirror.

Air-gapped installations with no outbound access should supply snapshots from
local files:

```sh
go run ./cmd/feed-ingest --feed cisa-kev --file /path/to/known_exploited_vulnerabilities.json --db /path/to/trawl.db
go run ./cmd/feed-ingest --feed epss --file /path/to/epss_scores-current.csv.gz --db /path/to/trawl.db
```

Or post snapshots into a running `trawl server` via `jobs/feed-ingest` with
`KEV_FILE` and `EPSS_FILE` set. When no snapshot has been supplied, Trawl
leaves every finding in `not_checked` and displays *no CISA KEV catalogue
retrieved* rather than reporting zero exploited vulnerabilities.

## Desktop mode

The desktop application never fetches third-party feeds silently at startup: an
operator opening a local database has not asked Trawl to make background
connections. Instead, the executive view discloses snapshot age (or the absence
of any retrieved catalogue) and provides an explicit **Refresh feeds** action
to pull on demand.
