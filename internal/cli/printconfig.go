package cli

import (
	"fmt"

	"github.com/discodrive-cloud/discodrive-cli/internal/claudecfg"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "print-config",
		Summary: "cmd.print_config.summary",
		Usage:   "cmd.print_config.usage",
		Long:    "cmd.print_config.long",
		Examples: []string{
			"cmd.print_config.example.0",
			"cmd.print_config.example.1",
			"cmd.print_config.example.2",
		},
		Run: func(cc *cmdCtx, args []string) error {
			fs := newFlagSet("print-config", cc)
			allow := fs.Bool("allow-changes", false, i18n.T("cmd.print_config.flag.allow_changes"))
			httpAddr := fs.String("http", "", i18n.T("cmd.print_config.flag.http"))
			if err := fs.Parse(args); err != nil {
				return err
			}
			binPath, err := ddmcpPath()
			if err != nil {
				return err
			}

			if *httpAddr != "" {
				runCmd := binPath + " --http " + *httpAddr
				if *allow {
					runCmd += " --allow-changes"
				}
				fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.http_header"))
				fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.http_step1", i18n.M{"Cmd": runCmd}))
				fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.http_step2", i18n.M{"Addr": *httpAddr}))
				fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.http_creds"))
				return nil
			}

			snip, err := claudecfg.StdioSnippet(binPath, *allow)
			if err != nil {
				return err
			}
			fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.stdio_header", i18n.M{"Snippet": snip}))
			fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.stdio_creds1"))
			fmt.Fprintf(cc.Stdout, "  \"env\": { \"DISCODRIVE_SERVER\": \"...\", \"DISCODRIVE_TOKEN\": \"...\" }\n")
			fmt.Fprint(cc.Stdout, i18n.T("cmd.print_config.msg.stdio_footer"))
			return nil
		},
	})
}
