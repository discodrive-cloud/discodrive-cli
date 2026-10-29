package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "music",
		Summary: "cmd.music.summary",
		Usage:   "cmd.music.usage",
		Long:    "cmd.music.long",
		Examples: []string{
			"cmd.music.example.0",
			"cmd.music.example.1",
			"cmd.music.example.2",
			"cmd.music.example.3",
		},
		Run: runMusic,
	})
}

func runMusic(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "radio":
		return musicRadioList(cc)
	case "list":
		return musicList(cc, rest)
	case "radio-add":
		return musicRadioAdd(cc, rest)
	case "radio-rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.music.msg.need_id_radio_rm"))
		}
		return musicRadioRm(cc, rest[0])
	case "podcasts":
		return musicPodcastsList(cc)
	case "podcast-add":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.music.msg.need_feed_url"))
		}
		return musicPodcastAdd(cc, rest[0])
	case "podcast-rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.music.msg.need_id_podcast_rm"))
		}
		return musicPodcastRm(cc, rest[0])
	case "scan":
		return musicScan(cc)
	case "info":
		return musicInfo(cc)
	default:
		return errors.New(i18n.T("cmd.music.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

// musicList prints the contents of the folder designated as the music folder (a file listing).
// An optional subpath descends into it by the names of nested folders.
func musicList(cc *cmdCtx, args []string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	kids, err := c.ListMusicFolder(context.Background(), sub)
	if err != nil {
		return err
	}
	return renderNodes(cc.Stdout, kids, cc.JSON)
}

func musicRadioList(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	items, err := c.ListRadio(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, items)
	}
	for _, r := range items {
		fmt.Fprintf(cc.Stdout, "%-24s %-20s %s\n", r.ID, r.Name, r.StreamURL)
	}
	return nil
}

func musicRadioAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("music radio-add", cc)
	name := fs.String("name", "", i18n.T("cmd.music.flag.name"))
	stream := fs.String("stream", "", i18n.T("cmd.music.flag.stream"))
	home := fs.String("home", "", i18n.T("cmd.music.flag.home"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *stream == "" {
		return errors.New(i18n.T("cmd.music.msg.need_name_stream"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	r, err := c.CreateRadio(context.Background(), ddclient.Radio{Name: *name, StreamURL: *stream, HomepageURL: *home})
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, r)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.music.msg.station_added", i18n.M{"Name": r.Name, "ID": r.ID}))
	return nil
}

func musicRadioRm(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteRadio(context.Background(), id); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.music.msg.station_removed", i18n.M{"ID": id}))
	return nil
}

func musicPodcastsList(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	items, err := c.ListPodcasts(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, items)
	}
	for _, p := range items {
		fmt.Fprintf(cc.Stdout, "%-24s %-30s %s\n", p.ID, p.Title, p.FeedURL)
	}
	return nil
}

func musicPodcastAdd(cc *cmdCtx, feedURL string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	p, err := c.AddPodcast(context.Background(), feedURL)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, p)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.music.msg.podcast_added", i18n.M{"Title": p.Title, "ID": p.ID}))
	return nil
}

func musicPodcastRm(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeletePodcast(context.Background(), id); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.music.msg.podcast_removed", i18n.M{"ID": id}))
	return nil
}

func musicScan(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	scanning, err := c.MusicScan(context.Background())
	if err != nil {
		return err
	}
	if scanning {
		fmt.Fprintln(cc.Stdout, i18n.T("cmd.music.msg.scan_started"))
	} else {
		fmt.Fprintln(cc.Stdout, i18n.T("cmd.music.msg.scan_not_configured"))
	}
	return nil
}

func musicInfo(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	s, err := c.GetMusicSettings(context.Background())
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
	fmt.Fprint(cc.Stdout, i18n.T("cmd.music.msg.info", i18n.M{"Enabled": s.Enabled, "Folder": folder, "HasPassword": s.HasPassword}))
	return nil
}
