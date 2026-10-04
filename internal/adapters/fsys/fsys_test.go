package fsys

import (
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
