#!/bin/sh
# Emit the Homebrew formula for dd-cli + dd-mcp of a given version.
# usage: gen-brew-formula.sh <version>   (e.g. 1.2.3) → formula on stdout
#
# A formula, not a cask: casks are macOS-only, and these binaries ship for Linux too.
# The macOS archives hold signed and notarized binaries.
#
# Downloads the four published archives to hash them, so the release assets must
# already exist on GitHub.
set -eu

VER="${1:?usage: gen-brew-formula.sh <version>}"
BASE="https://github.com/discodrive-cloud/discodrive-cli/releases/download/v${VER}"

. "$(dirname "$0")/lib-sha256-url.sh"
sha() { sha256_url "$BASE/discodrive-cli-${VER}-$1.tar.gz"; }

SHA_DARWIN_ARM64=$(sha darwin-arm64)
SHA_DARWIN_AMD64=$(sha darwin-amd64)
SHA_LINUX_ARM64=$(sha linux-arm64)
SHA_LINUX_AMD64=$(sha linux-amd64)

cat <<FORMULA
class DiscodriveCli < Formula
  desc "DiscoDrive from the terminal and inside your AI assistant (dd-cli, dd-mcp)"
  homepage "https://github.com/discodrive-cloud/discodrive-cli"
  version "${VER}"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "${BASE}/discodrive-cli-${VER}-darwin-arm64.tar.gz"
      sha256 "${SHA_DARWIN_ARM64}"
    end
    on_intel do
      url "${BASE}/discodrive-cli-${VER}-darwin-amd64.tar.gz"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    on_arm do
      url "${BASE}/discodrive-cli-${VER}-linux-arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM64}"
    end
    on_intel do
      url "${BASE}/discodrive-cli-${VER}-linux-amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  def install
    bin.install "dd-cli"
    bin.install "dd-mcp"
  end

  def caveats
    <<~EOS
      Sign in to your DiscoDrive server (this also connects Claude):
        dd-cli login --server https://your-server.example
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/dd-cli --version")
    assert_match version.to_s, shell_output("#{bin}/dd-mcp --version")
  end
end
FORMULA
