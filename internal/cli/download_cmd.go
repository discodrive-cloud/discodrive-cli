package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "download",
		Summary: "cmd.download.summary",
		Usage:   "cmd.download.usage",
		Long:    "cmd.download.long",
		Examples: []string{
			"cmd.download.example.0",
			"cmd.download.example.1",
			"cmd.download.example.2",
		},
		Changing: true,
		Run:      runDownload,
	})
}

func runDownload(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "list":
		return downloadList(cc)
	case "add":
		return downloadAdd(cc, rest)
	case "status":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.download.msg.need_id"))
		}
		return downloadStatus(cc, rest[0])
	case "rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.download.msg.need_id"))
		}
		return downloadRm(cc, rest[0])
	default:
		return errors.New(i18n.T("cmd.download.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func downloadAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("download add", cc)
	title := fs.String("title", "", i18n.T("cmd.download.flag.title"))
	cookie := fs.String("cookie", "", i18n.T("cmd.download.flag.cookie"))
	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New(i18n.T("cmd.download.msg.need_url"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	item, err := c.QueueDownload(context.Background(), pos[0], *title, *cookie)
	if err != nil {
		return err
	}
	// The journal is a convenience; failing to write it must not hide the id.
	if jerr := rememberDownload(cc, downloadEntry{
		ID: item.ID, URL: item.URL, Title: item.Title,
		At: time.Now().UTC().Format(time.RFC3339),
	}); jerr != nil {
		fmt.Fprintf(cc.Stderr, "%s\n", i18n.T("cmd.download.msg.journal_failed", i18n.M{"Msg": jerr.Error()}))
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, item)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.download.msg.queued", i18n.M{"ID": item.ID, "Status": item.Status}))
	return nil
}

func downloadStatus(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	item, err := c.GetSavedItem(context.Background(), id)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, item)
	}
	return renderSavedItems(cc.Stdout, []ddclient.SavedItem{item}, false)
}

// downloadList walks the current login's part of the local journal and asks the
// server about each id. The server keeps no list of downloads and drops finished
// ones after a while, so entries it reports unknown are pruned instead of being
// reported. The journal is shared by all profiles: another login's entries are
// neither queried nor touched, and an untagged (pre-profile) entry is pruned
// only when this is the sole profile — otherwise it may belong to another one.
func downloadList(cc *cmdCtx) error {
	entries := loadDownloads(cc)
	o, known := cc.journalOwner()
	mine := 0
	for _, e := range entries {
		if e.mine(o, known) {
			mine++
		}
	}
	if mine == 0 {
		if cc.JSON {
			return renderJSON(cc.Stdout, []ddclient.SavedItem{})
		}
		fmt.Fprint(cc.Stdout, i18n.T("cmd.download.msg.journal_empty"))
		return nil
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	ctx := context.Background()
	items := []ddclient.SavedItem{}
	removed := map[downloadEntry]bool{}
	adopted := map[downloadEntry]downloadEntry{}
	for _, e := range entries {
		if !e.mine(o, known) {
			continue
		}
		item, err := c.GetSavedItem(ctx, e.ID)
		if ddclient.IsUnknownSavedItem(err) {
			if !e.untagged() || o.Sole {
				removed[e] = true
			}
			continue
		}
		if err != nil {
			return err
		}
		if e.untagged() && known {
			// This server knows the id: it is this login's, adopt it.
			original := e
			e.Profile, e.Server = o.Profile, o.Server
			adopted[original] = e
		}
		items = append(items, item)
	}
	if len(removed) > 0 || len(adopted) > 0 {
		// Apply only changes to entries actually queried; preserve concurrent additions.
		if jerr := updateDownloads(cc, func(current []downloadEntry) []downloadEntry {
			kept := make([]downloadEntry, 0, len(current))
			for _, e := range current {
				if removed[e] {
					continue
				}
				if replacement, ok := adopted[e]; ok {
					e = replacement
				}
				kept = append(kept, e)
			}
			return kept
		}); jerr != nil {
			fmt.Fprintf(cc.Stderr, "%s\n", i18n.T("cmd.download.msg.journal_failed", i18n.M{"Msg": jerr.Error()}))
		}
	}
	return renderSavedItems(cc.Stdout, items, cc.JSON)
}

func downloadRm(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteSavedItem(context.Background(), id); err != nil {
		return err
	}
	if jerr := forgetDownload(cc, id); jerr != nil {
		fmt.Fprintf(cc.Stderr, "%s\n", i18n.T("cmd.download.msg.journal_failed", i18n.M{"Msg": jerr.Error()}))
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.download.msg.cancelled", i18n.M{"ID": id}))
	return nil
}
