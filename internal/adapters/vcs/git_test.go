package vcs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/ports"
)

func repo(t *testing.T) string {
	t.Helper()
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0o644)
	run("add", ".")
	run("commit", "-qm", "init")
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc Login() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "new.go"), []byte("package a\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("x"), 0o644)
	return root
}

func TestDiffAndDefaultBase(t *testing.T) {
	root := repo(t)
	g := New(process.NewRunner(nil))
	base, err := g.DefaultBase(context.Background(), root)
	if err != nil || base != "main" {
		t.Fatalf("DefaultBase = %q %v", base, err)
	}
	diff, err := g.Diff(context.Background(), root, base)
	if err != nil || !strings.Contains(diff, "+func Login() {}") {
		t.Fatalf("Diff = %q %v", diff, err)
	}
}

func TestDiffRejectsOptionInjection(t *testing.T) {
	// Regression: --target was passed straight to `git diff`, so
	// "--output=<file>" overwrote an arbitrary file.
	root := repo(t)
	target := filepath.Join(t.TempDir(), "pwned")
	for _, ref := range []string{"--output=" + target, "-p", "", "main foo", "does-not-exist"} {
		if _, err := New(process.NewRunner(nil)).Diff(context.Background(), root, ref); err == nil {
			t.Errorf("ref %q must be rejected", ref)
		}
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("git wrote the injected output file")
	}
}

func TestFiles(t *testing.T) {
	root := repo(t)
	files, err := New(process.NewRunner(nil)).Files(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(files, ",")
	if !strings.Contains(got, "a.go") || !strings.Contains(got, "new.go") || strings.Contains(got, "ignored.txt") {
		t.Fatalf("Files = %v", files)
	}
	if _, err := New(process.NewRunner(nil)).Files(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a plain directory is not a repository")
	}
}

func TestCommitRecordsOnlyThePaths(t *testing.T) {
	root := repo(t)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "none"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@t"}, {"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@t"}} {
		t.Setenv(kv[0], kv[1])
	}
	g := New(process.NewRunner(nil))
	ctx := context.Background()
	// "[x].go" would be a glob without literal pathspecs.
	_ = os.WriteFile(filepath.Join(root, "[x].go"), []byte("package a\n"), 0o644)

	sha, err := g.Commit(ctx, root, "feat(SDD_0001_001): login", []string{"new.go", "[x].go"})
	if err != nil || len(sha) < 7 {
		t.Fatalf("Commit: %q %v", sha, err)
	}
	show := exec.Command("git", "-C", root, "show", "--name-only", "--format=%s", "HEAD")
	out, _ := show.Output()
	if got := strings.Fields(string(out)); strings.Join(got, " ") != "feat(SDD_0001_001): login [x].go new.go" {
		t.Fatalf("commit content: %q", out)
	}
	// a.go is still a pending change, not committed.
	status := exec.Command("git", "-C", root, "status", "--porcelain")
	st, _ := status.Output()
	if !strings.Contains(string(st), " M a.go") {
		t.Fatalf("status: %q", st)
	}
	// Nothing left to record for those paths.
	if sha, err := g.Commit(ctx, root, "again", []string{"new.go"}); sha != "" || err != nil {
		t.Fatalf("empty commit: %q %v", sha, err)
	}
	if _, err := g.Commit(ctx, t.TempDir(), "x", []string{"a"}); !errors.Is(err, ports.ErrNotARepository) {
		t.Fatalf("outside git: %v", err)
	}
}
