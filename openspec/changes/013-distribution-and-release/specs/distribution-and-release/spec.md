# Capability: distribution-and-release

The `distribution-and-release` capability turns a git tag into installable,
verifiable artefacts on every platform Trawl supports, and keeps the sibling
`checkmate-app` repository on the same pipeline.

## ADDED Requirements

### Requirement: Single Source Of Version Truth
Every Trawl binary MUST report the same version, derived from one stamped package. `pkg/version` MUST expose `Version`, `Commit` and `BuildDate`, stamped at link time via `-X github.com/adedayo/trawl/pkg/version.Version=<tag>`.

#### Scenario: All binaries agree
- **GIVEN** a single build of the desktop application, `trawl server` and every worker
- **WHEN** each reports its version
- **THEN** all report the identical string

#### Scenario: Unstamped binary falls back to build info
- **GIVEN** a binary produced by `go install` with no link-time stamp
- **WHEN** it reports its version
- **THEN** it reports its module version from `runtime/debug.ReadBuildInfo()` rather than `dev`

#### Scenario: Version subcommand
- **GIVEN** any Trawl binary
- **WHEN** `trawl version` is run
- **THEN** it prints version, commit and build date and exits zero

### Requirement: Tag-Triggered Cross-Platform Release
A tag matching `v*` MUST produce a complete set of desktop artefacts, and MUST publish nothing if any platform build fails.

#### Scenario: Full artefact matrix
- **GIVEN** a pushed tag matching `v*`
- **WHEN** the release workflow completes
- **THEN** it has produced macOS `darwin/universal` as `.dmg` and `.zip`, Windows `amd64` and `arm64` as an NSIS installer and portable `.zip`, and Linux `amd64` and `arm64` as `.tar.gz`, `.deb`, `.rpm` and `.AppImage`

#### Scenario: Ordinary pushes do not release
- **GIVEN** a push to a branch that is not a tag
- **WHEN** workflow triggers are evaluated
- **THEN** the release workflow does not run; only tag push and `workflow_dispatch` trigger it

#### Scenario: Partial failure publishes nothing
- **GIVEN** a release run in which one platform build fails
- **WHEN** the workflow concludes
- **THEN** no assets are published for any platform

#### Scenario: Pipeline is testable without releasing
- **GIVEN** a change to the release pipeline
- **WHEN** the workflow is invoked via `workflow_dispatch` in build-only mode
- **THEN** artefacts are produced without a release being created

### Requirement: Artefact Integrity Evidence
Every published artefact MUST be independently verifiable, and integrity evidence MUST NOT be conditional on optional signing secrets.

#### Scenario: Checksums and signatures always present
- **GIVEN** a completed release, with no signing secrets configured
- **WHEN** the published assets are inspected
- **THEN** a `SHA256SUMS` file covers every asset, and each asset carries a keyless cosign signature with a transparency-log entry

#### Scenario: Supply-chain documents attached
- **GIVEN** a completed release
- **WHEN** its assets are inspected
- **THEN** an SPDX SBOM accompanies each binary artefact and each container image, and SLSA build provenance accompanies each container image

### Requirement: Optional Platform Code Signing
Platform signing MUST be applied when credentials exist and skipped cleanly when they do not. Trawl holds no Apple Developer Program membership and does not intend to acquire one, so the credential-absent path is the normal path, not a degraded one.

#### Scenario: Apple secrets present
- **GIVEN** a release run with the Apple signing secrets configured
- **WHEN** the macOS artefacts are built
- **THEN** they are codesigned, notarised, and the notarisation ticket is stapled

#### Scenario: Apple secrets absent
- **GIVEN** a release run with no Apple signing secrets
- **WHEN** the macOS artefacts are built
- **THEN** the bundle is ad-hoc signed so it executes on arm64 and carries a stable code identity for keychain, TCC and firewall grants, and the run completes successfully

#### Scenario: Windows signing is conditional
- **GIVEN** a release run
- **WHEN** the Windows installer and executable are built
- **THEN** they are Authenticode-signed if the Windows signing secrets are present, and the run still succeeds if they are absent

#### Scenario: No false notarisation claim
- **GIVEN** an artefact that was not notarised
- **WHEN** it is published and described
- **THEN** it is never labelled Developer ID signed or notarised

#### Scenario: Notarisation failure after successful signing
- **GIVEN** a run in which Developer ID signing succeeded but notarisation did not
- **WHEN** the platform job concludes
- **THEN** it fails rather than publishing an unnotarised artefact

