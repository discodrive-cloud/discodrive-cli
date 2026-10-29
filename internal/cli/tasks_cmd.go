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
		Name:    "tasks",
		Summary: "cmd.tasks.summary",
		Usage:   "cmd.tasks.usage",
		Long:    "cmd.tasks.long",
		Examples: []string{
			"cmd.tasks.example.0",
			"cmd.tasks.example.1",
			"cmd.tasks.example.2",
			"cmd.tasks.example.3",
		},
		Run: runTasks,
	})
}

func runTasks(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "list":
		return tasksList(cc)
	case "add":
		return tasksAdd(cc, rest)
	case "done":
		return tasksSetDone(cc, rest, true)
	case "undone":
		return tasksSetDone(cc, rest, false)
	case "rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.tasks.msg.need_uid_rm"))
		}
		return tasksRm(cc, rest[0])
	default:
		return errors.New(i18n.T("cmd.tasks.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func tasksList(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	items, err := c.ListTasks(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, items)
	}
	for _, t := range items {
		mark := "[ ]"
		if t.Completed {
			mark = "[x]"
		}
		due := ""
		if t.Due != "" {
			due = i18n.T("cmd.tasks.msg.due_suffix", i18n.M{"Due": t.Due})
		}
		fmt.Fprintf(cc.Stdout, "%s %-24s %s%s\n", mark, t.UID, t.Summary, due)
	}
	return nil
}

func tasksAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("tasks add", cc)
	summary := fs.String("summary", "", i18n.T("cmd.tasks.flag.summary"))
	due := fs.String("due", "", i18n.T("cmd.tasks.flag.due"))
	priority := fs.Int("priority", 0, i18n.T("cmd.tasks.flag.priority"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *summary == "" {
		return errors.New(i18n.T("cmd.tasks.msg.need_summary"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	uid, err := c.CreateTask(context.Background(), ddclient.Task{Summary: *summary, Due: *due, Priority: *priority})
	if err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.tasks.msg.created", i18n.M{"UID": uid}))
	return nil
}

func tasksSetDone(cc *cmdCtx, args []string, completed bool) error {
	if len(args) < 1 {
		return errors.New(i18n.T("cmd.tasks.msg.need_uid"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	if _, err := c.SetTaskDone(context.Background(), args[0], completed); err != nil {
		return err
	}
	if completed {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.tasks.msg.marked_done", i18n.M{"UID": args[0]}))
	} else {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.tasks.msg.unmarked", i18n.M{"UID": args[0]}))
	}
	return nil
}

func tasksRm(cc *cmdCtx, uid string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteTask(context.Background(), uid); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.tasks.msg.deleted", i18n.M{"UID": uid}))
	return nil
}
