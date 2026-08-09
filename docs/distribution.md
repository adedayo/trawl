# Installing Trawl

Trawl ships as a desktop application, a headless server, and a set of container
images. Pick the one that matches how you intend to run it.

---

## Desktop

### macOS

```sh
brew install --cask adedayo/tap/trawl
```

The cask verifies the download's SHA-256 and clears the quarantine flag for
you, so the app opens normally. If you install this way you can skip the rest
of this section.

Or download `Trawl-macos-universal.dmg` from the
[latest release](https://github.com/adedayo/trawl/releases/latest). The build
is universal, so one download works on both Apple Silicon and Intel.

#### Why macOS complains, and what to do about it

Trawl is ad-hoc signed, not Developer ID signed. Apple only issues Developer ID
certificates to members of its Developer Program, which costs $99 a year.
Trawl is free software given away for the community's benefit, and it is not
going to charge you — directly or indirectly — to fund a rent to Apple for
permission to do that. So on first launch macOS will say the app "cannot be
opened because Apple cannot check it for malicious software."

That message is about *provenance*, not about the file being damaged or
dangerous. Apple has not checked it because we have not paid Apple to check it.
Establish provenance yourself instead — this is stronger evidence than
notarisation, because it ties the artefact to the public build that produced
it:

```sh
# 1. Confirm the bytes are the bytes we published.
shasum -a 256 -c SHA256SUMS --ignore-missing

# 2. Confirm we published them, from our release workflow. (See "Verifying a
#    download" below for cosign.)
```

Then clear the quarantine attribute on the file you just verified:

```sh
xattr -d com.apple.quarantine ~/Downloads/Trawl-macos-universal.dmg
# after dragging to /Applications:
xattr -dr com.apple.quarantine /Applications/Trawl.app
```

> This is deliberately narrow. It removes a flag from **one file you have just
> verified**. It is not `sudo spctl --master-disable`, which turns Gatekeeper
> off for everything you will ever download, and which you should not run for
> this or any other application. If you would rather not touch the terminal,
> **System Settings → Privacy & Security → Open Anyway** grants the same
> per-application exception through the UI.

### Windows

```powershell
winget install Adedayo.Trawl
```

Or download `Trawl-windows-amd64-installer.exe` (or `-arm64-`). The installer
is per-user and does not request administrator rights — Trawl writes only to
your own profile, and a security tool that asks for elevation it does not need
trains you to grant it to things that do.

### Linux

| Format | Use when |
|---|---|
| `.deb` | Debian, Ubuntu and derivatives — declares its GTK/WebKit dependencies |
| `.rpm` | Fedora, RHEL, openSUSE |
| `.AppImage` | Your distribution's WebKit is the wrong vintage, or you want no install at all |
| `.tar.gz` | You want to unpack it somewhere and manage dependencies yourself |

```sh
sudo apt install ./trawl_1.2.3_amd64.deb
# or
sudo dnf install ./trawl-1.2.3.x86_64.rpm
# or
chmod +x Trawl-linux-amd64.AppImage && ./Trawl-linux-amd64.AppImage
```

The tarball contains a bare binary. It will not start without
`libgtk-3-0` and `libwebkit2gtk-4.1-0`; the `.deb` and `.rpm` declare those,
which is the reason to prefer them.

---

## Headless server and CLI

```sh
brew install adedayo/tap/trawl-cli
trawl server
```

Homebrew installs a service definition, so `brew services start trawl-cli`
runs it under launchd or systemd.

Or build from source:

```sh
go install github.com/adedayo/trawl/cmd/trawl@latest
```

A `go install`-ed binary is not stamped by our release pipeline, but it still
reports a real version — `pkg/version` falls back to the module version and
VCS stamp the Go toolchain embeds.

---

## Containers

Six images, one per role, `linux/amd64` and `linux/arm64`:

| Image | Role |
|---|---|
| `ghcr.io/adedayo/trawl-server` | Ingest target, job broker, read API |
| `ghcr.io/adedayo/trawl-dashboard` | nginx serving the Angular bundle |
| `ghcr.io/adedayo/trawl-cloudrun` | Server with the dashboard embedded — single container |
| `ghcr.io/adedayo/trawl-scan-worker` | Port, HTTP and vulnerability scanning |
| `ghcr.io/adedayo/trawl-discovery-worker` | Passive asset discovery |
| `ghcr.io/adedayo/trawl-repo-scan-worker` | Repository secret scanning |

They are separate rather than one image with a mode flag because the roles have
genuinely different dependencies — the server is a static Go binary, the scan
worker carries a scanning toolchain — and collapsing them would mean every
operator pulls the scanner to run the API.

```sh
cd deploy/compose
TRAWL_VERSION=v1.2.3 docker compose up -d
```

Tags are `vX.Y.Z`, `vX.Y`, `sha-<short>` and `latest`. `latest` is a
convenience for evaluation. Pin `TRAWL_VERSION` for anything you depend on.

---

## Verifying a download

Every asset is checksummed and signed. Neither requires trusting this
repository's secrets: Sigstore signing is keyless, against the workflow's own
OIDC identity.

```sh
# Integrity
sha256sum -c SHA256SUMS --ignore-missing

# Provenance — proves this artefact was built by this workflow, in this repo
cosign verify-blob \
  --certificate Trawl-macos-universal.dmg.pem \
  --signature   Trawl-macos-universal.dmg.sig \
  --certificate-identity-regexp '^https://github.com/adedayo/trawl/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  Trawl-macos-universal.dmg
```

For container images:

```sh
gh attestation verify oci://ghcr.io/adedayo/trawl-server:v1.2.3 --owner adedayo
docker buildx imagetools inspect ghcr.io/adedayo/trawl-server:v1.2.3 \
  --format '{{ json .SBOM }}'
```

Each release also carries an SPDX SBOM. An image nobody can enumerate the
contents of is an image nobody can tell you is vulnerable.

---

## Reproducibility

Release builds build the repository exactly as committed. No dependency is
resolved to a moving reference, no `go.mod` is rewritten mid-build, and every
base image and scanner tool is pinned to a version. This is what makes an SBOM
or a provenance statement mean anything: without it, an attestation describes a
dependency graph that existed only during one CI run.

Renovate proposes upgrades to those pins. Taking one is a reviewed change, not
a side effect of rebuilding.
