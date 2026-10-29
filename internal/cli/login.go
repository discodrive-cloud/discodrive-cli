package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "login",
		Summary: "cmd.login.summary",
		Usage:   "cmd.login.usage",
		Long:    "cmd.login.long",
		Examples: []string{
			"cmd.login.example.0",
			"cmd.login.example.1",
		},
		Run: runLogin,
	})
}

func runLogin(cc *cmdCtx, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(cc.Stderr)
	server := fs.String("server", cc.Server, i18n.T("cmd.login.flag.server"))
	name := fs.String("name", defaultDeviceName(), i18n.T("cmd.login.flag.name"))
	profile := fs.String("profile", cc.Profile, i18n.T("cmd.login.flag.profile"))
	noClaude := fs.Bool("no-claude", false, i18n.T("cmd.login.flag.no_claude"))
	allowChanges := fs.Bool("allow-changes", false, i18n.T("cmd.login.flag.allow_changes"))
	insecure := fs.Bool("insecure", false, i18n.T("cmd.login.flag.insecure"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		return errors.New(i18n.T("cmd.login.msg.need_server"))
	}
	srv := strings.TrimRight(*server, "/")
	ctx := context.Background()
	if ddclient.Cleartext(srv) {
		fmt.Fprintln(cc.Stderr, i18n.T("ui.cleartext_server", i18n.M{"Server": srv}))
	}

	token, err := ddclient.PairInteractive(ctx, srv, *name, "desktop", *insecure, cc.Stdout)
	if err != nil {
		return err
	}

	profName := *profile
	if profName == "" {
		profName = "default"
	}
	cfg, err := ddclient.Load(cc.ConfigPath)
	if err != nil {
		return err
	}
	cfg.Upsert(ddclient.Profile{Name: profName, ServerURL: srv, DeviceToken: token, Insecure: *insecure}, true)
	if err := cfg.Save(cc.ConfigPath); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.login.msg.done", i18n.M{"Profile": profName}))

	if !*noClaude {
		changed, err := setupClaudeConfig(*allowChanges)
		if err != nil {
			fmt.Fprintf(cc.Stderr, "%s: %v\n", i18n.T("cmd.login.msg.claude_setup_failed"), err)
		} else if len(changed) > 0 {
			mode := i18n.T("cmd.login.msg.mode_readonly")
			if *allowChanges {
				mode = i18n.T("cmd.login.msg.mode_changes_allowed")
			}
			fmt.Fprint(cc.Stdout, i18n.T("cmd.login.msg.connected", i18n.M{"Mode": mode, "Changed": strings.Join(changed, ", ")}))
		}
	}
	return nil
}

func defaultDeviceName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "dd-cli"
}
