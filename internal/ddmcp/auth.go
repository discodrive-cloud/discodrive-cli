package ddmcp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/claudecfg"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// resolveCreds picks the credentials used to start the server: first the DISCODRIVE_SERVER +
// DISCODRIVE_TOKEN environment variables (if BOTH are set — an explicit override for headless
// setups and containers; DISCODRIVE_INSECURE_TLS=1 there disables certificate verification),
// otherwise the profile from the saved config, including its TLS mode.
// getenv and lookup are injected for testability.
func resolveCreds(profile string, getenv func(string) string, lookup func(string) (ddclient.Profile, bool)) (ddclient.Profile, error) {
	if s, tok := getenv("DISCODRIVE_SERVER"), getenv("DISCODRIVE_TOKEN"); s != "" && tok != "" {
		ins := getenv("DISCODRIVE_INSECURE_TLS")
		return ddclient.Profile{
			ServerURL:   strings.TrimRight(s, "/"),
			DeviceToken: tok,
			Insecure:    ins == "1" || ins == "true",
		}, nil
	}
	if p, ok := lookup(profile); ok {
		return p, nil
	}
	return ddclient.Profile{}, errors.New(i18n.T("mcp.err.no_credentials"))
}

// newClient builds a ddclient.Client honoring the profile's TLS mode, with the
// cross-process JWT cache (shared with dd-cli runs against the same server).
func newClient(p ddclient.Profile) *ddclient.Client {
	opts := []ddclient.Option{ddclient.WithJWTCache()}
	if p.Insecure {
		opts = append(opts, ddclient.WithInsecureTLS())
	}
	return ddclient.New(p.ServerURL, p.DeviceToken, opts...)
}

// Login performs device pairing directly from dd-mcp and saves the profile to the shared
// cli.json, so the MCP server can be used without installing dd-cli.
func Login(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dd-mcp login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "", i18n.T("mcp.login.flag.server"))
	name := fs.String("name", "dd-mcp", i18n.T("mcp.login.flag.name"))
	profile := fs.String("profile", "default", i18n.T("mcp.login.flag.profile"))
	noClaude := fs.Bool("no-claude", false, i18n.T("mcp.login.flag.no_claude"))
	allowChanges := fs.Bool("allow-changes", false, i18n.T("mcp.login.flag.allow_changes"))
	insecure := fs.Bool("insecure", false, i18n.T("mcp.login.flag.insecure"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		return errors.New(i18n.T("mcp.login.err.need_server"))
	}
	srv := strings.TrimRight(*server, "/")
	if ddclient.Cleartext(srv) {
		fmt.Fprintln(stderr, i18n.T("ui.cleartext_server", i18n.M{"Server": srv}))
	}

	token, err := ddclient.PairInteractive(ctx, srv, *name, "mcp", *insecure, stdout)
	if err != nil {
		return err
	}

	path, err := ddclient.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := ddclient.Load(path)
	if err != nil {
		return err
	}
	prof := *profile
	if prof == "" {
		prof = "default"
	}
	cfg.Upsert(ddclient.Profile{Name: prof, ServerURL: srv, DeviceToken: token, Insecure: *insecure}, true)
	if err := cfg.Save(path); err != nil {
		return err
	}
	fmt.Fprint(stdout, i18n.T("mcp.login.msg.done", i18n.M{"Profile": prof}))

	if !*noClaude {
		if exe, err := os.Executable(); err != nil {
			fmt.Fprint(stderr, i18n.T("mcp.login.msg.claude_path_failed", i18n.M{"Err": err}))
		} else if changed, err := claudecfg.Setup(claudecfg.StablePath(exe), *allowChanges); err != nil {
			fmt.Fprint(stderr, i18n.T("mcp.login.msg.claude_setup_failed", i18n.M{"Err": err}))
		} else if len(changed) > 0 {
			fmt.Fprint(stdout, i18n.T("mcp.login.msg.claude_connected", i18n.M{"Changed": strings.Join(changed, ", ")}))
		}
	}
	return nil
}

