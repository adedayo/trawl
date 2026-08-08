# Homebrew formula for the headless Trawl CLI and server.
#
# This is a separate artefact from the `trawl` cask, not a duplicate of it. The
# cask installs the desktop application; this installs the binary an operator
# runs as `trawl server` on a host with no display. Shipping only the cask
# would mean the server deployment path had no package manager entry at all.
class TrawlCli < Formula
  desc "Continuous external attack surface monitoring — headless server and CLI"
  homepage "https://github.com/adedayo/trawl"
  url "https://github.com/adedayo/trawl/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "Apache-2.0"
  head "https://github.com/adedayo/trawl.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/adedayo/trawl/pkg/version.Version=v#{version}
      -X github.com/adedayo/trawl/pkg/version.BuildDate=#{time.iso8601}
    ]

    # ./cmd/trawl only. The repository root is the Wails desktop binary, which
    # embeds a built Angular bundle via go:embed and therefore cannot compile
    # from a source tarball without a Node toolchain.
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/trawl"
  end

  service do
    run [opt_bin/"trawl", "server"]
    keep_alive true
    log_path var/"log/trawl.log"
    error_log_path var/"log/trawl.log"
  end

  test do
    # Asserts the ldflags path above is correct. A typo in that import path
    # produces a binary that silently reports "dev", which is the failure this
    # formula is most likely to ship.
    assert_match version.to_s, shell_output("#{bin}/trawl version")
    assert_match "Usage:", shell_output("#{bin}/trawl help")
  end
end
