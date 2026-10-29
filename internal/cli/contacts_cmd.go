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
		Name:    "contacts",
		Summary: "cmd.contacts.summary",
		Usage:   "cmd.contacts.usage",
		Long:    "cmd.contacts.long",
		Examples: []string{
			"cmd.contacts.example.0",
			"cmd.contacts.example.1",
			"cmd.contacts.example.2",
			"cmd.contacts.example.3",
		},
		Run: runContacts,
	})
}

// subverb extracts the subverb: the first non-flag argument. Returns ("", args) if there is none.
func subverb(args []string) (string, []string) {
	if len(args) == 0 || args[0] == "" || args[0][0] == '-' {
		return "", args
	}
	return args[0], args[1:]
}

func runContacts(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "list":
		return contactsList(cc)
	case "show":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.contacts.msg.need_uid_show"))
		}
		return contactsShow(cc, rest[0])
	case "add":
		return contactsAdd(cc, rest)
	case "rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.contacts.msg.need_uid_rm"))
		}
		return contactsRm(cc, rest[0])
	default:
		return errors.New(i18n.T("cmd.contacts.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

func contactsList(cc *cmdCtx) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	items, err := c.ListContacts(context.Background())
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, items)
	}
	for _, it := range items {
		email := ""
		if len(it.Emails) > 0 {
			email = it.Emails[0]
		}
		fmt.Fprintf(cc.Stdout, "%-24s %-28s %s\n", it.UID, it.FullName, email)
	}
	return nil
}

func contactsShow(cc *cmdCtx, uid string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	ct, err := c.GetContact(context.Background(), uid)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, ct)
	}
	fmt.Fprintf(cc.Stdout, "%s\n", ct.FullName)
	if ct.Org != "" {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.contacts.msg.org_line", i18n.M{"Org": ct.Org}))
	}
	for _, e := range ct.Emails {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.contacts.msg.email_line", i18n.M{"Type": e.Type, "Value": e.Value}))
	}
	for _, p := range ct.Phones {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.contacts.msg.phone_line", i18n.M{"Type": p.Type, "Value": p.Value}))
	}
	return nil
}

func contactsAdd(cc *cmdCtx, args []string) error {
	fs := newFlagSet("contacts add", cc)
	name := fs.String("name", "", i18n.T("cmd.contacts.flag.name"))
	email := fs.String("email", "", "email")
	phone := fs.String("phone", "", i18n.T("cmd.contacts.flag.phone"))
	org := fs.String("org", "", i18n.T("cmd.contacts.flag.org"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New(i18n.T("cmd.contacts.msg.need_name"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	form := ddclient.Contact{FullName: *name, Org: *org}
	if *email != "" {
		form.Emails = []ddclient.TypedValue{{Type: "home", Value: *email}}
	}
	if *phone != "" {
		form.Phones = []ddclient.TypedValue{{Type: "cell", Value: *phone}}
	}
	uid, err := c.CreateContact(context.Background(), form)
	if err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.contacts.msg.created", i18n.M{"UID": uid}))
	return nil
}

func contactsRm(cc *cmdCtx, uid string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteContact(context.Background(), uid); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.contacts.msg.deleted", i18n.M{"UID": uid}))
	return nil
}
