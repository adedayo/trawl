# Trawl — Self-Authorization & Scope Authorization Contract

> **Notice to Operators**: This document is a formal record of authorization required prior to executing any discovery or scanning activity with Trawl. Modify this template with your organisation's details and authorized assets before initiating live scans.

---

## 1. Authorization Statement

This document certifies that **Dayo Adetoye** (the "Operator") is fully authorized to perform continuous external attack surface monitoring, passive OSINT asset discovery, non-destructive network/service scanning, email authentication checks, and public repository secret scanning against the target assets enumerated in Section 2.

The authorization extends solely to non-destructive testing and passive analysis performed in accordance with the Rules of Engagement set forth in Section 3.

---

## 2. Authorized Scope

The scope of authorization is strictly limited to the following assets and their direct subdomains (where applicable):

### 2.1 Authorized Domains
- `dayo-adetoye.com` (and all subdomains discovered via passive OSINT)
- `example.com` *(Template placeholder — replace with authorized domain)*

### 2.2 Authorized CIDR Ranges
- `192.0.2.0/24` *(Template placeholder — replace with authorized IP range)*

### 2.3 Authorized Public Repositories
- `https://github.com/adedayo/trawl`
- `https://github.com/adedayo/trawl-private`

---

## 3. Rules of Engagement & Technical Boundaries

Scanning operations executed by Trawl MUST adhere to the following mandatory constraints:

### 3.1 Explicitly Allowed Techniques (Non-Destructive Only)
- Passive OSINT gathering (Certificate Transparency logs, DNS enumeration, WHOIS/ASN queries).
- Port discovery via TCP SYN/ACK banner probing (`naabu`).
- HTTP/HTTPS service fingerprinting, header inspection, and TLS certificate/cipher parsing (`httpx`).
- Vulnerability template matching restricted to version detection and non-destructive HTTP requests (`nuclei`, prioritizing CISA KEV templates).
- DNS-over-HTTPS queries for SPF, DKIM, DMARC, BIMI, MTA-STS, TLS-RPT, and CAA record analysis.
- Read-only secret pattern scanning against publicly accessible git history (`gitleaks`).

### 3.2 Explicitly Prohibited Techniques
- **No Active Exploitation**: No exploit payloads, remote code execution attempts, or state-modifying requests shall be transmitted.
- **No Credential Brute-Forcing**: No automated login attempts, password spraying, or dictionary attacks against authentication endpoints.
- **No Denial of Service (DoS)**: High-rate rate-limiting or stress-testing that risks service degradation is prohibited.
- **No Authenticated Scraping**: Only unauthenticated, public-facing endpoints and public repositories are in scope.
- **No Private Repository Access**: Scanning is strictly confined to publicly reachable repositories; no Personal Access Tokens (PATs) or private keys are accepted.

---

## 4. Defense-in-Depth Scope Enforcement

In addition to policy authorization, scope is programmatically enforced by the system runtime:

1. **Independent Scan Guardrail**: The `scan-worker` component independently validates every target against the configured `SEED_DOMAINS` and `SEED_CIDRS` allowlist before transmitting packets.
2. **Public Repo Verification**: The `repo-scan-worker` rejects any repository URL containing authentication tokens or credentials.
3. **Dry-Run Validation**: All scanning jobs support `--dry-run` to print resolved targets without sending network traffic.

---

## 5. SOC & Security Operations Coordination

- **Scanner Origin IPs**: Scans will originate from dedicated container infrastructure configured by the Operator.
- **EDR/SIEM Exclusions**: The Operator's SOC/Security Operations should whitelist scanner origin IPs to prevent triggering internal alert responses during automated checks.

---

## 6. Authorization Sign-Off

- **Authorized By**: Dayo Adetoye
- **Role / Title**: Security Lead / System Owner
- **Date**: 2026-07-23
- **Status**: ACTIVE & AUTHORIZED
