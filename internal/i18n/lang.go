// Package i18n localizes the user-facing output of dd-cli/dd-mcp into the server's languages.
package i18n

import (
	"slices"
	"strings"
)

type Lang string

const Default Lang = "en"

// Supported lists the DiscoDrive server's languages (web/i18n/locales). en is the source and default.
var Supported = []Lang{"en", "de", "es", "fr", "ru", "sr", "uk"}

func isSupported(l Lang) bool {
	return slices.Contains(Supported, l)
}

// Resolve selects the language using a local priority chain: flag (--lang) →
// DISCODRIVE_LANG → LC_ALL → LANG → en. Empty and unsupported values are skipped.
func Resolve(flag string, getenv func(string) string) Lang {
	for _, cand := range []string{flag, getenv("DISCODRIVE_LANG"), getenv("LC_ALL"), getenv("LANG")} {
		if l, ok := normalize(cand); ok {
			return l
		}
	}
	return Default
}

// normalize extracts the 2-letter code from values like "de", "de_DE.UTF-8",
// "fr-FR", "RU" and checks that it is supported.
func normalize(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "_-.@"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "", false
	}
	l := Lang(s)
	if isSupported(l) {
		return l, true
	}
	return "", false
}
