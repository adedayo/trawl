# Design: 013-distribution-and-release

## Context

Trawl is not one artefact. It is a desktop application (`main.go` + Wails), a
headless server and CLI (`cmd/trawl`), and three containerised workers
(`jobs/*`). `checkmate-app` is a single desktop artefact. A design copied
straight across would under-serve Trawl and leave `checkmate-app` unchanged in
the ways that matter, so the shared design is expressed as capabilities rather
than as a file to copy.

## Goals

1. One tag produces every artefact for every platform, or produces none.
2. Every artefact is verifiable by someone who does not trust the builder.
3. Installation costs a user one command or one double-click, with no
   security dialogue to dismiss and no instruction to run `xattr -d`.
4. A fork with no secrets configured still gets a complete release.
5. The two repositories stay recognisably the same pipeline.

## Key Decisions

### Version stamping goes through a package, not a `var` in `main`

`checkmate-app` declares `var AppVersion` in `app.go` and stamps it with
`-X main.AppVersion`. That works for exactly one binary. Trawl has three, and
`-X main.AppVersion` cannot reach a package that is not `main`.

`pkg/version` holds the values and is stamped via
`-X github.com/adedayo/trawl/pkg/version.Version`. The desktop binary, the
server and the workers read the same package, and a single ldflags string
covers all of them. The package also carries `Commit`, `BuildDate` and
`runtime/debug.ReadBuildInfo()` fallbacks, so a `go install`-ed binary reports
its module version instead of `dev`.

The alternative — repeating `-X main.AppVersion` per binary — was rejected
because it makes the ldflags string a function of the binary being built, and
therefore something that drifts between the Dockerfile, the workflow and the
release script.

### Signing is optional but not conditional on being present

Every signing step is guarded on the secret being non-empty, and the workflow
succeeds either way. This is a fork-friendliness requirement, but it is also a
supply-chain one: the alternative — a workflow that only works with secrets —
means the only way to test a change to the release pipeline is to run it with
production signing credentials.

Cosign runs keyless via OIDC, so signature and transparency-log entry require
no secret at all and are therefore unconditional. Checksums are likewise
unconditional. The distinction is deliberate: identity-bearing signatures
(Apple, Authenticode) are optional; integrity evidence is not.

### Homebrew casks carry a real checksum

`checkmate-app` uses `sha256 :no_check`, which instructs Homebrew to install
whatever is at the URL. That defeats the only integrity check in the install
path. The release job computes the DMG digest and the bump action writes it
into the cask, so a tampered asset fails the install rather than completing
it.

### Linux gets real packages

The `.tar.gz` is kept for the "unpack it anywhere" case, but `nfpm` produces
`.deb` and `.rpm` with correct dependency declarations
(`libgtk-3-0`, `libwebkit2gtk-4.1-0`), a desktop entry and an icon, and
`linuxdeploy` produces an `.AppImage` for distributions whose WebKit is the
wrong vintage. This is what the `checkmate-app` job already claims to do.

### `checkmate-app` stops rewriting its own `go.mod`

Its build steps run `go mod edit -dropreplace`, `-droprequire`, then
`go get github.com/adedayo/checkmate@main`. Three consequences: the build
depends on the state of another repository's default branch at build time, so
it is not reproducible; a `v2.1.0` tag does not identify the code that was
built; and an SBOM or provenance statement over the result describes a
dependency graph that existed only during that run.

The fix is a `replace` directive for local development only, removed from the
committed `go.mod`, with `checkmate` pinned to a released version. The release
workflow then builds the repository as committed. Where a local checkout is
still wanted, `go.work` provides it without touching `go.mod`.

### Container images are per-role, not one image with a mode flag

Trawl already has five distinct Dockerfiles. Publishing five images with a
shared tag vocabulary (`latest`, `vX.Y.Z`, `vX.Y`, `sha-<short>`) keeps the
compose file declarative and lets an operator pin the scan worker without
pinning the server. Base images are pinned by digest so that a rebuild of an
old tag produces the old image.

## Risks / Trade-offs

- **Twelve build jobs per tag.** Mitigated by running on tags and dispatch
  only, and by caching Go and npm.
- **Notarisation is slow and occasionally flaky.** The step retries and, on
  persistent failure, fails the macOS job rather than shipping an unnotarised
  artefact under a signed label.
- **Universal macOS binaries double download size.** Accepted: a single
  download that works on both architectures removes the most common
  installation mistake.
- **`nfpm` and `linuxdeploy` are two more tools in the pipeline.** Both are
  pinned by version and neither is on the critical path for the tarball, so a
  breakage degrades to "no `.deb` this release" rather than "no release".

## Migration Plan

Sequenced so each phase is independently shippable:

1. Version plumbing and build assets. No user-visible change.
2. Desktop release workflow with checksums. First real release.
3. Release script and documentation.
4. Container matrix.
5. Package managers.
6. Signing, notarisation, SBOM and provenance.

`checkmate-app` follows the same sequence, with the `go.mod` de-hacking as its
phase 0.

## Open Questions

- None outstanding. Apple Developer Program membership has been declined: it
  is a $99/year rent for the right to give free software away, and this
  project will not pay it. macOS artefacts are therefore ad-hoc signed and
  not notarised. Trust is carried by `SHA256SUMS` and a keyless cosign
  signature — publicly logged in Rekor and bound to the release workflow,
  which is a stronger provenance claim than notarisation makes. Installation
  documentation states this plainly and offers only per-artefact remedies
  (`xattr -d com.apple.quarantine` on a verified download, or **Open
  Anyway**); the Homebrew cask does it in a `postflight` after Homebrew has
  verified the checksum. A Windows OV/EV certificate remains a purchasing
  decision, and the signing steps stay in place, guarded, in case that
  changes.
