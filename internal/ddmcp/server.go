package ddmcp

import (
	"context"
	"os"
	"slices"

	"github.com/discodrive-cloud/discodrive-cli/internal/buildinfo"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// allToolSpecs is every tool the server knows about, before the Changing gate.
func allToolSpecs() []toolSpec {
	return slices.Concat(toolSpecs(), pimToolSpecs(), mediaToolSpecs(), bookmarkToolSpecs(), savedToolSpecs())
}

// buildServer creates the MCP server and registers the allowed tools on top of client c.
func buildServer(c *ddclient.Client, allowChanges bool) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "discodrive", Version: buildinfo.Version()}, nil)
	for _, sp := range selectTools(allToolSpecs(), allowChanges) {
		sp.register(s, c, &mcp.Tool{Name: sp.Name, Description: sp.Description})
	}
	return s
}

// Run builds a client from the resolved credentials (env override or config profile)
// and runs the stdio MCP server until ctx is cancelled or the transport closes.
func Run(ctx context.Context, allowChanges bool, profile string) error {
	p, err := resolveCreds(profile, os.Getenv, configLookup)
	if err != nil {
		return err
	}
	return buildServer(newClient(p), allowChanges).Run(ctx, &mcp.StdioTransport{})
}

// configLookup looks up a profile in the saved cli.json (a wrapper for resolveCreds).
func configLookup(profile string) (ddclient.Profile, bool) {
	path, err := ddclient.ConfigPath()
	if err != nil {
		return ddclient.Profile{}, false
	}
	cfg, err := ddclient.Load(path)
	if err != nil {
		return ddclient.Profile{}, false
	}
	return cfg.Lookup(profile)
}
