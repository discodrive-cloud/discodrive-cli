package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:     "ls",
		Summary:  "cmd.ls.summary",
		Usage:    "cmd.ls.usage",
		Long:     "cmd.ls.long",
		Examples: []string{"cmd.ls.example.0", "cmd.ls.example.1", "cmd.ls.example.2"},
		Run:      runLs,
	})
	register(&command{
		Name:     "get",
		Summary:  "cmd.get.summary",
		Usage:    "cmd.get.usage",
		Long:     "cmd.get.long",
		Examples: []string{"cmd.get.example.0", "cmd.get.example.1", "cmd.get.example.2"},
		Run:      runGet,
	})
	register(&command{
		Name:     "put",
		Summary:  "cmd.put.summary",
		Usage:    "cmd.put.usage",
		Long:     "cmd.put.long",
		Examples: []string{"cmd.put.example.0"},
		Run:      runPut,
	})
	register(&command{
		Name:     "mkdir",
		Summary:  "cmd.mkdir.summary",
		Usage:    "cmd.mkdir.usage",
		Long:     "cmd.mkdir.long",
		Examples: []string{"cmd.mkdir.example.0"},
		Run:      runMkdir,
	})
}

// splitPath splits a slash-separated path into its parent and base name.
func splitPath(p string) (dir, base string) {
	p = strings.Trim(p, "/")
	dir, base = path.Split(p)
	return strings.Trim(dir, "/"), base
}

func runLs(cc *cmdCtx, args []string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	node, err := c.Resolve(context.Background(), target)
	if err != nil {
		return err
	}
	if !node.IsDir {
		return renderNodes(cc.Stdout, []ddclient.Node{node}, cc.JSON)
	}
	kids, err := c.ListChildren(context.Background(), node.ID)
	if err != nil {
		return err
	}
	return renderNodes(cc.Stdout, kids, cc.JSON)
}

