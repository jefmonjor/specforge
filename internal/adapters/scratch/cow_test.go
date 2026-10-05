package scratch

import (
	"os"
	"path/filepath"
	"testing"
)

// Whether or not the file system clones, a copy is independent of its
// original.
func TestACopyIsIndependentOfItsOriginal(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.sh"), filepath.Join(dir, "sub", "b.sh")
	if err := os.WriteFile(src, []byte("echo a\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("echo b\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(src); string(got) != "echo a\n" {
		t.Fatalf("original = %q", got)
	}
	if info, err := os.Stat(dst); err != nil || (info.Mode().Perm()&0o100 == 0 && os.PathSeparator == '/') {
		t.Fatalf("the copy keeps its mode: %v", err)
	}
}
