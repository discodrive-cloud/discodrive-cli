#!/usr/bin/env bash
# Cross-build dd-cli and dd-mcp for every released platform.
#
# Usage: scripts/build-release.sh <version>   (e.g. 1.2.3; "dev" for a local check)
#
# Output: dist/<os>-<arch>/dd-cli[.exe] and dd-mcp[.exe]. Static (CGO off), stripped,
# with the version stamped into internal/buildinfo. Builds only — signing the macOS
# binaries is a separate step that never runs in the job that builds.
set -euo pipefail

ver="${1:?usage: $0 <version>}"
pkg="github.com/discodrive-cloud/discodrive-cli/internal/buildinfo"
targets=(darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64)

rm -rf dist
for t in "${targets[@]}"; do
  os="${t%/*}" arch="${t#*/}" ext=""
  [ "$os" = windows ] && ext=".exe"
  out="dist/$os-$arch"
  mkdir -p "$out"
  for cmd in dd-cli dd-mcp; do
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
      -ldflags "-s -w -X $pkg.version=v$ver" -o "$out/$cmd$ext" "./cmd/$cmd"
  done
  echo "built $out"
done
