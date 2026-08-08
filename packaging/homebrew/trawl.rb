cask "trawl" do
  # sha256 is written by the release workflow. It is deliberately not
  # :no_check — that setting tells Homebrew to install whatever happens to be
  # at the URL, which removes the only integrity check in the install path and
  # is a strange thing for a security tool to ask its users to accept.
  version "0.1.0"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"

  url "https://github.com/adedayo/trawl/releases/download/v#{version}/Trawl-macos-universal.dmg",
      verified: "github.com/adedayo/trawl/"
  name "Trawl"
  desc "Continuous external attack surface monitoring"
  homepage "https://github.com/adedayo/trawl"

  livecheck do
    url :url
    strategy :github_latest
  end

  depends_on macos: ">= :catalina"

  app "Trawl.app"

  # Trawl is ad-hoc signed, not notarised — see docs/distribution.md for why we
  # decline to pay Apple to give free software away. Homebrew has already done
  # the thing notarisation is a proxy for: it verified the download against the
  # sha256 above before we got here. Clearing the quarantine flag on that
  # verified bundle is therefore not a weakening of the install; it just stops
  # Gatekeeper re-asking a question Homebrew answered with better evidence.
  #
  # Scoped to this bundle only. Nothing here touches system-wide policy.
  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/Trawl.app"],
                   sudo: false
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
