package scratch

import (
	"os"
	"path/filepath"
	"testing"
)

// The temporary directory of macOS is on APFS: a copy there is a clone.
func TestCopiesAreClonesOnAPFS(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	if err := os.WriteFile(src, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cloneFile(src, dst) {
		t.Fatal("APFS clones files copy-on-write")
	}
	if got, _ := os.ReadFile(dst); string(got) != "package a\n" {
		t.Fatalf("clone = %q", got)
	}
	if err := os.WriteFile(dst, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(src); string(got) != "package a\n" {
		t.Fatal("writing the clone never changes the original")
	}
}
