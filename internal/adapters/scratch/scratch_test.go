package scratch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	git(t, root, "config", "core.autocrlf", "false") // what was committed is what is checked out, on every OS
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

// committed makes a repository at root with files committed.
func committed(t *testing.T, root string, files map[string]string) {
	t.Helper()
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	for rel, content := range files {
		write(t, root, rel, content)
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "core.autocrlf", "false") // what was committed is what is checked out, on every OS
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "base")
}

func exists(dir, rel string) bool {
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

func TestTheCopyIsTheWorkingTreeAndTheBaseTheCommit(t *testing.T) {
	root := t.TempDir()
	committed(t, root, map[string]string{"keep.go": "package a\n", "gone.go": "package a\n", "staged.go": "package a // v1\n"})
	if err := os.Remove(filepath.Join(root, "gone.go")); err != nil {
		t.Fatal(err)
	}
	write(t, root, "staged.go", "package a // v2\n")
	git(t, root, "add", "staged.go")
	write(t, root, "new/new.go", "package new\n")

	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	got, _ := os.ReadFile(filepath.Join(c.Dir, "staged.go"))
	if exists(c.Dir, "gone.go") || !exists(c.Dir, "new/new.go") || !exists(c.Dir, "keep.go") || string(got) != "package a // v2\n" {
		t.Fatalf("the copy is the working tree: gone=%v new=%v staged=%q", exists(c.Dir, "gone.go"), exists(c.Dir, "new/new.go"), got)
	}
	if !exists(c.BaseDir, "gone.go") || exists(c.BaseDir, "new/new.go") {
		t.Fatal("the base is the commit")
	}
	if out, _ := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput(); string(out) != "D  gone.go\nM  staged.go\n?? new/\n" && string(out) != " D gone.go\nM  staged.go\n?? new/\n" {
		t.Fatalf("the project is untouched: %q", out)
	}
}

func TestAProjectInsideALargerRepositoryIsCopiedAlone(t *testing.T) {
	top := t.TempDir()
	committed(t, top, map[string]string{"service/pay.go": "package pay // v1\n", "other/huge.go": "package other\n"})
	write(t, top, "service/pay.go", "package pay // v2\n")
	root := filepath.Join(top, "service")
	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	work, _ := os.ReadFile(filepath.Join(c.Dir, "pay.go"))
	base, _ := os.ReadFile(filepath.Join(c.BaseDir, "pay.go"))
	if string(work) != "package pay // v2\n" || string(base) != "package pay // v1\n" {
		t.Fatalf("work %q, base %q", work, base)
	}
	if exists(c.Dir, "other/huge.go") || exists(c.BaseDir, "other/huge.go") || exists(c.BaseDir, "service") {
		t.Fatal("only the project's directory is copied, at the copy's root")
	}
}

func TestASandboxBorrowsTheProjectsGitObjects(t *testing.T) {
	root := t.TempDir()
	committed(t, root, map[string]string{"a.go": "package a\n", "b.go": "package b\n"})
	write(t, root, "c.go", "package c\n")
	c, err := New(process.NewRunner(nil), 0).Sandbox(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	alternates, _ := os.ReadFile(filepath.Join(c.Dir, ".git", "objects", "info", "alternates"))
	if !strings.Contains(filepath.ToSlash(string(alternates)), filepath.ToSlash(filepath.Join(filepath.Base(root), ".git", "objects"))) {
		t.Fatalf("the sandbox borrows the project's objects: %q", alternates)
	}
	// a.go and b.go are already in the project: only c.go's blob, the
	// trees and the commit are stored in the sandbox.
	var loose int
	_ = filepath.WalkDir(filepath.Join(c.Dir, ".git", "objects"), func(p string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() && len(filepath.Base(filepath.Dir(p))) == 2 {
			loose++
		}
		return nil
	})
	if loose != 3 {
		t.Fatalf("objects stored in the sandbox = %d, want 3 (c.go, the tree, the commit)", loose)
	}
	if out, _ := exec.Command("git", "-C", c.Dir, "status", "--porcelain").CombinedOutput(); len(out) != 0 {
		t.Fatalf("the sandbox starts clean: %q", out)
	}
}

func TestFilesInGitLFSKeepTheirContent(t *testing.T) {
	root := t.TempDir()
	committed(t, root, map[string]string{".gitattributes": "*.bin filter=lfs diff=lfs merge=lfs -text\n", "a.go": "package a\n"})
	write(t, root, "model.bin", "real content, not a pointer")
	git(t, root, "-c", "filter.lfs.clean=cat", "-c", "filter.lfs.smudge=cat", "add", "model.bin")
	git(t, root, "-c", "filter.lfs.clean=cat", "-c", "filter.lfs.smudge=cat", "commit", "-qm", "lfs")
	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	if got, _ := os.ReadFile(filepath.Join(c.Dir, "model.bin")); string(got) != "real content, not a pointer" {
		t.Fatalf("LFS files hold their content: %q", got)
	}
}

func TestATooLargeProjectIsRefusedBeforeAnyCopy(t *testing.T) {
	root := t.TempDir()
	committed(t, root, map[string]string{"big.bin": string(make([]byte, 4096))})
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	if _, err := New(process.NewRunner(nil), 1024).Sandbox(context.Background(), root); !errors.Is(err, ports.ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("nothing was written: %v", left)
	}
}

func TestAnUnknownBaseGivesNoBaseCopy(t *testing.T) {
	root := t.TempDir()
	committed(t, root, map[string]string{"a.go": "package a\n"})
	c, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "no-such-branch")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Remove() }()
	if c.BaseDir != "" || !exists(c.Dir, "a.go") {
		t.Fatalf("base %q", c.BaseDir)
	}
	if _, err := New(process.NewRunner(nil), 0).Copy(context.Background(), root, "--output=x"); err == nil {
		t.Fatal("an option is never taken as a ref")
	}
}
