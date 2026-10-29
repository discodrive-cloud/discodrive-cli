package ddmcp

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolSpec describes a single MCP tool and how to register it on the server.
type toolSpec struct {
	Name        string
	Description string
	// Changing: only under --allow-changes. Set for operations that change existing
	// data, and for those that make the server fetch a caller-supplied URL: in the
	// default mode such a tool would let injected text read by the assistant send
	// private data out through the URL.
	Changing bool
	register func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool)
}

// selectTools keeps only the allowed tools: changing ones only when allowChanges is set.
func selectTools(specs []toolSpec, allowChanges bool) []toolSpec {
	out := make([]toolSpec, 0, len(specs))
	for _, sp := range specs {
		if sp.Changing && !allowChanges {
			continue
		}
		out = append(out, sp)
	}
	return out
}

// --- tool input/output structs ---

type listInput struct {
	Path string `json:"path" jsonschema:"folder to list (slash-path; empty = root)"`
}
type nodeOut struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    *int64 `json:"size"`
	Version int64  `json:"version"`
	// ContentHash is the sha256 (hex) of the stored bytes; absent for a folder.
	ContentHash string `json:"content_hash,omitempty"`
}
type listOutput struct {
	Nodes []nodeOut `json:"nodes"`
}

type readInput struct {
	Path string `json:"path" jsonschema:"path to the file"`
}
type readOutput struct {
	Path    string `json:"path"`
	Content string `json:"content" jsonschema:"file contents as text"`
}

type searchInput struct {
	Query string `json:"query" jsonschema:"name substring to search for"`
	Root  string `json:"root" jsonschema:"search root (default: entire storage)"`
}
type searchHitOut struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	ID    string `json:"id"`
}
type searchOutput struct {
	Hits []searchHitOut `json:"hits"`
}

type uploadInput struct {
	Path    string `json:"path" jsonschema:"destination path (folder/name)"`
	Content string `json:"content" jsonschema:"file contents as text"`
}
type mkdirInput struct {
	Path string `json:"path" jsonschema:"path of the new folder"`
}
type pathInput struct {
	Path string `json:"path" jsonschema:"path to the node"`
}
type moveInput struct {
	Src string `json:"src" jsonschema:"source path"`
	Dst string `json:"dst" jsonschema:"destination path (folder)"`
}
type renameInput struct {
	Path string `json:"path" jsonschema:"path to the node"`
	Name string `json:"name" jsonschema:"new name"`
}
type okOutput struct {
	OK   bool   `json:"ok"`
	Note string `json:"note"`
}

func toNodeOut(n ddclient.Node) nodeOut {
	return nodeOut{ID: n.ID, Name: n.Name, IsDir: n.IsDir, Size: n.Size, Version: n.Version, ContentHash: n.ContentHash}
}

func splitDirBase(p string) (string, string) {
	p = strings.Trim(p, "/")
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "", p
	}
	return p[:i], p[i+1:]
}

