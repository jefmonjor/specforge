package fsys

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomicCreatesParentsAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "state.json")
	if err := WriteAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "two" {
		t.Fatalf("content = %q", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
	}
}

func TestAppendFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x", "log.md")
	fs := OS{}
	if err := fs.AppendFile(path, []byte("a\n")); err != nil {
		t.Fatal(err)
	}
	if err := fs.AppendFile(path, []byte("b\n")); err != nil {
		t.Fatal(err)
	}
	got, _ := fs.ReadFile(path)
	if string(got) != "a\nb\n" || !fs.Exists(path) || fs.Exists(path+".nope") {
		t.Fatalf("content = %q", got)
	}
}

// lockedFor makes the first n renames fail, as Windows does while another
// process holds the destination open.
func lockedFor(t *testing.T, n int) *int {
	t.Helper()
	calls := 0
	renameFile = func(from, to string) error {
		calls++
		if calls <= n {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: errors.New("access is denied")}
		}
		return os.Rename(from, to)
	}
	retryReplace = true
	t.Cleanup(func() { renameFile, retryReplace = os.Rename, runtime.GOOS == "windows" })
	return &calls
}

func TestWriteAtomicWaitsForALockedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := lockedFor(t, 2)
	if err := WriteAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "two" || *calls != 3 {
		t.Fatalf("content %q after %d renames", got, *calls)
	}
}

// The file is never deleted to make room: a lock that outlives the retries
// leaves the previous content in place, and no temporary file behind.
func TestWriteAtomicKeepsTheOriginalWhenStillLocked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := WriteAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockedFor(t, 1000)
	if err := WriteAtomic(path, []byte("two"), 0o600); err == nil {
		t.Fatal("a file locked for good must be an error")
	}
	if got, _ := os.ReadFile(path); string(got) != "one" {
		t.Fatalf("the original was lost: %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}
