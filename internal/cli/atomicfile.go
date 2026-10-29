package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// writeFileAtomic stages bytes beside the destination, so download and close
// errors leave the original intact. Existing regular files retain their access
// permissions; new files are private. Privilege bits and symlinks are not copied.
func writeFileAtomic(dst string, write func(io.Writer) error) error {
	mode := os.FileMode(0o600)
	info, err := os.Lstat(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	return writeFileAtomicMode(dst, mode, write)
}

// Explicit mode is used for credentials and journals, whose permissions must
// remain private even if an existing destination has permissive access bits.
func writeFileAtomicMode(dst string, mode os.FileMode, write func(io.Writer) error) error {
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
	return os.Rename(f.Name(), dst)
}
