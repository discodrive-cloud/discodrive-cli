// Package buildinfo reports the version the binaries were built as.
package buildinfo

import "runtime/debug"

// version is set at release time:
// -ldflags "-X github.com/discodrive-cloud/discodrive-cli/internal/buildinfo.version=v1.2.3".
var version = ""

// Version is the release version: the ldflags value, else the module version
// recorded by `go install …@vX`, else "dev" for a local build.
func Version() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
