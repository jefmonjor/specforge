// Package fsys implements ports.Files on the real filesystem with atomic
// writes: data goes to a temporary file in the same directory, is synced,
// and then renamed over the target, so a crash leaves either the old or the
// new content, never a truncated file.
package fsys

import (
	"fmt"
	"os"
	"path/filepath"

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
		f.Close()
		return fmt.Errorf("appending to %s: %w", path, err)
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
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	committed = true
	return nil
}
