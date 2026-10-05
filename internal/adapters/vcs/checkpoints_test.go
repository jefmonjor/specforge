package vcs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/ports"
)

func gitIn(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "<missing>"
	}
	return string(data)
}

// An agent that gets past the guard (here git itself, run directly)
// destroys uncommitted and untracked work; the checkpoint brings it back
// and leaves what was created since.
func TestACheckpointBringsBackWhatAnAgentDestroyed(t *testing.T) {
	root := repo(t) // a.go modified, new.go untracked, ignored.txt ignored
	gitIn(t, root, "add", "a.go")
	statusBefore := gitIn(t, root, "status", "--porcelain")
	g := New(process.NewRunner(nil))
	ctx := context.Background()

	cp, err := g.Save(ctx, root, "0001 scenario 1 GREEN")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, root, "status", "--porcelain"); got != statusBefore {
		t.Fatalf("a checkpoint changes nothing the developer sees:\n%s\nwas\n%s", got, statusBefore)
	}
	if branches := gitIn(t, root, "branch", "--list"); branches != "* main\n" {
		t.Fatalf("no branch is created: %q", branches)
	}

	gitIn(t, root, "reset", "--hard")
	gitIn(t, root, "clean", "-fdx")
	_ = os.WriteFile(filepath.Join(root, "later.go"), []byte("package a // written after\n"), 0o644)
	if read(t, root, "new.go") != "<missing>" {
		t.Fatal("the work is gone")
	}

	if err := g.Restore(ctx, root, cp.ID); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "a.go") != "package a\n\nfunc Login() {}\n" || read(t, root, "new.go") != "package a\n" {
		t.Fatalf("uncommitted and untracked work is back: a.go=%q new.go=%q", read(t, root, "a.go"), read(t, root, "new.go"))
	}
	if read(t, root, "later.go") == "<missing>" {
		t.Fatal("a file created since is never deleted")
	}
	if read(t, root, "ignored.txt") != "<missing>" {
		t.Fatal("ignored files are not in checkpoints")
	}
}

func TestCheckpointsAreListedNewestFirstAndNotRepeated(t *testing.T) {
	root := repo(t)
	g := New(process.NewRunner(nil))
	ctx := context.Background()
	first, err := g.Save(ctx, root, "one")
	if err != nil {
		t.Fatal(err)
	}
	same, err := g.Save(ctx, root, "nothing changed")
	if err != nil || same.ID != first.ID {
		t.Fatalf("an unchanged tree reuses the last checkpoint: %+v %v", same, err)
	}
	_ = os.WriteFile(filepath.Join(root, "b.go"), []byte("package a\n"), 0o644)
	second, err := g.Save(ctx, root, "two")
	if err != nil {
		t.Fatal(err)
	}
	list, err := g.List(ctx, root)
	if err != nil || len(list) != 2 || list[0].ID != second.ID || list[0].Label != "two" || list[1].Label != "one" || list[0].At.IsZero() {
		t.Fatalf("list = %+v %v", list, err)
	}
}

func TestOnlyTheNewestCheckpointsAreKept(t *testing.T) {
	root := repo(t)
	g := New(process.NewRunner(nil))
	defer func(n int) { KeepCheckpoints = n }(KeepCheckpoints)
	KeepCheckpoints = 3
	for i := range 5 {
		_ = os.WriteFile(filepath.Join(root, "n.go"), []byte{byte('a' + i)}, 0o644)
		if _, err := g.Save(context.Background(), root, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if list, _ := g.List(context.Background(), root); len(list) != 3 {
		t.Fatalf("kept %d", len(list))
	}
}

func TestCheckpointsOutsideGitAndBadIDs(t *testing.T) {
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	g := New(process.NewRunner(nil))
	if _, err := g.Save(context.Background(), t.TempDir(), "x"); !errors.Is(err, ports.ErrNotARepository) {
		t.Fatalf("want ErrNotARepository, got %v", err)
	}
	root := repo(t)
	for _, id := range []string{"", "--source=x", "../heads/main", "nope"} {
		if err := g.Restore(context.Background(), root, id); err == nil {
			t.Errorf("Restore(%q) must fail", id)
		}
	}
}
