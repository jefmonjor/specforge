// Package scratch makes disposable copies of a project for the verifier:
// the working tree as it is, and the project at its base commit. Copies
// live in the system's temporary directory and are removed by the caller.
package scratch

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"path/filepath"
	"strings"

	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// linked are dependency directories the copies share with the project
// instead of copying them: they are large and the verifier only reads
// them.
var linked = []string{"node_modules", ".venv", "venv"}

// DefaultMaxBytes bounds a copy.
const DefaultMaxBytes = 500 << 20

// Copier is the ports.Scratch.
type Copier struct {
	proc     ports.CommandRunner
	maxBytes int64
}

var _ ports.Scratch = (*Copier)(nil)

// New returns a copier that refuses projects larger than maxBytes (the
// default when zero).
func New(proc ports.CommandRunner, maxBytes int64) *Copier {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Copier{proc: proc, maxBytes: maxBytes}
}

// Copy implements ports.Scratch.
func (c *Copier) Copy(ctx context.Context, root, base string) (ports.Copies, error) {
	dir, err := os.MkdirTemp("", "specforge-verify-*")
	if err != nil {
		return ports.Copies{}, err
	}
	out := ports.Copies{Dir: filepath.Join(dir, "work")}
	out.Remove = func() error { return os.RemoveAll(dir) }
	fail := func(err error) (ports.Copies, error) {
		return ports.Copies{}, errors.Join(err, out.Remove())
	}
	files, err := c.files(ctx, root)
	if err != nil {
		return fail(err)
	}
	if err := c.copyFiles(root, out.Dir, files); err != nil {
		return fail(err)
	}
	if err := link(root, out.Dir); err != nil {
		return fail(err)
	}
	if base != "" {
		baseDir := filepath.Join(dir, "base")
		ok, err := c.archive(ctx, root, base, baseDir)
		if err != nil {
			return fail(err)
		}
		if ok {
			out.BaseDir = baseDir
			if err := link(root, baseDir); err != nil {
				return fail(err)
			}
		}
	}
	return out, nil
}

// files lists what git would version (tracked and untracked, not
// ignored), or every file outside git, skipping dependency and build
// directories.
func (c *Copier) files(ctx context.Context, root string) ([]string, error) {
	res, err := c.proc.Run(ctx, ports.Command{Name: "git", Args: []string{"-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard"}})
	if err == nil && res.Success() {
		var out []string
		for _, f := range strings.Split(res.Stdout, "\x00") {
			if f != "" {
				out = append(out, f)
			}
		}
		return out, nil
	}
	if err != nil && !errors.Is(err, ports.ErrToolNotFound) {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (stack.IgnoredDirs[d.Name()] || d.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err == nil && d.Type().IsRegular() {
			out = append(out, filepath.ToSlash(rel))
		}
		return err
	})
	return out, err
}

func (c *Copier) copyFiles(root, dst string, files []string) error {
	var total int64
	for _, rel := range files {
		src := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		if err != nil || !info.Mode().IsRegular() {
			continue // deleted in the working tree, or a link
		}
		if total += info.Size(); total > c.maxBytes {
			return fmt.Errorf("%w: more than %d MB", ports.ErrTooLarge, c.maxBytes>>20)
		}
		if err := copyFile(src, filepath.Join(dst, filepath.FromSlash(rel)), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return os.MkdirAll(dst, 0o755)
}

func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(err, out.Close())
	}
	return out.Close()
}

// link shares the project's dependency directories with a copy.
func link(root, dst string) error {
	for _, name := range linked {
		src := filepath.Join(root, name)
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			continue
		}
		target := filepath.Join(dst, name)
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		if err := os.Symlink(src, target); err != nil {
			if runtime.GOOS == "windows" {
				c := exec.Command("cmd", "/c", "mklink", "/J", target, src)
				if c.Run() == nil {
					continue
				}
				continue
			}
			return fmt.Errorf("linking %s into the copy: %w", name, err)
		}
	}
	return nil
}

// archive extracts the project at commit base into dst. It reports false
// outside git or when base is not a commit.
func (c *Copier) archive(ctx context.Context, root, base, dst string) (bool, error) {
	if strings.HasPrefix(base, "-") {
		return false, fmt.Errorf("invalid git ref %q", base)
	}
	tarball := dst + ".tar"
	defer func() { _ = os.Remove(tarball) }()
	// -o: the archive can be larger than the output the runner keeps.
	res, err := c.proc.Run(ctx, ports.Command{Name: "git", Args: []string{"-C", root, "archive", "--format=tar", "-o", tarball, "--end-of-options", base}})
	if err != nil || !res.Success() {
		if err != nil && !errors.Is(err, ports.ErrToolNotFound) {
			return false, err
		}
		return false, nil
	}
	f, err := os.Open(tarball)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	return true, untar(f, dst)
}

// untar extracts regular files and directories, refusing any path that
// would leave dst.
func untar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return os.MkdirAll(dst, 0o755)
		}
		if err != nil {
			return fmt.Errorf("reading the base archive: %w", err)
		}
		target := filepath.Join(dst, filepath.FromSlash(h.Name))
		if rel, err := filepath.Rel(dst, target); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unsafe path in the base archive: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(h.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				return errors.Join(err, f.Close())
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

// Sandbox implements ports.Scratch.
func (c *Copier) Sandbox(ctx context.Context, root string) (ports.Copies, error) {
	copies, err := c.Copy(ctx, root, "")
	if err != nil {
		return copies, err
	}
	// A private identity, no hooks and no signing: this repository only
	// marks where the sandbox started.
	base := []string{"-C", copies.Dir, "-c", "user.name=SpecForge", "-c", "user.email=specforge@localhost",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "--allow-empty", "--no-verify", "-m", "sandbox"}} {
		res, err := c.proc.Run(ctx, ports.Command{Name: "git", Args: append(append([]string{}, base...), args...)})
		if err == nil && !res.Success() {
			err = fmt.Errorf("git %s in the sandbox: %s", args[0], res.Combined())
		}
		if err != nil {
			return ports.Copies{}, errors.Join(err, copies.Remove())
		}
	}
	return copies, nil
}
