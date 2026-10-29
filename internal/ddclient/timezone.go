package ddclient

import (
	"os"
	"strings"
	"time"
	_ "time/tzdata" // Named zones also work on Windows and minimal installations.
)

// Resolve the system's IANA name, not an abbreviation or numeric UTC offset.
// macOS and Unix typically expose it in /etc/localtime; TZ also supports servers.
func localTimezone() string {
	candidates := []string{strings.TrimPrefix(os.Getenv("TZ"), ":"), time.Local.String()}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, zone, ok := strings.Cut(target, "zoneinfo/"); ok {
			candidates = append(candidates, zone)
		}
	}
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(data)))
	}
	for _, name := range candidates {
		if name == "" || name == "Local" {
			continue
		}
		if name != "UTC" && !strings.Contains(name, "/") {
			continue
		}
		if _, err := time.LoadLocation(name); err == nil {
			return name
		}
	}
	return ""
}
