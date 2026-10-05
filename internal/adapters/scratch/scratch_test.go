package scratch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/ports"
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

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "g"),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestCopyTheWorkingTreeAndTheBase(t *testing.T) {
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	write(t, root, ".gitignore", "secret.env\nnode_modules/\n")
	write(t, root, "pay/net.go", "package pay // v1\n")
	git(t, root, "init", "-q")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "base")
	write(t, root, "pay/net.go", "package pay // v2\n")
	write(t, root, "pay/bonus.go", "package pay\n")
	write(t, root, "secret.env", "TOKEN=x\n")
	write(t, root, "node_modules/lib/index.js", "x\n")

	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	read := func(dir, rel string) string {
		data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		return string(data)
	}
	if read(c.Dir, "pay/net.go") != "package pay // v2\n" || read(c.Dir, "pay/bonus.go") == "" {
		t.Fatal("the copy holds the working tree")
	}
	if read(c.Dir, "secret.env") != "" {
		t.Fatal("ignored files are not copied")
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Lstat(filepath.Join(c.Dir, "node_modules")); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("dependencies are linked, not copied: %v", err)
		}
	}
	if read(c.BaseDir, "pay/net.go") != "package pay // v1\n" || read(c.BaseDir, "pay/bonus.go") != "" {
		t.Fatal("the base copy holds the last commit")
	}
	if err := c.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.Dir); !os.IsNotExist(err) {
		t.Fatal("Remove deletes the copies")
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "lib", "index.js")); err != nil {
		t.Fatal("removing a copy never touches the project's dependencies")
	}
}

func TestCopyOutsideGitAndTheSizeLimit(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.py", "print(1)\n")
	write(t, root, "node_modules/x.js", "x")
	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	if _, err := os.Stat(filepath.Join(c.Dir, "a.py")); err != nil || c.BaseDir != "" {
		t.Fatalf("outside git: every file, no base: %v %q", err, c.BaseDir)
	}
	write(t, root, "big.bin", string(make([]byte, 2048)))
	if _, err := New(process.NewRunner(nil), 1024).Copy(context.Background(), root, ""); !errors.Is(err, ports.ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

func TestSandboxIsARepositoryOfTheCopy(t *testing.T) {
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	write(t, root, "a.go", "package a\n")
	c, err := New(process.NewRunner(nil), 0).Sandbox(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	out, err := exec.Command("git", "-C", c.Dir, "status", "--porcelain").CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Fatalf("a clean repository at the copy: %v %q", err, out)
	}
	write(t, c.Dir, "a.go", "package a // changed\n")
	if out, _ := exec.Command("git", "-C", c.Dir, "status", "--porcelain").CombinedOutput(); string(out) != " M a.go\n" {
		t.Fatalf("changes in the sandbox are measured from the copy: %q", out)
	}
}