// toolSpecs returns the full set of file-domain tools.
func toolSpecs() []toolSpec {
	return []toolSpec{
		{Name: "dd_files_list", Description: i18n.T("tool.dd_files_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, listOutput, error) {
					node, err := c.Resolve(ctx, in.Path)
					if err != nil {
						return nil, listOutput{}, err
					}
					kids, err := c.ListChildren(ctx, node.ID)
					if err != nil {
						return nil, listOutput{}, err
					}
					out := listOutput{Nodes: make([]nodeOut, 0, len(kids))}
					for _, k := range kids {
						out.Nodes = append(out.Nodes, toNodeOut(k))
					}
					return nil, out, nil
				})
		}},
		{Name: "dd_files_read", Description: i18n.T("tool.dd_files_read.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, readOutput, error) {
					node, err := c.Resolve(ctx, in.Path)
					if err != nil {
						return nil, readOutput{}, err
					}
					var sb strings.Builder
					w := &boundedTextWriter{dst: &sb, remaining: maxFileReadBytes}
					if err := c.Download(ctx, node.ID, w); err != nil {
						return nil, readOutput{}, err
					}
					return nil, readOutput{Path: in.Path, Content: sb.String()}, nil
				})
		}},
		{Name: "dd_files_search", Description: i18n.T("tool.dd_files_search.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
					root := in.Root
					if root == "" {
						root = "/"
					}
					hits, err := c.Search(ctx, root, in.Query)
					if err != nil {
						return nil, searchOutput{}, err
					}
					out := searchOutput{Hits: make([]searchHitOut, 0, len(hits))}
					for _, h := range hits {
						out.Hits = append(out.Hits, searchHitOut{Path: h.Path, IsDir: h.Node.IsDir, ID: h.Node.ID})
					}
					return nil, out, nil
				})
		}},
		{Name: "dd_files_upload", Changing: true, Description: i18n.T("tool.dd_files_upload.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in uploadInput) (*mcp.CallToolResult, nodeOut, error) {
					dir, base := splitDirBase(in.Path)
					parent, err := c.Resolve(ctx, dir)
					if err != nil {
						return nil, nodeOut{}, err
					}
					// Content arrives inline as text, so there is no source file whose date to
					// carry: the server dates it on arrival.
					node, err := c.Upload(ctx, parent.ID, base, strings.NewReader(in.Content), time.Time{})
					if err != nil {
						return nil, nodeOut{}, err
					}
					return nil, toNodeOut(node), nil
				})
		}},
		{Name: "dd_files_mkdir", Description: i18n.T("tool.dd_files_mkdir.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in mkdirInput) (*mcp.CallToolResult, nodeOut, error) {
					dir, base := splitDirBase(in.Path)
					parent, err := c.Resolve(ctx, dir)
					if err != nil {
						return nil, nodeOut{}, err
					}
					node, err := c.CreateFolder(ctx, parent.ID, base)
					if err != nil {
						return nil, nodeOut{}, err
					}
					return nil, toNodeOut(node), nil
				})
		}},
		{Name: "dd_files_delete", Changing: true, Description: i18n.T("tool.dd_files_delete.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, okOutput, error) {
					node, err := c.Resolve(ctx, in.Path)
					if err != nil {
						return nil, okOutput{}, err
					}
					if node.ID == "" {
						return nil, okOutput{}, errors.New(i18n.T("tool.dd_files_delete.msg.cannot_delete_root"))
					}
					if err := c.Delete(ctx, node.ID); err != nil {
						return nil, okOutput{}, err
					}
					return nil, okOutput{OK: true, Note: i18n.T("tool.dd_files_delete.msg.deleted", i18n.M{"Path": in.Path})}, nil
				})
		}},
		{Name: "dd_files_move", Changing: true, Description: i18n.T("tool.dd_files_move.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in moveInput) (*mcp.CallToolResult, okOutput, error) {
					src, err := c.Resolve(ctx, in.Src)
					if err != nil {
						return nil, okOutput{}, err
					}
					dst, err := c.Resolve(ctx, in.Dst)
					if err != nil {
						return nil, okOutput{}, err
					}
					if _, err := c.Move(ctx, src.ID, dst.ID); err != nil {
						return nil, okOutput{}, err
					}
					return nil, okOutput{OK: true, Note: i18n.T("tool.dd_files_move.msg.moved")}, nil
				})
		}},
		{Name: "dd_files_rename", Changing: true, Description: i18n.T("tool.dd_files_rename.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t,
				func(ctx context.Context, _ *mcp.CallToolRequest, in renameInput) (*mcp.CallToolResult, okOutput, error) {
					node, err := c.Resolve(ctx, in.Path)
					if err != nil {
						return nil, okOutput{}, err
					}
					if _, err := c.Rename(ctx, node.ID, in.Name); err != nil {
						return nil, okOutput{}, err
					}
					return nil, okOutput{OK: true, Note: i18n.T("tool.dd_files_rename.msg.renamed", i18n.M{"Name": in.Name})}, nil
				})
		}},
	}
}

// Bound actual bytes, not the listing's potentially stale size. Reject oversize
// content instead of returning truncated text as if it were a complete file.
const maxFileReadBytes = 1 << 20

type boundedTextWriter struct {
	dst       io.Writer
	remaining int
}

func (w *boundedTextWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, errors.New(i18n.T("err.file_read_too_large"))
	}
	n, err := w.dst.Write(p)
	w.remaining -= n
	return n, err
}