func runGet(cc *cmdCtx, args []string) error {
	if len(args) < 1 {
		return errors.New(i18n.T("cmd.get.msg.need_path"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	node, err := c.Resolve(context.Background(), args[0])
	if err != nil {
		return err
	}
	if node.IsDir {
		return errors.New(i18n.T("cmd.get.msg.is_dir", i18n.M{"Path": args[0]}))
	}
	dest := path.Base(strings.Trim(args[0], "/"))
	if len(args) > 1 {
		dest = args[1]
	}
	if dest == "-" {
		return c.Download(context.Background(), node.ID, cc.contentOut())
	}
	if err := writeFileAtomic(dest, func(w io.Writer) error {
		return c.Download(context.Background(), node.ID, w)
	}); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.get.msg.saved", i18n.M{"Path": dest}))
	return nil
}

func runPut(cc *cmdCtx, args []string) error {
	if len(args) < 2 {
		return errors.New(i18n.T("cmd.put.msg.need_two_args"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	dir, base := splitPath(args[1])
	if base == "" {
		base = path.Base(args[0])
	}
	parent, err := c.Resolve(context.Background(), dir)
	if err != nil {
		return err
	}
	if !parent.IsDir {
		return errors.New(i18n.T("cmd.put.msg.not_folder", i18n.M{"Path": dir}))
	}
	// Carry the file's own date so the server does not stamp it with the upload time.
	var modTime time.Time
	if fi, serr := f.Stat(); serr == nil {
		modTime = fi.ModTime()
	}
	node, err := c.Upload(context.Background(), parent.ID, base, f, modTime)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, node)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.put.msg.uploaded", i18n.M{"Path": args[1]}))
	return nil
}

func runMkdir(cc *cmdCtx, args []string) error {
	if len(args) < 1 {
		return errors.New(i18n.T("cmd.mkdir.msg.need_path"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	dir, base := splitPath(args[0])
	if base == "" {
		return errors.New(i18n.T("cmd.mkdir.msg.empty_name"))
	}
	parent, err := c.Resolve(context.Background(), dir)
	if err != nil {
		return err
	}
	if !parent.IsDir {
		return errors.New(i18n.T("cmd.mkdir.msg.not_folder", i18n.M{"Path": dir}))
	}
	node, err := c.CreateFolder(context.Background(), parent.ID, base)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, node)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.mkdir.msg.created", i18n.M{"Path": args[0]}))
	return nil
}

func init() {
	register(&command{
		Name:     "mv",
		Summary:  "cmd.mv.summary",
		Usage:    "cmd.mv.usage",
		Long:     "cmd.mv.long",
		Examples: []string{"cmd.mv.example.0", "cmd.mv.example.1"},
		Changing: true,
		Run:      runMv,
	})
	register(&command{
		Name:     "rename",
		Summary:  "cmd.rename.summary",
		Usage:    "cmd.rename.usage",
		Long:     "cmd.rename.long",
		Examples: []string{"cmd.rename.example.0", "cmd.rename.example.1"},
		Changing: true,
		Run:      runRename,
	})
	register(&command{
		Name:     "rm",
		Summary:  "cmd.rm.summary",
		Usage:    "cmd.rm.usage",
		Long:     "cmd.rm.long",
		Examples: []string{"cmd.rm.example.0"},
		Changing: true,
		Run:      runRm,
	})
	register(&command{
		Name:    "share",
		Summary: "cmd.share.summary",
		Usage:   "cmd.share.usage",
		Long:    "cmd.share.long",
		Examples: []string{
			"cmd.share.example.0",
			"cmd.share.example.1",
			"cmd.share.example.2",
		},
		Run: runShare,
	})
}

// newFlagSet returns a FlagSet that writes errors to the command's stderr and does not exit the process.
func newFlagSet(name string, cc *cmdCtx) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(cc.Stderr)
	return fs
}

func runRename(cc *cmdCtx, args []string) error {
	if len(args) < 2 {
		return errors.New(i18n.T("cmd.rename.msg.need_args"))
	}
	newName := args[1]
	if strings.Contains(newName, "/") {
		return errors.New(i18n.T("cmd.rename.msg.name_has_slash", i18n.M{"Name": newName}))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	node, err := c.Resolve(context.Background(), args[0])
	if err != nil {
		return err
	}
	if node.ID == "" {
		return errors.New(i18n.T("cmd.rename.msg.cannot_rename_root"))
	}
	renamed, err := c.Rename(context.Background(), node.ID, newName)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, renamed)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.rename.msg.renamed", i18n.M{"Path": args[0], "Name": newName}))
	return nil
}

func runRm(cc *cmdCtx, args []string) error {
	if len(args) < 1 {
		return errors.New(i18n.T("cmd.rm.msg.need_path"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	node, err := c.Resolve(context.Background(), args[0])
	if err != nil {
		return err
	}
	if node.ID == "" {
		return errors.New(i18n.T("cmd.rm.msg.cannot_delete_root"))
	}
	if err := c.Delete(context.Background(), node.ID); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.rm.msg.deleted", i18n.M{"Path": args[0]}))
	return nil
}

func runMv(cc *cmdCtx, args []string) error {
	if len(args) < 2 {
		return errors.New(i18n.T("cmd.mv.msg.need_two_args"))
	}
	c, err := cc.client()
	if err != nil {
		return err
	}
	ctx := context.Background()
	src, err := c.Resolve(ctx, args[0])
	if err != nil {
		return err
	}
	if src.ID == "" {
		return errors.New(i18n.T("cmd.mv.msg.cannot_move_root"))
	}
	// dst is an existing folder → move into it keeping the same name.
	dstNode, derr := c.Resolve(ctx, args[1])
	if derr != nil && !ddclient.IsNotFound(derr) {
		return derr
	}
	if derr == nil && dstNode.IsDir {
		if _, err := c.Move(ctx, src.ID, dstNode.ID); err != nil {
			return err
		}
		fmt.Fprint(cc.Stdout, i18n.T("cmd.mv.msg.moved_into_folder", i18n.M{"Src": args[0], "Dst": args[1]}))
		return nil
	}
	// otherwise: dst = new parent + new name.
	dstDir, dstBase := splitPath(args[1])
	parent, err := c.Resolve(ctx, dstDir)
	if err != nil {
		return err
	}
	if !parent.IsDir {
		return errors.New(i18n.T("err.files.not_dir", i18n.M{"Path": dstDir}))
	}
	if dstBase == "" {
		dstBase = src.Name
	}
	if _, err := c.Relocate(ctx, src.ID, parent.ID, dstBase); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.mv.msg.moved", i18n.M{"Src": args[0], "Dst": args[1]}))
	return nil
}

// parsePositional parses fs over args where up to max positional arguments may be
// interleaved with flags. flag.Parse stops at the first non-flag, so a positional
// would otherwise "swallow" every flag that follows it; the remainder is parsed
// again until it runs out. Positionals beyond max are dropped.
func parsePositional(fs *flag.FlagSet, args []string, max int) ([]string, error) {
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		if len(pos) < max {
			pos = append(pos, rest[0])
		}
		rest = rest[1:]
	}
}

func runShare(cc *cmdCtx, args []string) error {
	fs := newFlagSet("share", cc)
	email := fs.String("email", "", i18n.T("cmd.share.flag.email"))
	access := fs.String("access", "view", i18n.T("cmd.share.flag.access"))
	expires := fs.Duration("expires", 0, i18n.T("cmd.share.flag.expires"))

	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New(i18n.T("cmd.share.msg.need_path"))
	}
	path := pos[0]
	c, err := cc.client()
	if err != nil {
		return err
	}
	node, err := c.Resolve(context.Background(), path)
	if err != nil {
		return err
	}
	opts := ddclient.ShareOpts{Access: *access}
	if *email != "" {
		opts.Email = *email
	} else {
		opts.Link = true
	}
	if *expires > 0 {
		secs := int64(expires.Seconds())
		opts.ExpiresInSeconds = &secs
	}
	res, err := c.Share(context.Background(), node.ID, opts)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, res)
	}
	if res.Token != "" {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.share.msg.link", i18n.M{"URL": c.ShareLinkURL(res.Token), "Access": res.Access}))
	} else {
		fmt.Fprint(cc.Stdout, i18n.T("cmd.share.msg.granted", i18n.M{"ShareID": res.ShareID, "Access": res.Access}))
	}
	return nil
}
