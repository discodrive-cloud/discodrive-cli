package ddmcp

import (
	"context"
	"os"
	"slices"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/buildinfo"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverOptions is what shapes the tool set and the instructions of one server.
type serverOptions struct {
	// AllowChanges enables the Changing tools (--allow-changes).
	AllowChanges bool
	// LocalFiles: the MCP client runs on the same machine as dd-mcp (stdio), so tools
	// may read and write local files by path. Never set over HTTP: there the client may
	// be elsewhere, and the files of the server's host are not its to touch.
	LocalFiles bool
}

// allToolSpecs is every tool the server knows about, before the gates.
func allToolSpecs(o serverOptions) []toolSpec {
	return slices.Concat(toolSpecs(o), pimToolSpecs(), mediaToolSpecs(), bookmarkToolSpecs(), savedToolSpecs())
}

// buildServer creates the MCP server and registers the allowed tools on top of client c.
func buildServer(c *ddclient.Client, o serverOptions) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "discodrive", Version: buildinfo.Version()},
		&mcp.ServerOptions{Instructions: serverInstructions(o)})
	for _, sp := range selectTools(allToolSpecs(o), o) {
		t := &mcp.Tool{Name: sp.Name, Description: sp.Description}
		if sp.InputSchema != nil {
			t.InputSchema = sp.InputSchema(o)
		}
		sp.register(s, c, t)
	}
	return s
}

// serverInstructions is the short capability map the client puts into the model's
// context at initialize. It exists so that an assistant with nothing but this server
// finds the way to a task — or tells the user how to enable it — instead of deciding
// that it cannot be done.
func serverInstructions(o serverOptions) string {
	parts := []string{i18n.T("mcp.instructions.intro")}
	if !o.AllowChanges {
		parts = append(parts, i18n.T("mcp.instructions.changes_off"))
	}
	if o.AllowChanges && o.LocalFiles {
		parts = append(parts, i18n.T("mcp.instructions.local_files"))
	}
	parts = append(parts, i18n.T("mcp.instructions.cli"), i18n.T("mcp.instructions.read_limit"))
	return strings.Join(parts, "\n\n")
}

// Run builds a client from the resolved credentials (env override or config profile)
// and runs the stdio MCP server until ctx is cancelled or the transport closes.
func Run(ctx context.Context, allowChanges bool, profile string) error {
	p, err := resolveCreds(profile, os.Getenv, configLookup)
	if err != nil {
		return err
	}
	o := serverOptions{AllowChanges: allowChanges, LocalFiles: true}
	return buildServer(newClient(p), o).Run(ctx, &mcp.StdioTransport{})
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
