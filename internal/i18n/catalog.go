package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// M holds named substitutions for {Name} placeholders.
type M map[string]any

var (
	mu       sync.RWMutex
	active   = Default
	catalog  map[Lang]map[string]string
	loadOnce sync.Once
)

func load() {
	catalog = map[Lang]map[string]string{}
	for _, l := range Supported {
		b, err := localeFS.ReadFile("locales/" + string(l) + ".json")
		if err != nil {
			continue
		}
		var nested map[string]any
		if json.Unmarshal(b, &nested) != nil {
			continue
		}
		flat := map[string]string{}
		flatten("", nested, flat)
		catalog[l] = flat
	}
}

// flatten expands nested JSON into flat dotted keys: {"a":{"b":"x"}} → {"a.b":"x"}.
func flatten(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case string:
			out[key] = t
		case map[string]any:
			flatten(key, t, out)
		}
	}
}

// SetActive sets the active language (usually once at startup after Resolve).
func SetActive(l Lang) { mu.Lock(); active = l; mu.Unlock() }

// Active returns the current active language.
func Active() Lang { mu.RLock(); defer mu.RUnlock(); return active }

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

// T returns the localized string for a key: active language → en → the key itself.
// If args are passed, {Name} placeholders are substituted from them.
func T(key string, args ...M) string {
	loadOnce.Do(load)
	mu.RLock()
	a := active
	mu.RUnlock()

	s, ok := lookup(a, key)
	if !ok {
		s, ok = lookup(Default, key)
	}
	if !ok {
		return key
	}
	if len(args) == 0 || len(args[0]) == 0 {
		return s
	}
	return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := args[0][name]; ok {
			return fmt.Sprint(v)
		}
		return m
	})
}

func lookup(l Lang, key string) (string, bool) {
	if m, ok := catalog[l]; ok {
		if s, ok := m[key]; ok && s != "" {
			return s, true
		}
	}
	return "", false
}
