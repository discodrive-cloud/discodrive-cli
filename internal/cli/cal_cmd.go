package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "cal",
		Summary: "cmd.cal.summary",
		Usage:   "cmd.cal.usage",
		Long:    "cmd.cal.long",
		Examples: []string{
			"cmd.cal.example.0",
			"cmd.cal.example.1",
			"cmd.cal.example.2",
			"cmd.cal.example.3",
			"cmd.cal.example.4",
			"cmd.cal.example.5",
		},
		Run: runCal,
	})
}

func runCal(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "events":
		return calEvents(cc, rest)
	case "add":
		return calAdd(cc, rest)
	case "show":
		return calShow(cc, rest)
	case "set":
		return calSet(cc, rest)
	case "rm":
		return calRm(cc, rest)
	case "calendars":
		return calCalendars(cc)
	default:
		return errors.New(i18n.T("cmd.cal.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func calEvents(cc *cmdCtx, args []string) error {
	fs := newFlagSet("cal events", cc)
	from := fs.String("from", "", i18n.T("cmd.cal.flag.from"))
	to := fs.String("to", "", i18n.T("cmd.cal.flag.to"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	start := *from
	end := *to
	if start == "" {
		start = time.Now().UTC().Format(time.RFC3339)
	}
	if end == "" {
		end = time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	evs, err := c.ListEvents(context.Background(), start, end)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, evs)
	}
	for _, e := range evs {
		fmt.Fprintf(cc.Stdout, "%-20s %-28s %s\n", e.Start, e.Summary, e.UID)
	}
	return nil
}

func calAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("cal add", cc)
	summary := fs.String("summary", "", i18n.T("cmd.cal.flag.summary"))
	start := fs.String("start", "", i18n.T("cmd.cal.flag.start"))
	end := fs.String("end", "", i18n.T("cmd.cal.flag.end"))
	location := fs.String("location", "", i18n.T("cmd.cal.flag.location"))
	description := fs.String("description", "", i18n.T("cmd.cal.flag.description"))
	allDay := fs.Bool("all-day", false, i18n.T("cmd.cal.flag.all_day"))
	repeat := fs.String("repeat", "", i18n.T("cmd.cal.flag.repeat"))
	until := fs.String("until", "", i18n.T("cmd.cal.flag.until"))
	alarm := fs.String("alarm", "", i18n.T("cmd.cal.flag.alarm"))
	calendar := fs.String("calendar", "", i18n.T("cmd.cal.flag.calendar"))
	tz := fs.String("tz", "", i18n.T("cmd.cal.flag.tz"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *summary == "" || *start == "" {
		return errors.New(i18n.T("cmd.cal.msg.need_summary_start"))
	}
	if err := ddclient.CheckAlarm(*alarm); err != nil {
		return err
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	uid, err := c.CreateEvent(context.Background(), ddclient.EventForm{
		Summary: *summary, Start: *start, End: *end, Location: *location,
		Description: *description, AllDay: *allDay, Freq: *repeat, Until: *until,
		Alarm: *alarm, CalendarID: *calendar, TZ: *tz,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.cal.msg.created", i18n.M{"UID": uid}))
	return nil
}

func calShow(cc *cmdCtx, args []string) error {
	fs := newFlagSet("cal show", cc)
	calendar := fs.String("calendar", "", i18n.T("cmd.cal.flag.calendar"))
	uid, err := parseUIDArgs(fs, args)
	if err != nil {
		return err
	}
	if uid == "" {
		return errors.New(i18n.T("cmd.cal.msg.need_uid_show"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	ev, err := c.GetEvent(context.Background(), uid, *calendar)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, ev)
	}
	printEvent(cc, ev)
	return nil
}

func calSet(cc *cmdCtx, args []string) error {
	fs := newFlagSet("cal set", cc)
	summary := fs.String("summary", "", i18n.T("cmd.cal.flag.summary_set"))
	start := fs.String("start", "", i18n.T("cmd.cal.flag.start_set"))
	end := fs.String("end", "", i18n.T("cmd.cal.flag.end_set"))
	location := fs.String("location", "", i18n.T("cmd.cal.flag.location"))
	description := fs.String("description", "", i18n.T("cmd.cal.flag.description"))
	allDay := fs.Bool("all-day", false, i18n.T("cmd.cal.flag.all_day"))
	repeat := fs.String("repeat", "", i18n.T("cmd.cal.flag.repeat"))
	until := fs.String("until", "", i18n.T("cmd.cal.flag.until"))
	alarm := fs.String("alarm", "", i18n.T("cmd.cal.flag.alarm_set"))
	calendar := fs.String("calendar", "", i18n.T("cmd.cal.flag.calendar"))
	tz := fs.String("tz", "", i18n.T("cmd.cal.flag.tz"))
	uid, err := parseUIDArgs(fs, args)
	if err != nil {
		return err
	}
	if uid == "" {
		return errors.New(i18n.T("cmd.cal.msg.need_uid_set"))
	}

	// Only the flags actually given are changed; an empty value clears the field.
	patch := ddclient.EventPatch{CalendarID: *calendar}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "tz":
			patch.TZ = tz
		case "summary":
			patch.Summary = summary
		case "start":
			patch.Start = start
		case "end":
			patch.End = end
		case "location":
			patch.Location = location
		case "description":
			patch.Description = description
		case "all-day":
			patch.AllDay = allDay
		case "repeat":
			patch.Freq = repeat
		case "until":
			patch.Until = until
		case "alarm":
			patch.Alarm = alarm
		}
	})
	if patch.Empty() {
		return errors.New(i18n.T("cmd.cal.msg.set_no_flags"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	ev, err := c.PatchEvent(context.Background(), uid, patch)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, ev)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.cal.msg.updated", i18n.M{"UID": uid}))
	printEvent(cc, ev)
	return nil
}

// printEvent renders the master form of an event as aligned "field: value" lines,
// skipping what is not set.
func printEvent(cc *cmdCtx, ev ddclient.EventForm) {
	line := func(labelKey, value string) {
		if value != "" {
			fmt.Fprintf(cc.Stdout, "%-14s %s\n", i18n.T(labelKey), value)
		}
	}
	line("cmd.cal.field.summary", ev.Summary)
	line("cmd.cal.field.start", ev.Start)
	line("cmd.cal.field.end", ev.End)
	if ev.AllDay {
		line("cmd.cal.field.all_day", i18n.T("core.yes"))
	}
	line("cmd.cal.field.location", ev.Location)
	line("cmd.cal.field.description", ev.Description)
	line("cmd.cal.field.repeat", ev.Freq)
	line("cmd.cal.field.until", ev.Until)
	line("cmd.cal.field.alarm", ev.Alarm)
	line("cmd.cal.field.calendar", ev.CalendarID)
	line("cmd.cal.field.tz", ev.TZ)
	line("cmd.cal.field.uid", ev.UID)
}

// parseUIDArgs parses flags around a single positional uid: flag.Parse stops at the
// first non-flag, so the remainder is parsed repeatedly to accept flags on both sides.
func parseUIDArgs(fs *flag.FlagSet, args []string) (string, error) {
	var uid string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return "", err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			return uid, nil
		}
		if uid == "" {
			uid = rest[0]
		}
		rest = rest[1:]
		if len(rest) == 0 {
			return uid, nil
		}
	}
}

func calRm(cc *cmdCtx, args []string) error {
	fs := newFlagSet("cal rm", cc)
	calendar := fs.String("calendar", "", i18n.T("cmd.cal.flag.rm_calendar"))
	uid, err := parseUIDArgs(fs, args)
	if err != nil {
		return err
	}
	if uid == "" {
		return errors.New(i18n.T("cmd.cal.msg.need_uid"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteEvent(context.Background(), uid, *calendar); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.cal.msg.deleted", i18n.M{"UID": uid}))
	return nil
}

func calCalendars(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	cals, err := c.ListCalendars(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, cals)
	}
	for _, cal := range cals {
		def := ""
		if cal.IsDefault {
			def = i18n.T("cmd.cal.msg.default_mark")
		}
		fmt.Fprintf(cc.Stdout, "%-24s %s%s\n", cal.ID, cal.Name, def)
	}
	return nil
}
