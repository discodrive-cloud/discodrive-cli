package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "books",
		Summary: "cmd.books.summary",
		Usage:   "cmd.books.usage",
		Long:    "cmd.books.long",
		Examples: []string{
			"cmd.books.example.0",
			"cmd.books.example.1",
			"cmd.books.example.2",
			"cmd.books.example.3",
		},
		Run: runBooks,
	})
}

func runBooks(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "list":
		return booksList(cc, rest)
	case "facets":
		return booksFacets(cc)
	case "get":
		return booksGet(cc, rest)
	case "meta":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.books.msg.need_node_id"))
		}
		return booksMeta(cc, rest[0])
	case "scan":
		return booksScan(cc)
	case "info":
		return booksInfo(cc)
	default:
		return errors.New(i18n.T("cmd.books.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func booksList(cc *cmdCtx, args []string) error {
	fs := newFlagSet("books list", cc)
	q := fs.String("q", "", i18n.T("cmd.books.flag.q"))
	author := fs.String("author", "", i18n.T("cmd.books.flag.author"))
	series := fs.String("series", "", i18n.T("cmd.books.flag.series"))
	genre := fs.String("genre", "", i18n.T("cmd.books.flag.genre"))
	start := fs.Int("start", 0, i18n.T("cmd.books.flag.start"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	list, err := c.ListBooks(context.Background(), ddclient.BookQuery{
		Q: *q, Author: *author, Series: *series, Genre: *genre, Start: *start,
	})
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, list)
	}
	for _, b := range list.Books {
		fmt.Fprintf(cc.Stdout, "%-24s %-24s %-30s [%s]\n", b.ID, b.NodeID, b.Title, b.Format)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.books.msg.shown", i18n.M{"Count": len(list.Books), "Total": list.Total}))
	return nil
}

func booksFacets(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	f, err := c.BookFacets(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, f)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.books.msg.facets", i18n.M{"Authors": f.Authors, "Series": f.Series, "Genres": f.Genres}))
	return nil
}

func booksGet(cc *cmdCtx, args []string) error {
	fs := newFlagSet("books get", cc)
	out := fs.String("out", "", i18n.T("cmd.books.flag.out"))
	// flag.Parse stops at the first non-flag, so a positional <bookID> could
	// "swallow" flags that follow it. Parse the remainder repeatedly (like runShare).
	var bookID string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		if bookID == "" {
			bookID = rest[0]
		}
		rest = rest[1:]
		if len(rest) == 0 {
			break
		}
	}
	if bookID == "" {
		return errors.New(i18n.T("cmd.books.msg.need_book_id"))
	}
	dst := *out
	if dst == "" {
		dst = bookID + ".bin"
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := writeFileAtomic(dst, func(w io.Writer) error {
		return c.DownloadBook(context.Background(), bookID, w)
	}); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.books.msg.saved", i18n.M{"Path": dst}))
	return nil
}

func booksMeta(cc *cmdCtx, nodeID string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	m, err := c.GetBookMeta(context.Background(), nodeID)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, m)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.books.msg.meta", i18n.M{
		"Title": m.Title, "Authors": m.Authors, "Series": m.Series, "SeriesIndex": m.SeriesIndex,
		"Language": m.Language, "Publisher": m.Publisher, "Date": m.Date, "Description": m.Description,
	}))
	return nil
}

func booksScan(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	scanning, err := c.EbookScan(context.Background())
	if err != nil {
		return err
	}
	if scanning {
		fmt.Fprintln(cc.Stdout, i18n.T("cmd.books.msg.scan_started"))
	} else {
		fmt.Fprintln(cc.Stdout, i18n.T("cmd.books.msg.scan_not_configured"))
	}
	return nil
}

func booksInfo(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	s, err := c.GetEbookSettings(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, s)
	}
	folder := "—"
	if s.Folder != nil {
		folder = s.Folder.Name
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.books.msg.info", i18n.M{
		"Enabled": s.Enabled, "Folder": folder, "HasPassword": s.HasPassword, "HasApiKey": s.HasApiKey,
	}))
	return nil
}
