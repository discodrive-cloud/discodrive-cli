package ddclient

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigPathHonorsXDG(t *testing.T) {
	base := filepath.Join(t.TempDir(), "xdg-test")
	t.Setenv("XDG_CONFIG_HOME", base)
	got, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "discodrive", "cli.json"); got != want {
		t.Fatalf("ConfigPath = %q, want %q", got, want)
	}
}

func TestConfigRoundTripAndPerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "discodrive", "cli.json")
	c := Config{}
	c.Upsert(Profile{Name: "home", ServerURL: "https://drive.example", DeviceToken: "kfd_a"}, true)
	c.Upsert(Profile{Name: "work", ServerURL: "https://work.example", DeviceToken: "kfd_b"}, false)
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v, want 0600", fi.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil || len(got.Profiles) != 2 || got.Default != "home" {
		t.Fatalf("load = %+v err=%v", got, err)
	}
	def, ok := got.Lookup("")
	if !ok || def.Name != "home" || def.DeviceToken != "kfd_a" {
		t.Fatalf("default lookup = %+v ok=%v", def, ok)
	}
	if _, ok := got.Lookup("work"); !ok {
		t.Fatal("work profile missing")
	}
}

func TestConfigRemoveDefault(t *testing.T) {
	c := Config{}
	c.Upsert(Profile{Name: "home", ServerURL: "u", DeviceToken: "t"}, true)
	if !c.Remove("home") {
		t.Fatal("Remove returned false")
	}
	if c.Default != "" || len(c.Profiles) != 0 {
		t.Fatalf("after remove: %+v", c)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got.Profiles) != 0 {
		t.Fatalf("expected empty config, got %+v", got)
	}
}

func TestConfigSaveTightensExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	cfg.Upsert(Profile{Name: "default", ServerURL: "https://drive.example", DeviceToken: "secret"}, true)
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o want=600", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil || len(got.Profiles) != 1 {
		t.Fatalf("config=%+v err=%v", got, err)
	}
}
