package ddmcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/discodrive-cloud/discodrive-cli/internal/atomicfile"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/google/jsonschema-go/jsonschema"
)

// Moving files between DiscoDrive and the machine dd-mcp runs on. The bytes go
// straight between disk and server: they never pass through the model's context,
// so binary and large files arrive intact.

type uploadInput struct {
	Path string `json:"path" jsonschema:"destination in DiscoDrive: folder/name, or an existing folder to keep the local file's name"`
	// A pointer, so that an empty text file can still be uploaded.
	Content   *string `json:"content,omitempty" jsonschema:"file contents as text (text files only)"`
	LocalPath string  `json:"local_path,omitempty" jsonschema:"absolute path of a file on this machine to upload, any type including binary (instead of content)"`
}

type downloadInput struct {
	Path      string `json:"path" jsonschema:"file in DiscoDrive (slash-path)"`
	LocalPath string `json:"local_path" jsonschema:"absolute path on this machine to save to; an existing folder keeps the file's name"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace an existing local file (default: refuse)"`
}

type downloadOutput struct {
	Path        string `json:"path"`
	LocalPath   string `json:"local_path" jsonschema:"where the file was saved"`
	Size        int64  `json:"size"`
	ContentHash string `json:"content_hash" jsonschema:"sha256 (hex) of the saved bytes"`
}

func uploadDescription(o serverOptions) string {
	if o.LocalFiles {
		return i18n.T("tool.dd_files_upload.desc")
	}
	return i18n.T("tool.dd_files_upload.desc_remote")
}

// uploadSchema drops local_path where the client may be on another machine, so the
// model is not offered a parameter that can only fail.
func uploadSchema(o serverOptions) *jsonschema.Schema {
	s, err := jsonschema.For[uploadInput](nil)
	if err != nil {
		panic(err) // a static struct: fails only on a programming error, caught by tests
	}
	if !o.LocalFiles {
		delete(s.Properties, "local_path")
	}
	return s
}

// upload takes either inline text or a local file, never both.
func upload(ctx context.Context, c *ddclient.Client, o serverOptions, in uploadInput) (ddclient.Node, error) {
	hasContent, hasLocal := in.Content != nil, in.LocalPath != ""
	if hasLocal && !o.LocalFiles {
		return ddclient.Node{}, errors.New(i18n.T("tool.dd_files_upload.msg.local_remote"))
	}
	if hasContent == hasLocal {
		key := "tool.dd_files_upload.msg.need_one"
		if !o.LocalFiles {
			key = "tool.dd_files_upload.msg.need_content"
		}
		return ddclient.Node{}, errors.New(i18n.T(key))
	}
	if hasContent {
		parent, name, err := uploadTarget(ctx, c, in.Path, "")
		if err != nil {
			return ddclient.Node{}, err
		}
		// Inline text has no source file whose date to carry: the server dates it on arrival.
		return c.Upload(ctx, parent.ID, name, strings.NewReader(*in.Content), time.Time{})
	}

	if err := requireAbs(in.LocalPath); err != nil {
		return ddclient.Node{}, err
	}
	f, err := os.Open(in.LocalPath)
	if err != nil {
		return ddclient.Node{}, errors.New(i18n.T("err.files.local_read", i18n.M{"Path": in.LocalPath, "Err": err}))
	}
	defer f.Close()
	parent, name, err := uploadTarget(ctx, c, in.Path, filepath.Base(in.LocalPath))
	if err != nil {
		return ddclient.Node{}, err
	}
	return c.UploadFile(ctx, parent.ID, name, f)
}

// uploadTarget resolves where an upload goes. An existing folder (the root, a path
// ending in "/", or simply a folder that exists) receives the file under localName;
// anything else is read as folder/name.
func uploadTarget(ctx context.Context, c *ddclient.Client, dest, localName string) (ddclient.Node, string, error) {
	intoFolder := strings.Trim(dest, "/") == "" || strings.HasSuffix(dest, "/")
	if !intoFolder {
		if n, err := c.Resolve(ctx, dest); err == nil && n.IsDir {
			intoFolder = true
		}
	}
	dir, name := splitDirBase(dest)
	if intoFolder {
		dir, name = strings.Trim(dest, "/"), localName
	}
	if name == "" {
		return ddclient.Node{}, "", errors.New(i18n.T("tool.dd_files_upload.msg.need_name", i18n.M{"Path": dest}))
	}
	parent, err := c.Resolve(ctx, dir)
	if err != nil {
		return ddclient.Node{}, "", err
	}
	if !parent.IsDir {
		return ddclient.Node{}, "", errors.New(i18n.T("err.files.not_dir", i18n.M{"Path": dir}))
	}
	return parent, name, nil
}

