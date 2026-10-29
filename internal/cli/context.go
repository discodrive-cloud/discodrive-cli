package cli

import (
	"errors"
	"io"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// cmdCtx is the environment for a single command: global flags, output streams, and config path.
// Stdout and Stderr are terminal-safe in human mode (see safeWriter); RawStdout is the unfiltered
// stream for file contents (get -), nil when the context was built without one.
type cmdCtx struct {
	JSON       bool
	Profile    string
	Server     string
	Stdout     io.Writer
	Stderr     io.Writer
	RawStdout  io.Writer
	ConfigPath string
}

// contentOut is where verbatim file bytes go: the raw stream, never the filtered one.
func (cc *cmdCtx) contentOut() io.Writer {
	if cc.RawStdout != nil {
		return cc.RawStdout
	}
	return cc.Stdout
}

func normServer(u string) string { return strings.TrimRight(u, "/") }

// profile resolves the saved login the command runs as. --server never sends a login's
// device token to another address: it selects the saved login paired with that server
// (and must match the one named by --profile), so a token only goes where it was issued.
func (cc *cmdCtx) profile() (ddclient.Config, ddclient.Profile, error) {
	cfg, err := ddclient.Load(cc.ConfigPath)
	if err != nil {
		return cfg, ddclient.Profile{}, err
	}
	if cc.Server == "" {
		p, ok := cfg.Lookup(cc.Profile)
		if !ok {
			return cfg, p, errors.New(i18n.T("ui.no_profile"))
		}
		return cfg, p, nil
	}
	want := normServer(cc.Server)
	if cc.Profile != "" {
		p, ok := cfg.Lookup(cc.Profile)
		if !ok {
			return cfg, p, errors.New(i18n.T("ui.no_profile"))
		}
		if normServer(p.ServerURL) != want {
			return cfg, ddclient.Profile{}, errors.New(i18n.T("ui.server_mismatch",
				i18n.M{"Profile": p.Name, "Have": p.ServerURL, "Server": want}))
		}
		return cfg, p, nil
	}
	if p, ok := cfg.Lookup(""); ok && normServer(p.ServerURL) == want {
		return cfg, p, nil
	}
	for _, p := range cfg.Profiles {
		if normServer(p.ServerURL) == want {
			return cfg, p, nil
		}
	}
	return cfg, ddclient.Profile{}, errors.New(i18n.T("ui.no_server_profile", i18n.M{"Server": want}))
}

// client builds a ddclient.Client for the resolved login (see profile).
func (cc *cmdCtx) client() (*ddclient.Client, error) {
	_, p, err := cc.profile()
	if err != nil {
		return nil, err
	}
	return newClient(p.ServerURL, p), nil
}

// newClient builds a ddclient.Client honoring the profile's TLS mode, with the
// cross-process JWT cache (each CLI invocation is a fresh process).
func newClient(server string, p ddclient.Profile) *ddclient.Client {
	opts := []ddclient.Option{ddclient.WithJWTCache()}
	if p.Insecure {
		opts = append(opts, ddclient.WithInsecureTLS())
	}
	return ddclient.New(server, p.DeviceToken, opts...)
}
