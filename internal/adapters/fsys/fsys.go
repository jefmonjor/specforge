// Package fsys implements ports.Files on the real filesystem with atomic
// writes: data goes to a temporary file in the same directory, is synced,
// and then renamed over the target, so a crash leaves either the old or the
// new content, never a truncated file.
package fsys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"specforge/internal/ports"
)

// OS is the real filesystem.
type OS struct{}

var _ ports.Files = OS{}

// ReadFile implements ports.Files.
func (OS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// Exists implements ports.Files.
func (OS) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteFile implements ports.Files with mode 0644.
func (OS) WriteFile(path string, data []byte) error {
	return WriteAtomic(path, data, 0o644)
}

// WritePrivate implements ports.Files with mode 0600.
func (OS) WritePrivate(path string, data []byte) error {
	return WriteAtomic(path, data, 0o600)
}

// Remove implements ports.Files.
func (OS) Remove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AppendFile implements ports.Files.
func (OS) AppendFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return errors.Join(fmt.Errorf("appending to %s: %w", path, err), f.Close())
	}
	return f.Close()
}

// WriteAtomic replaces path with data and perm, creating parent directories.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName) // best effort: the error that got us here matters more
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), tmp.Close())
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(fmt.Errorf("syncing %s: %w", path, err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", path, err)
	}
	if err := replace(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	committed = true
	return nil
}

// renameFile is os.Rename, replaceable in tests.
var renameFile = os.Rename

// retryReplace is true where another process can lock a file against
// replacement (Windows); replaceable in tests.
var retryReplace = runtime.GOOS == "windows"

// replaceRetries bounds how long replace waits for another process.
var replaceRetries = []time.Duration{10, 20, 40, 80, 160, 320} // milliseconds

// replace renames tmp over path. On Windows os.Rename already replaces an
// existing file, but fails while another process (an antivirus, an editor,
// a search indexer) has path open; that lock is short-lived, so it retries
// for about half a second. The existing file is never deleted first: a
// failure leaves it intact, never a missing file.
func replace(tmp, path string) error {
	err := renameFile(tmp, path)
	if err == nil || !retryReplace {
		return err
	}
	for _, wait := range replaceRetries {
		time.Sleep(wait * time.Millisecond)
		if err = renameFile(tmp, path); err == nil {
			return nil
		}
	}
	return err
}
