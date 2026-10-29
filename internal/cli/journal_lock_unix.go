//go:build !windows

package cli

import (
	"errors"
	"os"
	"syscall"
)

func lockJournal(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
func unlockJournal(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