### Requirement: Package Manager Distribution
Installation MUST cost the user a single command, or a download and a double-click, on each major platform. Every manifest under `packaging/` MUST have a release job that publishes it.

#### Scenario: Homebrew publication
- **GIVEN** a completed release
- **WHEN** `adedayo/homebrew-tap` is inspected
- **THEN** it carries an updated cask for the desktop application and an updated formula for the headless CLI, each with a real computed `sha256` and never `:no_check`

#### Scenario: Windows installer is mandatory
- **GIVEN** a release run that fails to produce a per-user NSIS installer
- **WHEN** the workflow concludes
- **THEN** the release fails

#### Scenario: No orphaned manifest
- **GIVEN** a manifest committed under `packaging/`
- **WHEN** packaging validation runs
- **THEN** it fails unless a release job publishes that manifest, because an unpublished manifest documents an install path that does not work

#### Scenario: Remedies are per-artefact
- **GIVEN** documentation for an artefact that is not notarised
- **WHEN** it is read
- **THEN** it states the artefact is not notarised, explains this reflects the absence of a paid Apple membership rather than an unverified artefact, and offers only per-artefact remedies such as clearing `com.apple.quarantine` on a checksum-verified download or **Open Anyway** — never disabling Gatekeeper globally

#### Scenario: Cask clears quarantine
- **GIVEN** a cask installation, where Homebrew has already verified the download against a real `sha256`
- **WHEN** the `postflight` runs
- **THEN** `com.apple.quarantine` is cleared from the installed bundle

### Requirement: Container Distribution
Every containerised role MUST be published as a multi-architecture image with pinned bases.

#### Scenario: All roles published
- **GIVEN** a completed release
- **WHEN** GHCR is inspected
- **THEN** `trawl-server`, `trawl-dashboard`, `trawl-scan-worker`, `trawl-discovery-worker` and `trawl-repo-scan-worker` are present, each built for `linux/amd64` and `linux/arm64` and tagged `latest`, `vX.Y.Z`, `vX.Y` and `sha-<short>`

#### Scenario: Rebuilding an old tag reproduces the old image
- **GIVEN** a Dockerfile whose base images are pinned by digest
- **WHEN** an old tag is rebuilt
- **THEN** the resulting image matches the original

#### Scenario: Compose uses published images
- **GIVEN** the committed compose files
- **WHEN** they are inspected
- **THEN** they reference published images by tag rather than building from source

### Requirement: Reproducible Release Builds
A release build MUST build the repository as committed.

#### Scenario: Dependency graph is not mutated
- **GIVEN** a release build
- **WHEN** it runs
- **THEN** `go.mod` is not modified and no dependency resolves to a moving reference such as `@main`

#### Scenario: Multi-repository development
- **GIVEN** a developer working across Trawl and a sibling repository
- **WHEN** they wire the local checkout in
- **THEN** they use `go.work`, and no `replace` directive is committed

### Requirement: Single Human Release Entry Point
Cutting a release MUST be one command with deterministic preconditions, and MUST do nothing the workflow is responsible for.

#### Scenario: Preconditions enforced
- **GIVEN** `scripts/release.sh vX.Y.Z`
- **WHEN** it is invoked on a dirty tree, with a version failing the semver pattern, or with a failing `./test.sh`
- **THEN** it refuses to proceed

#### Scenario: Version files updated consistently
- **GIVEN** a valid release invocation
- **WHEN** the script runs
- **THEN** every file carrying a version is updated in one commit, and both the Go engine and the Angular bundle are verified to build before tagging

#### Scenario: Script tags and stops
- **GIVEN** a successful release invocation
- **WHEN** the script finishes
- **THEN** it has created and pushed an annotated tag, and has not built or uploaded any artefact

### Requirement: Sibling Repository Parity
`checkmate-app` MUST be brought onto the same pipeline, and divergence MUST be a deliberate consequence of a repository difference rather than drift.

#### Scenario: Same guarantees apply
- **GIVEN** the `checkmate-app` pipeline
- **WHEN** it is assessed against this capability
- **THEN** it satisfies the versioning, integrity, signing, package-manager and reproducibility requirements above

#### Scenario: Build-time rewriting removed
- **GIVEN** the `checkmate-app` Dockerfile and release workflow
- **WHEN** they are inspected
- **THEN** they contain no `go mod edit` or `go get @main` rewriting

#### Scenario: Advertised formats are produced
- **GIVEN** the `checkmate-app` release job
- **WHEN** it completes
- **THEN** its cask carries a computed digest rather than `sha256 :no_check`, and it has produced the Linux package formats it advertises
