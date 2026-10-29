// Command dd-mcp is the DiscoDrive MCP server (stdio).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/discodrive-cloud/discodrive-cli/internal/buildinfo"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddmcp"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func main() {
	// Language from the environment (DISCODRIVE_LANG/LC_ALL/LANG) — for subcommands and as a base;
	// for the server run it is overridden by the --lang flag below.
	i18n.SetActive(i18n.Resolve("", os.Getenv))

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("dd-mcp %s\n", buildinfo.Version())
		return
	}

	// Subcommands: dd-mcp is self-contained and does not require dd-cli to be installed.
	if len(os.Args) > 1 && (os.Args[1] == "login" || os.Args[1] == "logout" || os.Args[1] == "setup-claude" || os.Args[1] == "print-config") {
		var err error
		switch os.Args[1] {
		case "login":
			err = ddmcp.Login(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		case "logout":
			err = ddmcp.Logout(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		case "setup-claude":
			err = ddmcp.SetupClaude(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		case "print-config":
			err = ddmcp.PrintConfig(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		}
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return // help has already been printed by the FlagSet
			}
			fmt.Fprintf(os.Stderr, "dd-mcp: %v\n", err)
			os.Exit(1)
		}
		return
	}

	allowChanges := flag.Bool("allow-changes", false, i18n.T("mcp.main.flag.allow_changes"))
	profile := flag.String("profile", "", i18n.T("mcp.main.flag.profile"))
	httpAddr := flag.String("http", "", i18n.T("mcp.main.flag.http"))
	httpToken := flag.String("http-token", "", i18n.T("mcp.main.flag.http_token"))
	lang := flag.String("lang", "", i18n.T("mcp.main.flag.lang"))
	flag.Parse()

	// The --lang flag overrides the environment language for the server output.
	i18n.SetActive(i18n.Resolve(*lang, os.Getenv))

	var err error
	if *httpAddr != "" {
		token := *httpToken
		if token == "" {
			token = os.Getenv("DISCODRIVE_HTTP_TOKEN")
		}
		err = ddmcp.RunHTTP(context.Background(), *allowChanges, *profile, *httpAddr, token)
	} else {
		err = ddmcp.Run(context.Background(), *allowChanges, *profile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "dd-mcp: %v\n", err)
		os.Exit(1)
	}
}
