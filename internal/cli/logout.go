package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:     "logout",
		Summary:  "cmd.logout.summary",
		Usage:    "cmd.logout.usage",
		Long:     "cmd.logout.long",
		Examples: []string{"cmd.logout.example.0", "cmd.logout.example.1", "cmd.logout.example.2"},
		Run:      runLogout,
	})
}

func runLogout(cc *cmdCtx, args []string) error {
	fs := newFlagSet("logout", cc)
	localOnly := fs.Bool("local-only", false, i18n.T("cmd.logout.flag.local_only"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := ddclient.Load(cc.ConfigPath)
	if err != nil {
		return err
	}
	name := cc.Profile
	if name == "" {
		name = cfg.Default
	}
	p, ok := cfg.Lookup(name)
	if !ok {
		return errors.New(i18n.T("cmd.logout.msg.no_profile", i18n.M{"Name": name}))
	}
	if !*localOnly {
		c := newClient(p.ServerURL, p)
		if err := c.Unpair(context.Background()); err != nil {
			fmt.Fprintf(cc.Stderr, "%s: %v\n%s\n", i18n.T("cmd.logout.msg.revoke_failed"), err, i18n.T("cmd.logout.msg.revoke_manual_hint"))
		} else {
			fmt.Fprintln(cc.Stdout, i18n.T("cmd.logout.msg.revoked"))
		}
	}
	cfg.Remove(name)
	if err := cfg.Save(cc.ConfigPath); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.logout.msg.forgotten", i18n.M{"Name": name}))
	return nil
}
