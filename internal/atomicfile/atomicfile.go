// Package atomicfile writes local files so that a failed or interrupted write
// never leaves a half-written destination behind. dd-cli and dd-mcp share it.
package atomicfile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Write stages bytes beside the destination, so download and close errors leave
// the original intact. Existing regular files retain their access permissions;
// new files are private. Privilege bits and symlinks are not copied.
func Write(dst string, write func(io.Writer) error) error {
	mode := os.FileMode(0o600)
	info, err := os.Lstat(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	return WriteMode(dst, mode, write)
}

// WriteMode is Write with an explicit mode. It is used for credentials and
// journals, whose permissions must remain private even if an existing
// destination has permissive access bits.
func WriteMode(dst string, mode os.FileMode, write func(io.Writer) error) error {
	return stage(dst, mode, write, func(tmp string) error { return os.Rename(tmp, dst) })
}

// ErrExists is what WriteNew fails with when dst is already there.
var ErrExists = errors.New("destination already exists")

// WriteNew is Write for a destination that must not exist: it never replaces a file,
// including one that appears while the bytes are being written. The staged file is
// hard-linked into place, which fails rather than replaces; where the file system has
// no hard links it falls back to a check followed by a rename, which leaves a short
// window. A new file is private (0600).
func WriteNew(dst string, write func(io.Writer) error) error {
	return stage(dst, 0o600, write, func(tmp string) error {
		err := os.Link(tmp, dst)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, fs.ErrExist):
			return ErrExists
		}
		if _, serr := os.Lstat(dst); serr == nil {
			return ErrExists
		} else if !errors.Is(serr, fs.ErrNotExist) {
			return serr
		}
		return os.Rename(tmp, dst)
	})
}

// stage writes the bytes to a temporary file beside dst and hands its name to place,
// which moves it into position. The temporary file is removed whatever happens.
func stage(dst string, mode os.FileMode, write func(io.Writer) error, place func(tmp string) error) error {
	f, err := os.CreateTemp(filepath.Dir(dst), ".dd-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return place(f.Name())
}
