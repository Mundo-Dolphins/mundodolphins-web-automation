//go:build unix

package filelock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockAcquiredAndReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.lock")
	lock := New(path)

	acquired, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("TryAcquire() acquired = false, want true")
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	again, err := New(path).TryAcquire()
	if err != nil {
		t.Fatalf("second TryAcquire() error = %v", err)
	}
	if !again {
		t.Fatal("second TryAcquire() acquired = false, want true")
	}
}

func TestSecondInstanceDoesNotAcquireAndDoesNotFail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.lock")
	first := New(path)
	acquired, err := first.TryAcquire()
	if err != nil {
		t.Fatalf("first TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("first TryAcquire() acquired = false, want true")
	}
	defer func() {
		if err := first.Release(); err != nil {
			t.Fatalf("Release() error = %v", err)
		}
	}()

	second, err := New(path).TryAcquire()
	if err != nil {
		t.Fatalf("second TryAcquire() error = %v", err)
	}
	if second {
		t.Fatal("second TryAcquire() acquired = true, want false")
	}
}

func TestTryAcquireDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "lock")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}
	if ok, err := New(link).TryAcquire(); ok || err == nil {
		t.Fatalf("expected the symlinked lock path to be refused, got ok=%v err=%v", ok, err)
	}
}
