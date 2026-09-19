cask "trawl" do
  # sha256 is written by the release workflow. It is deliberately not
  # :no_check — that setting tells Homebrew to install whatever happens to be
  # at the URL, which removes the only integrity check in the install path and
  # is a strange thing for a security tool to ask its users to accept.
  version "0.1.0"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"

  # No `verified:` parameter. It is deprecated: it existed to assert that a URL
  # whose host differs from the homepage is nevertheless the right one, and
  # Homebrew now derives that itself.
  url "https://github.com/adedayo/trawl/releases/download/v#{version}/Trawl-macos-universal.dmg"
  name "Trawl"
  desc "Continuous external attack surface monitoring"
  homepage "https://github.com/adedayo/trawl"

  livecheck do
    url :url
    strategy :github_latest
  end

  # Deliberately unversioned. Homebrew 6.0.22 *disabled* `depends_on macos:
  # :catalina` — "There is no replacement" — and the string form `">= :catalina"`
  # before it. Either makes the cask uninstallable outright:
  #
  #   Error: Calling `depends_on macos: :catalina` is disabled!
  #
  # Nothing is lost. The stanza only produced a friendlier message on releases
  # older than Catalina, which cannot run a universal 64-bit bundle and which
  # Homebrew no longer supports. The bare form stays because `brew style`
  # requires it for a cask with a macOS-only artifact.
  depends_on :macos

  app "Trawl.app"

  # Trawl is ad-hoc signed, not notarised — see docs/distribution.md for why we
  # decline to pay Apple to give free software away. Homebrew has already done
  # the thing notarisation is a proxy for: it verified the download against the
  # sha256 above before we got here. Clearing the quarantine flag on that
  # verified bundle is therefore not a weakening of the install; it just stops
  # Gatekeeper re-asking a question Homebrew answered with better evidence.
  #
  # Scoped to this bundle only. Nothing here touches system-wide policy.
  #
  # `postflight_steps` and not `postflight`: arbitrary-Ruby flight blocks are
  # deprecated. `{{appdir}}` is a template token Homebrew expands at install
  # time; it cannot be written as `#{appdir}` because a steps block is
  # evaluated before any cask paths exist. `must_succeed: false` keeps the
  # tolerance the old non-bang `system_command` had — a bundle carrying no such
  # attribute must not turn a successful install into a failed one.
  postflight_steps do
    run "/usr/bin/xattr",
        args:         ["-dr", "com.apple.quarantine", "{{appdir}}/Trawl.app"],
        must_succeed: false
  end

  # Everything Trawl writes outside its own bundle. A scanner accumulates a
  # database of an organisation's external estate; leaving that behind after an
  # uninstall is a data-retention problem, not an untidiness problem.
  zap trash: [
    "~/.trawl",
    "~/Library/Application Support/Trawl",
    "~/Library/Preferences/com.adedayo.trawl.plist",
    "~/Library/Saved Application State/com.adedayo.trawl.savedState",
    "~/Library/WebKit/com.adedayo.trawl",
  ]
end
