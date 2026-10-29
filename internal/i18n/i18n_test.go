package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	// flag wins over everything
	if got := Resolve("de", env(map[string]string{"DISCODRIVE_LANG": "fr", "LANG": "es"})); got != "de" {
		t.Fatalf("flag must win: %q", got)
	}
	// DISCODRIVE_LANG wins over LC_ALL/LANG
	if got := Resolve("", env(map[string]string{"DISCODRIVE_LANG": "fr", "LC_ALL": "es", "LANG": "uk"})); got != "fr" {
		t.Fatalf("DISCODRIVE_LANG must win over the env locale: %q", got)
	}
	// LC_ALL wins over LANG
	if got := Resolve("", env(map[string]string{"LC_ALL": "es", "LANG": "uk"})); got != "es" {
		t.Fatalf("LC_ALL must win over LANG: %q", got)
	}
	// empty → en
	if got := Resolve("", env(nil)); got != Default {
		t.Fatalf("empty must yield en: %q", got)
	}
}

func TestNormalizeLocaleValues(t *testing.T) {
	cases := map[string]Lang{
		"de":          "de",
		"de_DE.UTF-8": "de",
		"fr-FR":       "fr",
		"RU":          "ru",
		"uk_UA":       "uk",
		"en_US.UTF-8": "en",
		"C":           Default, // unsupported → falls back in Resolve
		"":            Default,
		"xx":          Default,
	}
	for in, want := range cases {
		if got := Resolve(in, func(string) string { return "" }); got != want {
			t.Errorf("Resolve(%q) = %q, expected %q", in, got, want)
		}
	}
}

func TestTFallbackToEnglish(t *testing.T) {
	// Genuinely exercise the active→en branch: a synthetic key absent from de
	// but present in en. (Ordinary keys exist in every locale — TestLocalesComplete —
	// so the fallback branch never triggers on them.)
	loadOnce.Do(load)
	const k = "test.only.en.fallback"
	catalog[Default][k] = "EN-ONLY"
	defer delete(catalog[Default], k)
	SetActive("de")
	defer SetActive(Default)
	if got := T(k); got != "EN-ONLY" {
		t.Fatalf("expected the de→en fallback (%q), got %q", "EN-ONLY", got)
	}
}

func TestTUnknownKeyReturnsKey(t *testing.T) {
	if got := T("nope.such.key"); got != "nope.such.key" {
		t.Fatalf("an unknown key must be returned as is: %q", got)
	}
}

func TestTPlaceholders(t *testing.T) {
	SetActive(Default)
	if got := T("core.hello", M{"Name": "Bob"}); got != "Hello, Bob" {
		t.Fatalf("placeholder not substituted: %q", got)
	}
	SetActive("ru")
	defer SetActive(Default)
	if got := T("core.hello", M{"Name": "Bob"}); got != "Привет, Bob" {
		t.Fatalf("ru placeholder: %q", got)
	}
}

// TestLocalesSubsetOfEnglish: a locale must not have keys that are absent from en.
func TestLocalesSubsetOfEnglish(t *testing.T) {
	loadOnce.Do(load)
	en := catalog[Default]
	if len(en) == 0 {
		t.Fatal("en.json is empty or failed to load")
	}
	for _, l := range Supported {
		for k := range catalog[l] {
			if _, ok := en[k]; !ok {
				t.Errorf("%s.json: key %q is absent from en.json", l, k)
			}
		}
	}
}

// TestLocalesComplete: every locale contains ALL of en's keys (once translated).
func TestLocalesComplete(t *testing.T) {
	loadOnce.Do(load)
	en := catalog[Default]
	for _, l := range Supported {
		miss := 0
		for k := range en {
			if _, ok := catalog[l][k]; !ok {
				if miss < 10 {
					t.Errorf("%s.json: missing key %q", l, k)
				}
				miss++
			}
		}
		if miss > 0 {
			t.Errorf("%s.json: %d keys missing in total", l, miss)
		}
	}
}

