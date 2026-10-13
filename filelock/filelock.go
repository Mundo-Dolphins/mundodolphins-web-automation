//go:build unix

package filelock

import (
	"errors"
	"os"
	"syscall"
)

// Lock is an exclusive advisory lock on a file (flock). It is Unix-only. The
// lock file is never opened through a symlink, so a predictable path in a
// shared directory such as /tmp cannot be redirected.
type Lock struct {
	path string
	file *os.File
}

func New(path string) *Lock {
	return &Lock{path: path}
}

func (l *Lock) TryAcquire() (bool, error) {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return false, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return false, nil
		}
		return false, err
	}

	l.file = file
	return true, nil
}

func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}

	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
