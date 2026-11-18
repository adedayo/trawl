# Accepted risk

Vulnerabilities that are known, unfixable today, and carried deliberately.

The point of this file is that an unfixable vulnerability written down is an
accepted risk, whereas one left unstated is priced at zero. `./security-fix.sh`
and `./scripts/check-vulns.sh` report these on every run; this file records the
decision so that "still open" does not quietly become "forgotten".

Review on every release. Once an upstream fix ships, take it and delete the
entry.

---

## Go engine - zero reachable advisories

`govulncheck ./...` reports **0** reachable vulnerabilities across `stdlib`
(`go 1.26.11`) and all direct and transitive modules (`subfinder/v2 v2.17.0`
retired the transitive `golang.org/x/crypto/openpgp` `GO-2026-5932` import).

---

## npm - build and test tooling only, nothing shipped

Production dependencies audit clean in both workspaces:

```sh
npm audit --omit=dev                  # found 0 vulnerabilities
npm --prefix app audit --omit=dev     # found 0 vulnerabilities
```

### `picomatch <= 4.0.7` - no upstream patch published yet

| Field | Detail |
|---|---|
| Advisory | GHSA-fc5q-39xc-vjpf (CWE-1333, ReDoS through single-branch repeated extglobs) |
| Severity | High (CVSS 7.5) |
| Patched version | **None** - `4.0.7` is the latest published version on npm |
| Scope | Build and test toolchain only (`vitest`, `vite`, `@angular/build`, `@angular/cli`, `@spartan-ng/cli`, `@fission-ai/openspec`) |

Every remaining dev-only count reported by `npm audit` (7 in `root`, 26 in
`app`) is this single `picomatch <= 4.0.7` advisory propagated up its
dev-dependency reverse graph (`tinyglobby`, `fdir`, `vite`, `vitest`,
`@angular-devkit/*`, `@nx/*`).

Accepted because no patched version exists yet and `picomatch` is absent from
the shipped Angular bundle and Go binaries: it matches glob patterns over this
repository's own source paths during local and CI builds.
