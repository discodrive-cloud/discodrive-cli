package ddmcp

import (
	"context"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultArticleChars caps one read: a longread in markdown easily runs past
// any context window, so the body is paged through offset/max_chars.
const defaultArticleChars = 40000

type pocketListInput struct {
	Status string `json:"status,omitempty" jsonschema:"filter by status: pending|processing|done|error (optional)"`
	Q      string `json:"q,omitempty" jsonschema:"substring of the title or the url (optional)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"page size, 1..500 (default 50)"`
	Offset int    `json:"offset,omitempty" jsonschema:"pagination offset (optional)"`
}

type pocketListOutput struct {
	Items []ddclient.SavedItem `json:"items"`
}

type pocketReadInput struct {
	ID       string `json:"id" jsonschema:"saved item id from dd_pocket_list"`
	Offset   int    `json:"offset,omitempty" jsonschema:"start reading from this character offset (for long articles)"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"max characters to return (default 40000)"`
}

type pocketReadOutput struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Offset     int    `json:"offset"`
	TotalChars int    `json:"total_chars"`
	Truncated  bool   `json:"truncated"`
}

type pocketAddInput struct {
	URL   string `json:"url" jsonschema:"page URL to save (max 2000 characters)"`
	Title string `json:"title,omitempty" jsonschema:"title override; ignored if this url is already saved"`
}

type savedIDInput struct {
	ID string `json:"id" jsonschema:"saved item id"`
}

type downloadAddInput struct {
	URL   string `json:"url" jsonschema:"file URL for the server to download (max 2000 characters)"`
	Title string `json:"title,omitempty" jsonschema:"optional label for the queue entry"`
}

type downloadIDInput struct {
	ID string `json:"id" jsonschema:"download id returned by dd_download_add"`
}

func savedToolSpecs() []toolSpec {
	return []toolSpec{
		{Name: "dd_pocket_list", Description: i18n.T("tool.dd_pocket_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in pocketListInput) (*mcp.CallToolResult, pocketListOutput, error) {
				items, err := c.ListSavedArticles(ctx, ddclient.SavedQuery{
					Status: in.Status, Q: in.Q, Limit: in.Limit, Offset: in.Offset,
				})
				if err != nil {
					return nil, pocketListOutput{}, err
				}
				if items == nil {
					items = []ddclient.SavedItem{}
				}
				return nil, pocketListOutput{Items: items}, nil
			})
		}},
		{Name: "dd_pocket_read", Description: i18n.T("tool.dd_pocket_read.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in pocketReadInput) (*mcp.CallToolResult, pocketReadOutput, error) {
				item, err := c.GetSavedItem(ctx, in.ID)
				if err != nil {
					return nil, pocketReadOutput{}, err
				}
				var sb strings.Builder
				if err := c.SavedContent(ctx, in.ID, &sb); err != nil {
					return nil, pocketReadOutput{}, err
				}
				runes := []rune(sb.String())
				limit := in.MaxChars
				if limit <= 0 {
					limit = defaultArticleChars
				}
				start := in.Offset
				if start < 0 || start > len(runes) {
					start = len(runes)
				}
				end := start + limit
				if end > len(runes) {
					end = len(runes)
				}
				return nil, pocketReadOutput{
					ID: item.ID, URL: item.URL, Title: item.Title,
					Content:    string(runes[start:end]),
					Offset:     start,
					TotalChars: len(runes),
					Truncated:  end < len(runes),
				}, nil
			})
		}},
		{Name: "dd_pocket_add", Description: i18n.T("tool.dd_pocket_add.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in pocketAddInput) (*mcp.CallToolResult, ddclient.SavedItem, error) {
				item, err := c.SaveArticle(ctx, in.URL, in.Title)
				if err != nil {
					return nil, ddclient.SavedItem{}, err
				}
				return nil, item, nil
			})
		}},
		{Name: "dd_pocket_retry", Description: i18n.T("tool.dd_pocket_retry.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in savedIDInput) (*mcp.CallToolResult, ddclient.SavedItem, error) {
				item, err := c.RetrySavedItem(ctx, in.ID)
				if err != nil {
					return nil, ddclient.SavedItem{}, err
				}
				return nil, item, nil
			})
		}},
		{Name: "dd_pocket_delete", Description: i18n.T("tool.dd_pocket_delete.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in savedIDInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteSavedItem(ctx, in.ID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_pocket_delete.msg.deleted", i18n.M{"ID": in.ID})}, nil
			})
		}},
		{Name: "dd_download_add", Description: i18n.T("tool.dd_download_add.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in downloadAddInput) (*mcp.CallToolResult, ddclient.SavedItem, error) {
				item, err := c.QueueDownload(ctx, in.URL, in.Title, "")
				if err != nil {
					return nil, ddclient.SavedItem{}, err
				}
				return nil, item, nil
			})
		}},
		{Name: "dd_download_status", Description: i18n.T("tool.dd_download_status.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in downloadIDInput) (*mcp.CallToolResult, ddclient.SavedItem, error) {
				item, err := c.GetSavedItem(ctx, in.ID)
				if err != nil {
					return nil, ddclient.SavedItem{}, err
				}
				return nil, item, nil
			})
		}},
		{Name: "dd_download_cancel", Description: i18n.T("tool.dd_download_cancel.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in downloadIDInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteSavedItem(ctx, in.ID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_download_cancel.msg.cancelled", i18n.M{"ID": in.ID})}, nil
			})
		}},
	}
}
