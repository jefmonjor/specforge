package vcs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/domain/change"
	"specforge/internal/ports"
)

func TestChangesMeasuresTrackedAndUntrackedFiles(t *testing.T) {
	root := repo(t)
	_ = os.WriteFile(filepath.Join(root, "logo.png"), []byte{0x89, 'P', 'N', 'G', 0, 1, 2}, 0o644)
	g := New(process.NewRunner(nil))
	got, err := g.Changes(context.Background(), root, []string{"a.go", "new.go", "logo.png", "gone.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []change.File{
		{Path: "a.go", Added: 2},
		{Path: "logo.png", Binary: true},
		{Path: "new.go", Added: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes = %+v, want %+v", got, want)
	}
	if _, err := g.Changes(context.Background(), t.TempDir(), []string{"x"}); !errors.Is(err, ports.ErrNotARepository) {
		t.Fatalf("outside git: %v", err)
	}
}

func TestChangesBeforeTheFirstCommit(t *testing.T) {
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\nfunc A() {}"), 0o644)
	got, err := New(process.NewRunner(nil)).Changes(context.Background(), root, []string{"a.go"})
	if err != nil || len(got) != 1 || got[0].Added != 2 {
		t.Fatalf("Changes = %+v %v", got, err)
	}
}

func TestCommitChanges(t *testing.T) {
	root := repo(t)
	g := New(process.NewRunner(nil))
	got, err := g.CommitChanges(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := []change.File{{Path: ".gitignore", Added: 1}, {Path: "a.go", Added: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommitChanges = %+v, want %+v", got, want)
	}
	if _, err := g.CommitChanges(context.Background(), root, "--output=/tmp/x"); err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Fatalf("option injection: %v", err)
	}
}

func TestPatchIncludesUntrackedFiles(t *testing.T) {
	root := repo(t)
	g := New(process.NewRunner(nil))
	patch, err := g.Patch(context.Background(), root, []string{"a.go", "new.go"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"+++ b/a.go", "+func Login() {}", "new file mode 100644\n--- /dev/null\n+++ b/new.go", "+package a"} {
		if !strings.Contains(patch, want) {
			t.Errorf("missing %q in:\n%s", want, patch)
		}
	}
	if _, err := g.Patch(context.Background(), t.TempDir(), []string{"x"}); !errors.Is(err, ports.ErrNotARepository) {
		t.Fatalf("outside git: %v", err)
	}
}
