package scratch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"specforge/internal/domain/stack"
)

// linked are dependency directories the copies share with the project
// instead of copying them: they are large and the verifier only reads
// them.
var linked = []string{"node_modules", ".venv", "venv"}

// files lists what git would version (tracked and untracked, not
// ignored), or every file outside git, skipping dependency and build
// directories.
func (c *Copier) files(ctx context.Context, root string) ([]string, error) {
	text, err := c.output(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err == nil {
		var out []string
		for _, f := range strings.Split(text, "\x00") {
			if f != "" {
				out = append(out, f)
			}
		}
		return out, nil
	}
	if !errors.Is(err, errNotGit) {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && stack.IgnoredDirs[d.Name()] {
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

// copyFiles copies the regular files of root listed in files into dst.
func copyFiles(root, dst string, files []string) error {
	for _, rel := range files {
		src := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		if err != nil || !info.Mode().IsRegular() {
			continue // deleted in the working tree, or a link
		}
		if err := copyFile(src, filepath.Join(dst, filepath.FromSlash(rel)), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return os.MkdirAll(dst, 0o755)
}

// copyFile copies src to dst: a copy-on-write clone where the file system
// supports it, else the bytes.
func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if cloneFile(src, dst) {
		return os.Chmod(dst, perm)
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

// link shares the project's dependency directories with a copy: a
// symbolic link, or on Windows, without the right to create one, a
// directory junction; failing both there, the copy goes without them.
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
				_ = exec.Command("cmd", "/c", "mklink", "/J", target, src).Run()
				continue
			}
			return fmt.Errorf("linking %s into the copy: %w", name, err)
		}
	}
	return nil
}
