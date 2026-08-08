# Change: 013-distribution-and-release

## Why

Change 004 delivered the desktop binary; nothing delivered the path from a
tagged commit to a user who has the software installed. Today that path does
not exist. There is no release workflow, no version string in any binary, no
signed artefact, no package-manager entry, and no published container image
for the server or the workers. A user who wants to run Trawl must clone the
repository and build it.

That is not a packaging gap, it is an adoption gap, and it is also a trust
gap. An unsigned binary downloaded over HTTPS is quarantined by Gatekeeper,
flagged by SmartScreen, and cannot be verified by anyone. A security tool that
asks to be trusted with an organisation's external attack surface while
shipping artefacts nobody can attest to is arguing against itself.

The sibling `checkmate-app` repository has a working release pipeline and is
the obvious starting point. It is also the obvious cautionary tale: it ships
unsigned, pins its Homebrew cask to `sha256 :no_check`, publishes no checksums
or provenance, labels a job "AppImage/Deb" while producing only a tarball, and
rewrites its own `go.mod` mid-build so no two builds are the same artefact.
This change adopts its structure and fixes each of those defects, in both
repositories.

## What Changes

- `release-versioning` — a single `pkg/version` package, stamped at link time
  from the git tag, consumed by the desktop binary, `trawl server`, and the
  worker entrypoints. One `-ldflags` value; one answer to "what am I running".
- `desktop-distribution` — tag-triggered cross-platform build producing macOS
  universal `.dmg`/`.zip`, Windows NSIS installer and portable `.zip` for
  `amd64` and `arm64`, and Linux `.tar.gz`, `.deb`, `.rpm` and `.AppImage` for
  `amd64` and `arm64`.
- `artefact-integrity` — `SHA256SUMS` over every published artefact, keyless
  cosign signatures, SPDX SBOMs, and SLSA build provenance. This, not a
  platform vendor's certificate, is where trust comes from: a Rekor-logged
  signature bound to the release workflow says more about an artefact's origin
  than a notarisation ticket does. Apple Developer ID signing plus
  `notarytool` notarisation and Windows Authenticode signing remain wired in
  but gated on repository secrets. Trawl holds no Apple Developer Program
  membership and does not intend to buy one — $99/year to be permitted to give
  free software away is not a cost this project will carry — so macOS builds
  are ad-hoc signed, which arm64 requires anyway and which gives the bundle a
  stable identity for keychain and TCC grants.
- `package-manager-distribution` — Homebrew cask (desktop) and Homebrew
  formula (headless CLI), winget and Scoop manifests, and `.deb`/`.rpm`
  packages, all auto-updated from the release.
- `container-distribution` — multi-architecture images for `trawl-server`,
  `trawl-dashboard` and each of the three workers, published to GHCR with
  SBOM and provenance attestations.
- `release-automation` — `scripts/release.sh` as the single human entry point:
  refuses a dirty tree, runs `./test.sh`, bumps the version in every file that
  carries one, verifies both builds, then tags and pushes.
- **Applied to `checkmate-app` as well.** The same capabilities land in the
  sibling repository, replacing its current pipeline. Its `go.mod` rewriting
  is removed in favour of a pinned dependency, because a build that mutates
  its own dependency graph cannot be reproduced and therefore cannot be
  attested to.

## Impact

- **Users** install with `brew install --cask trawl`, `winget install trawl`,
  or a double-click. The Homebrew and winget paths verify a checksum and open
  without further prompting. A direct `.dmg` download on macOS shows
  Gatekeeper's "Apple cannot check it" dialogue, because we have not paid
  Apple to check it; the documented remedy is to verify the download against
  `SHA256SUMS` and clear `com.apple.quarantine` from that one file.
- **Operators** pull `ghcr.io/adedayo/trawl-server:vX.Y.Z` instead of building
  from a compose file.
- **Auditors** can verify any artefact against its checksum, signature, SBOM
  and provenance without trusting the release process.
- **CI cost** rises: the release matrix is 12 build jobs. It runs on tags and
  on manual dispatch only, never on push.

## Explicitly Out of Scope

- Snap and Flatpak. Both carry ongoing store-review and confinement
  maintenance disproportionate to current demand; revisit on request.
- Self-hosted APT/YUM repositories. The `.deb` and `.rpm` are attached to the
  GitHub release; a hosted repository is a separate operational commitment.
- Auto-update inside the application. Distribution first, then update.
- Paid code-signing certificate procurement, which is a purchasing decision.
  The workflow is written to consume the secrets the moment they exist.
