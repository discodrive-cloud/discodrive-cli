package ddclient

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Profile is a single saved DiscoDrive login (one server plus its device-token).
// Insecure disables TLS certificate verification for this server (set via login --insecure
// for self-signed certificates; every client built from the profile inherits it).
type Profile struct {
	Name        string `json:"name"`
	ServerURL   string `json:"server_url"`
	DeviceToken string `json:"device_token"`
	Insecure    bool   `json:"insecure,omitempty"`
}

// Config is the contents of cli.json: the set of profiles and the name of the default one.
type Config struct {
	Profiles []Profile `json:"profiles"`
	Default  string    `json:"default"`
}

// ConfigPath returns the path to cli.json, honoring $XDG_CONFIG_HOME (otherwise ~/.config).
func ConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "discodrive", "cli.json"), nil
}

// Load reads the config. A missing file is not an error (empty config).
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config (mkdir 0700, file 0600).
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Write a private replacement rather than retaining permissions of an existing
	// file. Secrets are never written into a world-readable destination, and a
	// failed write leaves the previous configuration intact.
	f, err := os.CreateTemp(filepath.Dir(path), ".cli-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Lookup returns the profile by name; an empty name means the default profile.
func (c Config) Lookup(name string) (Profile, bool) {
	if name == "" {
		name = c.Default
	}
	for _, p := range c.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Upsert replaces the profile with the same name or appends a new one.
func (c *Config) Upsert(p Profile, makeDefault bool) {
	for i := range c.Profiles {
		if c.Profiles[i].Name == p.Name {
			c.Profiles[i] = p
			if makeDefault {
				c.Default = p.Name
			}
			return
		}
	}
	c.Profiles = append(c.Profiles, p)
	if makeDefault || c.Default == "" {
		c.Default = p.Name
	}
}

// Remove deletes the profile by name and returns true if one was removed.
func (c *Config) Remove(name string) bool {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			if c.Default == name {
				c.Default = ""
			}
			return true
		}
	}
	return false
}
