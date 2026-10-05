package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestSnapshotInGitSeesEditsAdditionsDeletionsAndRenames(t *testing.T) {
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	git(t, root, "init", "-q")
	write(t, root, "a.go", "a")
	write(t, root, "b.go", "b")
	write(t, root, "c.go", "c")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init")

	ws := New(process.NewRunner(nil))
	before, err := ws.Snapshot(context.Background(), root)
	if err != nil || len(before) != 0 {
		t.Fatalf("clean tree snapshot = %v, %v", before, err)
	}

	write(t, root, "a.go", "a2")
	write(t, root, "new_test.go", "t")
	write(t, root, "node_modules/x/index.js", "ignored")
	if err := os.Remove(filepath.Join(root, "b.go")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "mv", "c.go", "renamed.go")

	after, err := ws.Snapshot(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	got := before.Changed(after)
	want := []string{"a.go", "b.go", "c.go", "new_test.go", "renamed.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Changed = %v, want %v", got, want)
	}
}

func TestSnapshotOutsideGitWalksTheTree(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.go", "a")
	write(t, root, "vendor/x.go", "ignored")
	ws := New(process.NewRunner(nil))
	before, err := ws.Snapshot(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := before["vendor/x.go"]; ok || len(before) != 1 {
		t.Fatalf("snapshot = %v", before)
	}
	write(t, root, "a.go", "changed")
	after, _ := ws.Snapshot(context.Background(), root)
	if got := before.Changed(after); !reflect.DeepEqual(got, []string{"a.go"}) {
		t.Fatalf("Changed = %v", got)
	}
}

func TestHashFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pkg/a_test.go", "x")
	write(t, root, "pkg/a.go", "y")
	write(t, root, "node_modules/z_test.go", "ignored")
	got, err := New(process.NewRunner(nil)).HashFiles(root, func(rel string) bool { return strings.HasSuffix(rel, "_test.go") })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["pkg/a_test.go"] == "" {
		t.Fatalf("HashFiles = %v", got)
	}
}

func TestParsePorcelainZ(t *testing.T) {
	out := " M a.go\x00?? dir/b.go\x00R  new.go\x00old.go\x00 D gone.go\x00"
	want := []string{"a.go", "dir/b.go", "new.go", "old.go", "gone.go"}
	if got := parsePorcelainZ(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("parse = %v, want %v", got, want)
	}
}
