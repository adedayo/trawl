# Accepted risk

Vulnerabilities that are known, unfixable today, and carried deliberately.

The point of this file is that an unfixable vulnerability you have written
down is an accepted risk, whereas one you have not been told about is priced
at zero. `./security-fix.sh` reports these on every run; this file records the
decision, so that "still open" does not quietly become "forgotten".

Review on every release. If a fix has shipped, take it and delete the entry.

---

## npm — build tooling only, not shipped

`npm audit` reports 10 advisories in `app` (7 high, 3 moderate). None are in
the production dependency graph:

```sh
npm --prefix app audit --omit=dev     # found 0 vulnerabilities
```

That is the number that describes the shipped artefact. The Angular
application bundles only what it imports, and none of the packages below are
imported by anything under `app/src`.

### `image-size` ≤ 2.0.2 — no patch exists

| | |
|---|---|
| Advisories | GHSA-w3rx-r6r6-pgpr (CVE-2025-71330), GHSA-5p2g-fcmc-qvqq (CVE-2025-71329) |
| Severity | High, CVSS 8.7 |
| EPSS | 0.43% (35th percentile) |
| Patched version | **None** — every published version is affected |
| Path | `@spartan-ng/cli → @nx/angular → @nx/webpack → less → image-size` |

Infinite loops in the ICNS, JXL and HEIF parsers: a crafted image with a
zero-valued size field hangs the Node event loop.

Carried because there is nowhere to upgrade to. The five `@nx/*` advisories
and the `less` advisory are the same two CVEs surfacing at each hop of the
chain above — six of the seven highs are one unfixed dependency, counted
repeatedly.

Reachability is what makes this acceptable: the vulnerable parsers are invoked
by `less` resolving image dimensions in stylesheets at build time. The inputs
are this repository's own stylesheets and assets, on a developer machine or in
CI. An attacker who could supply a malicious image to that step could already
run arbitrary code in the build. No user of a released Trawl binary is exposed,
because none of this is in the artefact.

### `@angular/cli` 22.x — fix requires a downgrade

| | |
|---|---|
| Severity | Moderate (with `@hono/node-server`, `@modelcontextprotocol/sdk`) |
| Installed | 22.1.3 |
| Vulnerable | `20.3.14 - 20.3.33 \|\| 21.0.5 - 22.1.3` |
| npm's proposed fix | `@angular/cli@21.0.4` — a **major downgrade** |

No forward fix exists: 22.1.3 is the latest published release and is still in
range. npm reports any version change as a fix without regard to direction,
and dropping a major version of the build toolchain to clear a moderate
advisory in a transitive dev tool is the worse trade. Revisit when 22.1.4 or
later ships.

## Go — `golang.org/x/crypto` GO-2026-5932

Reported by `govulncheck` as reachable, with no published fix. Retained until
one exists; `./security-fix.sh` will report it every run until then.

Three further modules import vulnerable packages without calling the
vulnerable symbols. These are not upgraded, because no reachable exposure
means no risk is retired by upgrading.

---

## What was fixed rather than accepted

For contrast, and so the shape of the decision is clear:

- `nanoid` < 3.3.17 (high, GHSA-2v37-7h3g-55p8) — pinned to 3.3.18 via a
  range-scoped `overrides` entry in both `package.json` and
  `app/package.json`. A parent's declared range pinned the vulnerable version,
  so `npm audit fix` could not resolve it alone.
- `@spartan-ng/cli` — moved from `dependencies` to `devDependencies`. It is a
  scaffolding CLI that is never imported by application code, so its
  classification as a runtime dependency was simply wrong. Correcting it also
  removes its subtree, and the `image-size` chain with it, from the production
  audit.
