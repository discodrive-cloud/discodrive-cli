package ddmcp

import (
	"context"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type musicListInput struct {
	Subpath string `json:"subpath" jsonschema:"optional subpath inside the music folder (e.g. \"Cosmic Gate\")"`
}
type musicListOutput struct {
	Entries []ddclient.Node `json:"entries"`
}
type radioListOutput struct {
	Radio []ddclient.Radio `json:"radio"`
}
type podcastsListOutput struct {
	Podcasts []ddclient.Podcast `json:"podcasts"`
}
type radioAddInput struct {
	Name        string `json:"name" jsonschema:"station name"`
	StreamURL   string `json:"stream_url" jsonschema:"stream URL"`
	HomepageURL string `json:"homepage_url" jsonschema:"homepage (optional)"`
}
type podcastAddInput struct {
	URL string `json:"url" jsonschema:"podcast RSS feed URL"`
}
type booksListInput struct {
	Q      string `json:"q" jsonschema:"full-text search (optional)"`
	Author string `json:"author" jsonschema:"filter by author (optional)"`
	Series string `json:"series" jsonschema:"filter by series (optional)"`
	Genre  string `json:"genre" jsonschema:"filter by genre (optional)"`
	Start  int    `json:"start" jsonschema:"pagination offset; page = 50 books"`
}
type booksListOutput struct {
	Books []ddclient.Book `json:"books"`
	Total int64           `json:"total"`
}
type facetsOutput struct {
	Authors []string `json:"authors"`
	Series  []string `json:"series"`
	Genres  []string `json:"genres"`
}
type bookMetaInput struct {
	NodeID string `json:"node_id" jsonschema:"book node-id (the nodeId field from the list)"`
}
type bookMetaUpdateInput struct {
	NodeID      string   `json:"node_id" jsonschema:"book node-id (the nodeId field from the list)"`
	Title       string   `json:"title" jsonschema:"title"`
	Authors     []string `json:"authors" jsonschema:"authors"`
	Tags        []string `json:"tags" jsonschema:"tags"`
	Series      string   `json:"series" jsonschema:"series"`
	SeriesIndex float64  `json:"series_index" jsonschema:"number within the series"`
	Language    string   `json:"language" jsonschema:"language"`
	Description string   `json:"description" jsonschema:"description"`
	Publisher   string   `json:"publisher" jsonschema:"publisher"`
	Date        string   `json:"date" jsonschema:"date"`
}
type bookMetaOutput struct {
	Meta ddclient.BookMeta `json:"meta"`
}

// mediaToolSpecs returns the tools for the music and books domains.
func mediaToolSpecs() []toolSpec {
	return []toolSpec{
		{Name: "dd_music_list", Description: i18n.T("tool.dd_music_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in musicListInput) (*mcp.CallToolResult, musicListOutput, error) {
				entries, err := c.ListMusicFolder(ctx, in.Subpath)
				if err != nil {
					return nil, musicListOutput{}, err
				}
				return nil, musicListOutput{Entries: entries}, nil
			})
		}},
		{Name: "dd_music_radio_list", Description: i18n.T("tool.dd_music_radio_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, radioListOutput, error) {
				items, err := c.ListRadio(ctx)
				if err != nil {
					return nil, radioListOutput{}, err
				}
				return nil, radioListOutput{Radio: items}, nil
			})
		}},
		{Name: "dd_music_podcasts_list", Description: i18n.T("tool.dd_music_podcasts_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, podcastsListOutput, error) {
				items, err := c.ListPodcasts(ctx)
				if err != nil {
					return nil, podcastsListOutput{}, err
				}
				return nil, podcastsListOutput{Podcasts: items}, nil
			})
		}},
		{Name: "dd_music_radio_add", Description: i18n.T("tool.dd_music_radio_add.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in radioAddInput) (*mcp.CallToolResult, idOutput, error) {
				r, err := c.CreateRadio(ctx, ddclient.Radio{Name: in.Name, StreamURL: in.StreamURL, HomepageURL: in.HomepageURL})
				if err != nil {
					return nil, idOutput{}, err
				}
				return nil, idOutput{ID: r.ID}, nil
			})
		}},
		{Name: "dd_music_podcast_add", Description: i18n.T("tool.dd_music_podcast_add.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in podcastAddInput) (*mcp.CallToolResult, idOutput, error) {
				p, err := c.AddPodcast(ctx, in.URL)
				if err != nil {
					return nil, idOutput{}, err
				}
				return nil, idOutput{ID: p.ID}, nil
			})
		}},
		{Name: "dd_books_list", Description: i18n.T("tool.dd_books_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in booksListInput) (*mcp.CallToolResult, booksListOutput, error) {
				list, err := c.ListBooks(ctx, ddclient.BookQuery{Q: in.Q, Author: in.Author, Series: in.Series, Genre: in.Genre, Start: in.Start})
				if err != nil {
					return nil, booksListOutput{}, err
				}
				return nil, booksListOutput{Books: list.Books, Total: list.Total}, nil
			})
		}},
		{Name: "dd_books_facets", Description: i18n.T("tool.dd_books_facets.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, facetsOutput, error) {
				f, err := c.BookFacets(ctx)
				if err != nil {
					return nil, facetsOutput{}, err
				}
				return nil, facetsOutput{Authors: f.Authors, Series: f.Series, Genres: f.Genres}, nil
			})
		}},
		{Name: "dd_books_meta", Description: i18n.T("tool.dd_books_meta.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookMetaInput) (*mcp.CallToolResult, bookMetaOutput, error) {
				m, err := c.GetBookMeta(ctx, in.NodeID)
				if err != nil {
					return nil, bookMetaOutput{}, err
				}
				return nil, bookMetaOutput{Meta: m}, nil
			})
		}},
		{Name: "dd_music_radio_delete", Changing: true, Description: i18n.T("tool.dd_music_radio_delete.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in uidInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteRadio(ctx, in.UID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_music_radio_delete.msg.deleted", i18n.M{"UID": in.UID})}, nil
			})
		}},
		{Name: "dd_music_podcast_delete", Changing: true, Description: i18n.T("tool.dd_music_podcast_delete.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in uidInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeletePodcast(ctx, in.UID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_music_podcast_delete.msg.unsubscribed", i18n.M{"UID": in.UID})}, nil
			})
		}},
		{Name: "dd_books_meta_update", Changing: true, Description: i18n.T("tool.dd_books_meta_update.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookMetaUpdateInput) (*mcp.CallToolResult, okOutput, error) {
				err := c.UpdateBookMeta(ctx, in.NodeID, ddclient.BookMeta{
					Title: in.Title, Authors: in.Authors, Tags: in.Tags, Series: in.Series,
					SeriesIndex: in.SeriesIndex, Language: in.Language, Description: in.Description,
					Publisher: in.Publisher, Date: in.Date,
				})
				if err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_books_meta_update.msg.updated", i18n.M{"NodeID": in.NodeID})}, nil
			})
		}},
	}
}
