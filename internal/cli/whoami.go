package cli

import (
	"context"
	"fmt"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:     "whoami",
		Summary:  "cmd.whoami.summary",
		Usage:    "cmd.whoami.usage",
		Long:     "cmd.whoami.long",
		Examples: []string{"cmd.whoami.example.0", "cmd.whoami.example.1"},
		Run:      runWhoami,
	})
}

func runWhoami(cc *cmdCtx, args []string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	u, err := c.Me(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, u)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.whoami.msg.line", i18n.M{"Email": u.Email, "Role": u.Role, "ID": u.ID}))
	return nil
}