// SetupClaude (re)registers dd-mcp in the Claude config files and toggles the changing-tools
// mode, without device pairing. For users who only have dd-mcp installed: a convenient
// --allow-changes switch that avoids editing JSON by hand.
func SetupClaude(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dd-mcp setup-claude", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allowChanges := fs.Bool("allow-changes", false, i18n.T("mcp.setup_claude.flag.allow_changes"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("mcp.err.claude_exe_path"), err)
	}
	changed, err := claudecfg.Setup(claudecfg.StablePath(exe), *allowChanges)
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		fmt.Fprintln(stdout, i18n.T("mcp.setup_claude.msg.none_found"))
		return nil
	}
	mode := i18n.T("mcp.setup_claude.msg.mode_readonly")
	if *allowChanges {
		mode = i18n.T("mcp.setup_claude.msg.mode_changes")
	}
	for _, p := range changed {
		fmt.Fprint(stdout, i18n.T("mcp.setup_claude.msg.updated", i18n.M{"Path": p}))
	}
	fmt.Fprint(stdout, i18n.T("mcp.setup_claude.msg.mode_line", i18n.M{"Mode": mode}))
	return nil
}

// PrintConfig prints a ready-to-use dd-mcp registration snippet for any MCP client: stdio
// (Claude/Cursor/Zed) or, with --http, a variant for HTTP clients (ChatGPT connectors, remote
// access). It never writes to anyone's config files — it only prints for manual pasting.
func PrintConfig(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dd-mcp print-config", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allow := fs.Bool("allow-changes", false, i18n.T("mcp.print_config.flag.allow_changes"))
	httpAddr := fs.String("http", "", i18n.T("mcp.print_config.flag.http"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("mcp.err.claude_exe_path"), err)
	}

	if *httpAddr != "" {
		runCmd := claudecfg.StablePath(exe) + " --http " + *httpAddr
		if *allow {
			runCmd += " --allow-changes"
		}
		fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.http_header"))
		fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.http_step1", i18n.M{"Cmd": runCmd}))
		fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.http_step2", i18n.M{"Addr": *httpAddr}))
		fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.http_creds"))
		return nil
	}

	snip, err := claudecfg.StdioSnippet(claudecfg.StablePath(exe), *allow)
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.stdio_header", i18n.M{"Snippet": snip}))
	fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.stdio_creds1"))
	fmt.Fprintf(stdout, "  \"env\": { \"DISCODRIVE_SERVER\": \"...\", \"DISCODRIVE_TOKEN\": \"...\" }\n")
	fmt.Fprint(stdout, i18n.T("mcp.print_config.msg.stdio_footer"))
	return nil
}

// Logout unpairs the device: it revokes the current device on the server and removes the
// matching profile from cli.json. --profile explicitly selects a saved login;
// otherwise the same environment override as Run is used. It also works in env-only mode (DISCODRIVE_SERVER/DISCODRIVE_TOKEN
// without a saved profile) — the device is then revoked using the env credentials, and there
// is nothing to forget locally. With --local-only the server-side revocation is skipped (only
// the local profile is cleaned up).
func Logout(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dd-mcp logout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	profile := fs.String("profile", "", i18n.T("mcp.logout.flag.profile"))
	localOnly := fs.Bool("local-only", false, i18n.T("mcp.logout.flag.local_only"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := ddclient.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := ddclient.Load(path)
	if err != nil {
		return err
	}
	name := *profile
	if name == "" {
		name = cfg.Default
	}

	// An explicit profile is a deliberate logout target. Otherwise match Run's
	// credential precedence, and never remove an unrelated saved login.
	saved, hasProfile := cfg.Lookup(name)
	creds := saved
	if *profile == "" {
		creds, err = resolveCreds("", os.Getenv, cfg.Lookup)
		if err != nil {
			return err
		}
		hasProfile = hasProfile && saved.DeviceToken == creds.DeviceToken &&
			strings.TrimRight(saved.ServerURL, "/") == strings.TrimRight(creds.ServerURL, "/")
	}

	if creds.DeviceToken == "" {
		return errors.New(i18n.T("mcp.logout.err.nothing_to_forget", i18n.M{"Name": name}))
	}

	if !*localOnly {
		c := newClient(creds)
		if err := c.Unpair(ctx); err != nil {
			fmt.Fprint(stderr, i18n.T("mcp.logout.msg.revoke_failed", i18n.M{"Err": err}))
		} else {
			fmt.Fprintln(stdout, i18n.T("mcp.logout.msg.revoked"))
		}
	}

	if hasProfile {
		cfg.Remove(name)
		if err := cfg.Save(path); err != nil {
			return err
		}
		fmt.Fprint(stdout, i18n.T("mcp.logout.msg.forgotten", i18n.M{"Name": name}))
	} else {
		fmt.Fprintln(stdout, i18n.T("mcp.logout.msg.env_only_nothing"))
	}
	return nil
}
