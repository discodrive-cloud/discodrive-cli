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
		Name:    "pocket",
		Summary: "cmd.pocket.summary",
		Usage:   "cmd.pocket.usage",
		Long:    "cmd.pocket.long",
		Examples: []string{
			"cmd.pocket.example.0",
			"cmd.pocket.example.1",
			"cmd.pocket.example.2",
			"cmd.pocket.example.3",
		},
		Changing: true,
		Run:      runPocket,
	})
}

func runPocket(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "list":
		return pocketList(cc, rest)
	case "add":
		return pocketAdd(cc, rest)
	case "read":
		return pocketRead(cc, rest)
	case "retry":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.pocket.msg.need_id"))
		}
		return pocketRetry(cc, rest[0])
	case "rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.pocket.msg.need_id"))
		}
		return pocketRm(cc, rest[0])
	default:
		return errors.New(i18n.T("cmd.pocket.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func pocketList(cc *cmdCtx, args []string) error {
	fs := newFlagSet("pocket list", cc)
	status := fs.String("status", "", i18n.T("cmd.pocket.flag.status"))
	q := fs.String("q", "", i18n.T("cmd.pocket.flag.q"))
	limit := fs.Int("limit", 0, i18n.T("cmd.pocket.flag.limit"))
	offset := fs.Int("offset", 0, i18n.T("cmd.pocket.flag.offset"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	items, err := c.ListSavedArticles(context.Background(), ddclient.SavedQuery{
		Status: *status, Q: *q, Limit: *limit, Offset: *offset,
	})
	if err != nil {
		return err
	}
	return renderSavedItems(cc.Stdout, items, cc.JSON)
}

func pocketAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("pocket add", cc)
	title := fs.String("title", "", i18n.T("cmd.pocket.flag.title"))
	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New(i18n.T("cmd.pocket.msg.need_url"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	item, err := c.SaveArticle(context.Background(), pos[0], *title)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, item)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.pocket.msg.saved", i18n.M{"ID": item.ID, "Status": item.Status}))
	return nil
}

func pocketRead(cc *cmdCtx, args []string) error {
	fs := newFlagSet("pocket read", cc)
	out := fs.String("out", "", i18n.T("cmd.pocket.flag.out"))
	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New(i18n.T("cmd.pocket.msg.need_id"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	if *out == "" || *out == "-" {
		return c.SavedContent(context.Background(), pos[0], cc.Stdout)
	}
	if err := writeFileAtomic(*out, func(w io.Writer) error {
		return c.SavedContent(context.Background(), pos[0], w)
	}); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.pocket.msg.written", i18n.M{"Path": *out}))
	return nil
}

func pocketRetry(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	item, err := c.RetrySavedItem(context.Background(), id)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, item)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.pocket.msg.retrying", i18n.M{"ID": item.ID}))
	return nil
}

func pocketRm(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteSavedItem(context.Background(), id); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.pocket.msg.deleted", i18n.M{"ID": id}))
	return nil
}