// download saves a DiscoDrive file to the local disk: staged beside the destination
// and moved into place only after the whole file arrived and its hash matched.
// Without overwrite an existing file is never replaced, even one that appears while
// the download runs.
func download(ctx context.Context, c *ddclient.Client, in downloadInput) (downloadOutput, error) {
	if err := requireAbs(in.LocalPath); err != nil {
		return downloadOutput{}, err
	}
	node, err := c.Resolve(ctx, in.Path)
	if err != nil {
		return downloadOutput{}, err
	}
	if node.IsDir {
		return downloadOutput{}, errors.New(i18n.T("tool.dd_files_download.msg.is_dir", i18n.M{"Path": in.Path}))
	}
	dst := in.LocalPath
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		// The name comes from the server: it must not lead out of the chosen folder.
		if !isLocalName(node.Name) {
			return downloadOutput{}, errors.New(i18n.T("tool.dd_files_download.msg.unsafe_name", i18n.M{"Name": node.Name}))
		}
		dst = filepath.Join(dst, node.Name)
	}
	exists := errors.New(i18n.T("tool.dd_files_download.msg.exists", i18n.M{"Path": dst}))
	if !in.Overwrite {
		if _, err := os.Lstat(dst); err == nil {
			return downloadOutput{}, exists // fail before spending the transfer
		}
	}

	var size int64
	var sum string
	write := func(w io.Writer) error {
		h := sha256.New()
		cw := &countingWriter{}
		if err := c.Download(ctx, node.ID, io.MultiWriter(w, h, cw)); err != nil {
			return err
		}
		size, sum = cw.n, hex.EncodeToString(h.Sum(nil))
		if node.ContentHash != "" && !strings.EqualFold(node.ContentHash, sum) {
			return errors.New(i18n.T("tool.dd_files_download.msg.hash_mismatch", i18n.M{"Got": sum, "Want": node.ContentHash}))
		}
		return nil
	}
	if in.Overwrite {
		err = atomicfile.Write(dst, write)
	} else {
		err = atomicfile.WriteNew(dst, write)
	}
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	switch {
	case errors.Is(err, atomicfile.ErrExists):
		return downloadOutput{}, exists
	case errors.As(err, &pathErr), errors.As(err, &linkErr):
		// The disk said no (read-only file, no such folder, no space): name the way out.
		return downloadOutput{}, errors.New(i18n.T("tool.dd_files_download.msg.local_write", i18n.M{"Path": dst, "Err": err}))
	case err != nil:
		return downloadOutput{}, err
	}
	return downloadOutput{Path: in.Path, LocalPath: dst, Size: size, ContentHash: sum}, nil
}

// requireAbs: local_path is read against dd-mcp's own working directory, which the
// assistant cannot see, so only an absolute path means the same to both.
func requireAbs(p string) error {
	if !filepath.IsAbs(p) {
		return errors.New(i18n.T("err.files.local_not_abs", i18n.M{"Path": p}))
	}
	return nil
}

// isLocalName: a single path element that stays inside the folder it is joined to,
// on every OS (a backslash is a separator on Windows and allowed in a server name).
func isLocalName(name string) bool {
	return name != "." && !strings.ContainsAny(name, `/\`) && filepath.IsLocal(name)
}

type countingWriter struct{ n int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

// isText: what dd_files_read can hand to the model without mangling it.
func isText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// fetchHint names the way to get a file that dd_files_read cannot return, given
// what this server offers.
func fetchHint(o serverOptions, path string) string {
	m := i18n.M{"Path": path}
	switch {
	case o.localTransfer():
		return i18n.T("err.file_hint.download_tool", m)
	case o.LocalFiles:
		return i18n.T("err.file_hint.cli_or_enable", m)
	default:
		return i18n.T("err.file_hint.cli", m)
	}
}