// TestPlaceholderParity: the set of {Name} placeholders in each translation matches en —
// otherwise substitution breaks (an extra placeholder stays unfilled, a missing one drops data).
func TestPlaceholderParity(t *testing.T) {
	loadOnce.Do(load)
	rx := regexp.MustCompile(`\{(\w+)\}`)
	set := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, g := range rx.FindAllStringSubmatch(s, -1) {
			m[g[1]] = true
		}
		return m
	}
	en := catalog[Default]
	for _, l := range Supported {
		for k, ev := range en {
			tv, ok := catalog[l][k]
			if !ok {
				continue // covered by TestLocalesComplete
			}
			e, a := set(ev), set(tv)
			if len(e) != len(a) {
				t.Errorf("%s.json: key %q — placeholders diverge: en=%v %s=%v", l, k, keysOf(e), l, keysOf(a))
				continue
			}
			for p := range e {
				if !a[p] {
					t.Errorf("%s.json: key %q — placeholder {%s} is missing", l, k, p)
				}
			}
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestTrailingNewlineParity: whether a translation has a trailing \n must match en.
// Some strings are printed via Fprint (the newline must live inside the value); losing the \n
// in a translation would run the output together, and key/placeholder parity does not catch that.
func TestTrailingNewlineParity(t *testing.T) {
	loadOnce.Do(load)
	en := catalog[Default]
	for _, l := range Supported {
		for k, ev := range en {
			tv, ok := catalog[l][k]
			if !ok {
				continue
			}
			if strings.HasSuffix(ev, "\n") != strings.HasSuffix(tv, "\n") {
				t.Errorf("%s.json: key %q — trailing \\n diverges from en (en=%v, %s=%v)",
					l, k, strings.HasSuffix(ev, "\n"), l, strings.HasSuffix(tv, "\n"))
			}
		}
	}
}

// TestGoPlaceholdersMatchCatalog scans the repository's Go sources: for every
// i18n.T("key", i18n.M{...}) call with a LITERAL key, the set of names in M{} must match the
// {Name} placeholders of that key's value in en.json. Catches name typos (otherwise
// substitution would silently leave {Name} in the output — the parity tests only compare locales to en).
func TestGoPlaceholdersMatchCatalog(t *testing.T) {
	loadOnce.Do(load)
	en := catalog[Default]
	root := repoRoot(t)

	callRe := regexp.MustCompile(`(?s)i18n\.T\(\s*"([^"]+)"\s*,\s*i18n\.M\{(.*?)\}\s*\)`)
	paramRe := regexp.MustCompile(`"(\w+)"\s*:`)
	phRe := regexp.MustCompile(`\{(\w+)\}`)

	for _, dir := range []string{"internal", "cmd"} {
		filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, m := range callRe.FindAllStringSubmatch(string(src), -1) {
				key, block := m[1], m[2]
				ev, ok := en[key]
				if !ok {
					t.Errorf("%s: i18n.T(%q) — key is absent from en.json", filepath.Base(path), key)
					continue
				}
				got := map[string]bool{}
				for _, p := range paramRe.FindAllStringSubmatch(block, -1) {
					got[p[1]] = true
				}
				want := map[string]bool{}
				for _, p := range phRe.FindAllStringSubmatch(ev, -1) {
					want[p[1]] = true
				}
				for p := range want {
					if !got[p] {
						t.Errorf("%s: i18n.T(%q) does not pass {%s} (present in en.json)", filepath.Base(path), key, p)
					}
				}
				for p := range got {
					if !want[p] {
						t.Errorf("%s: i18n.T(%q) passes an extra %q (absent from en.json)", filepath.Base(path), key, p)
					}
				}
			}
			return nil
		})
	}
}

// repoRoot walks up from the test's working directory to the directory containing go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found up the tree")
		}
		dir = parent
	}
}
