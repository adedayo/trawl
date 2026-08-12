# Capability: distribution-and-release

The `distribution-and-release` capability turns a git tag into installable,
verifiable artefacts on every platform Trawl supports, and keeps the sibling
`checkmate-app` repository on the same pipeline.

## Requirements

### Requirement: Single Source Of Version Truth
Every Trawl binary MUST report the same version, derived from one stamped
package.

#### Requirements
- MUST expose `Version`, `Commit` and `BuildDate` from `pkg/version`.
- MUST be stamped at link time via
  `-X github.com/adedayo/trawl/pkg/version.Version=<tag>`.
- MUST fall back to `runtime/debug.ReadBuildInfo()` when unstamped, so a
  `go install`-ed binary reports its module version rather than `dev`.
- The desktop application, `trawl server` and every worker MUST report the
  identical string for a given build.
- `trawl version` MUST print version, commit and build date and exit zero.

### Requirement: Tag-Triggered Cross-Platform Release
A tag matching `v*` MUST produce a complete set of desktop artefacts.

#### Requirements
- MUST produce macOS `darwin/universal` as both `.dmg` and `.zip`.
- MUST produce Windows `amd64` and `arm64` as an NSIS installer and a
  portable `.zip`.
- MUST produce Linux `amd64` and `arm64` as `.tar.gz`, `.deb`, `.rpm` and
  `.AppImage`.
- MUST NOT run on ordinary pushes; tag push and `workflow_dispatch` only.
- MUST publish nothing if any platform build fails.
- MUST be runnable via `workflow_dispatch` in a build-only mode that produces
  artefacts without creating a release, so pipeline changes are testable.

### Requirement: Artefact Integrity Evidence
Every published artefact MUST be independently verifiable.

#### Requirements
- MUST publish a `SHA256SUMS` file covering every released asset.
- MUST produce a keyless cosign signature and transparency-log entry for each
  asset, requiring no repository secret.
- MUST attach an SPDX SBOM for each binary artefact and each container image.
- MUST attach SLSA build provenance for each container image.
- Integrity evidence MUST NOT be conditional on optional signing secrets.

### Requirement: Optional Platform Code Signing
Platform signing MUST be applied when credentials exist and skipped cleanly
when they do not. Trawl does not hold an Apple Developer Program membership
and does not intend to acquire one, so the credential-absent path is the
normal path, not a degraded one.

#### Requirements
- MUST codesign and notarise the macOS artefacts when the Apple secrets are
  present, and MUST staple the notarisation ticket.
- MUST ad-hoc sign the macOS bundle when the Apple secrets are absent, so
  that the binary executes on arm64 and carries a stable code identity for
  keychain, TCC and firewall grants.
- MUST Authenticode-sign the Windows installer and executable when the
  Windows signing secrets are present.
- MUST complete successfully, producing verifiable but not notarised
  artefacts, when those secrets are absent.
- MUST NOT publish an artefact described as Developer ID signed or notarised
  if that did not occur.
- MUST fail the platform job rather than publish an unnotarised artefact when
  Developer ID signing succeeded but notarisation did not.

### Requirement: Package Manager Distribution
Installation MUST cost the user a single command, or a download and a
double-click, on each major platform.

#### Requirements
- MUST publish a Homebrew cask for the desktop application to
  `adedayo/homebrew-tap`, updated automatically on release.
- MUST publish a Homebrew formula for the headless CLI.
- MUST publish a per-user NSIS installer for Windows, and the release MUST
  fail if one is not produced. Windows is deliberately served by a download
  rather than a package manager: winget requires a manifest in Microsoft's
  central repository, which is recurring work for a channel most users reach
  past on their way to the Releases page, and Scoop's distinguishing benefit
  — installing without administrator rights — is something the NSIS installer
  already provides.
- Every manifest under `packaging/` MUST have a release job that publishes
  it. An unpublished manifest documents an install path that does not work.
- Homebrew formulae and casks MUST carry a real `sha256`. `:no_check` MUST
  NOT be used.
- Installation MUST NOT require the user to weaken a system-wide security
  control. Where an artefact is not notarised, the documentation MUST say so,
  MUST explain that this reflects the absence of a paid Apple membership
  rather than an unverified artefact, and MUST offer only per-artefact
  remedies — clearing `com.apple.quarantine` on a checksum-verified download,
  or **Open Anyway**. It MUST NOT instruct the user to disable Gatekeeper
  globally.
- The Homebrew cask MUST clear `com.apple.quarantine` from the installed
  bundle in a `postflight`, since Homebrew has already verified the download
  against a real `sha256`.

### Requirement: Container Distribution
Every containerised role MUST be published as a multi-architecture image.

#### Requirements
- MUST publish `trawl-server`, `trawl-dashboard`, `trawl-scan-worker`,
  `trawl-discovery-worker` and `trawl-repo-scan-worker` to GHCR.
- MUST build `linux/amd64` and `linux/arm64` for each.
- MUST tag each image `latest`, `vX.Y.Z`, `vX.Y` and `sha-<short>`.
- MUST pin base images by digest so that rebuilding an old tag reproduces the
  old image.
- The committed compose files MUST reference published images by tag rather
  than building from source.

### Requirement: Reproducible Release Builds
A release build MUST build the repository as committed.

#### Requirements
- MUST NOT mutate `go.mod` during a release build.
- MUST NOT resolve any dependency to a moving reference such as `@main`.
- Local multi-repository development MUST use `go.work` rather than a
  committed `replace` directive.

### Requirement: Single Human Release Entry Point
Cutting a release MUST be one command with deterministic preconditions.

#### Requirements
- `scripts/release.sh vX.Y.Z` MUST refuse to run on a dirty tree.
- MUST validate the version against a semver pattern.
- MUST run `./test.sh` and abort on failure.
- MUST update every file carrying a version, consistently, in one commit.
- MUST verify that both the Go engine and the Angular bundle build before
  tagging.
- MUST create an annotated tag and push it, and MUST do nothing else that the
  workflow is responsible for.

### Requirement: Sibling Repository Parity
`checkmate-app` MUST be brought onto the same pipeline.

#### Requirements
- MUST satisfy the versioning, integrity, signing, package-manager and
  reproducibility requirements above.
- MUST remove the `go mod edit` / `go get @main` rewriting from its
  Dockerfile and workflow.
- MUST replace `sha256 :no_check` in its cask with a computed digest.
- MUST produce the Linux package formats its release job advertises.
- Divergence between the two pipelines MUST be a deliberate consequence of a
  repository difference, not drift.
