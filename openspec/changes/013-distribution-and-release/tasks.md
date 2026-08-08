# Tasks: 013-distribution-and-release

## Phase 0 — Version plumbing & build assets
- [x] Add `pkg/version/version.go` with `Version`/`Commit`/`BuildDate` and a
      `ReadBuildInfo()` fallback, plus unit tests.
- [x] Consume `pkg/version` in `main.go` (window title, startup log) and in
      `cmd/trawl` via a `trawl version` subcommand, with `run()` extracted so
      the CLI surface is testable without spawning a process.
- [x] Bind `GetVersion()` to the frontend from `app.go`.
- [x] Add `build/windows/` assets: `icon.ico` (16–256px, generated from
      `appicon.png`), `info.json`, `wails.exe.manifest`,
      `installer/project.nsi`. `wails_tools.nsh` is left to the Wails CLI,
      which regenerates it from `wails.json` on every build.
- [x] Fill both `build/darwin/*.plist` with `com.adedayo.trawl`, an app
      category, a network usage description and a 10.15 minimum.
- [x] Add product metadata (`info`) to `wails.json`.
- [x] Add `packaging/` and `.gitignore` entries for release output.

## Phase 1 — Desktop release workflow
- [x] Add `.github/workflows/release.yml` triggered on `v*` and
      `workflow_dispatch`, with a build-only dispatch mode.
- [x] macOS job: `darwin/universal`, DMG + ZIP, optional codesign/notarise.
- [x] Windows job: `amd64` + `arm64`, NSIS installer + portable ZIP,
      optional Authenticode signing.
- [x] Linux job: `amd64` + `arm64` tarball, `.deb`, `.rpm`, `.AppImage`.
- [x] Checksums, keyless cosign signatures and SPDX SBOMs for all assets.
- [x] `publish-release` job gated on every platform job succeeding.

## Phase 2 — Release automation & docs
- [x] Add `scripts/release.sh` with clean-tree, semver, upstream-sync, test
      and build gates, plus a `--dry-run` mode.
- [x] Add `scripts/validate-packaging.sh` and wire it into `test.sh` and a new
      CI `packaging` job alongside `actionlint`.
- [x] Add `RELEASING.md` and `docs/distribution.md`.

## Phase 3 — Container matrix
- [x] Add `.github/workflows/containers.yml` publishing the six role images
      for `linux/amd64` and `linux/arm64` with SBOM and provenance.
- [x] Stamp `pkg/version` into the server and cloudrun Dockerfiles via
      `ARG VERSION/COMMIT/BUILD_DATE`.
- [x] Replace `@latest` scanner-tool installs in the worker Dockerfiles with
      pinned `ARG` versions; pin `nginx` and Ofelia.
- [x] Give `deploy/compose/docker-compose.yml` published `image:` tags
      alongside `build:`, keyed on `TRAWL_VERSION`.

## Phase 4 — Package managers
- [x] `packaging/homebrew/trawl.rb` cask with a real `sha256`.
- [x] `packaging/homebrew/trawl-cli.rb` formula for the headless binary,
      including a service definition and a version-stamping test.
- [x] `packaging/scoop/trawl.json` and winget submission in the release job.
- [x] `packaging/linux/nfpm.yaml` + `trawl.desktop` for `.deb`/`.rpm`.

## Phase 5 — checkmate-app parity
- [x] Remove the `go mod edit` / `go get @main` rewriting from `Dockerfile`
      and `release.yml`; pin `github.com/adedayo/checkmate` to `v1.3.3` as a
      direct dependency and add `go.work.example` for local cross-repo work.
- [x] Add `pkg/version` and derive `AppVersion` from it; bind `GetBuildInfo()`.
- [x] Replace `release.yml` with the parity pipeline (universal macOS, dual-arch
      Windows and Linux, containers, checksums, cosign, SBOM, provenance).
- [x] Replace `sha256 :no_check` in the cask, and correct its `app` stanza —
      it declared `checkmate-app.app`, which no build has ever produced.
- [x] Add `.deb`, `.rpm` and `.AppImage` production to the Linux job.
- [x] Fix both plists: `com.adedayo.checkmate`, app category, 10.15 minimum.
- [x] Add `.nvmrc`, a `ci.yml` (the repository had no CI at all), a rewritten
      `test.sh`, `scripts/release.sh` and `scripts/validate-packaging.sh`.
- [x] Add `RELEASING.md` and `docs/distribution.md`.

## Phase 6 — Verification
- [ ] Dry-run the release workflow via `workflow_dispatch` build-only mode in
      both repositories.
- [ ] Verify a downloaded artefact against `SHA256SUMS` and its cosign
      signature from a clean machine.
- [ ] Confirm `brew install --cask` opens the app with no Gatekeeper prompt —
      the `postflight` quarantine strip is the mechanism, and it is the only
      install path where that claim should hold. Direct `.dmg` downloads are
      expected to prompt; verify the documented `xattr` remedy works instead.
