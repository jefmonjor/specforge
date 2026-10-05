// Package scratch makes disposable copies of a project for the verifier
// and the parallel loop: the working tree as it is, and the project at its
// base commit. Copies live in the system's temporary directory and are
// removed by the caller.
//
// A copy costs what it writes, so each byte is written once at most:
//   - the size limit is checked before anything is written;
//   - files are cloned copy-on-write where the file system can (APFS,
//     Btrfs, XFS), which writes almost nothing;
//   - the base is checked out straight from git, without an archive;
//   - a sandbox's repository borrows the project's git objects, so its
//     first commit stores only what the project's git does not have.
package scratch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"specforge/internal/ports"
)

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
	out, _, err := c.copy(ctx, root, base)
	return out, err
}

// Sandbox implements ports.Scratch.
func (c *Copier) Sandbox(ctx context.Context, root string) (ports.Copies, error) {
	out, repo, err := c.copy(ctx, root, "")
	if err != nil {
		return out, err
	}
	if err := c.git(ctx, out.Dir, "init", "-q"); err != nil {
		return ports.Copies{}, errors.Join(fmt.Errorf("in the sandbox: %w", err), out.Remove())
	}
	if repo.objects != "" {
		// Borrowed, not copied: git add stores only the objects the
		// project's repository does not already hold.
		alternates := filepath.Join(out.Dir, ".git", "objects", "info", "alternates")
		if err := os.WriteFile(alternates, []byte(repo.objects+"\n"), 0o644); err != nil {
			return ports.Copies{}, errors.Join(err, out.Remove())
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "--allow-empty", "--no-verify", "-m", "sandbox"}} {
		if err := c.git(ctx, out.Dir, args...); err != nil {
			return ports.Copies{}, errors.Join(fmt.Errorf("in the sandbox: %w", err), out.Remove())
		}
	}
	return out, nil
}

// copy makes the copies and returns what it learnt of the project's git.
func (c *Copier) copy(ctx context.Context, root, base string) (ports.Copies, repository, error) {
	files, err := c.files(ctx, root)
	if err != nil {
		return ports.Copies{}, repository{}, err
	}
	if err := c.fits(root, files); err != nil {
		return ports.Copies{}, repository{}, err
	}
	repo, err := c.repository(ctx, root)
	if err != nil {
		return ports.Copies{}, repository{}, err
	}
	dir, err := os.MkdirTemp("", "specforge-verify-*")
	if err != nil {
		return ports.Copies{}, repository{}, err
	}
	out := ports.Copies{Dir: filepath.Join(dir, "work"), Remove: func() error { return os.RemoveAll(dir) }}
	fail := func(err error) (ports.Copies, repository, error) {
		return ports.Copies{}, repository{}, errors.Join(err, out.Remove())
	}
	if err := copyFiles(root, out.Dir, files); err != nil {
		return fail(err)
	}
	if err := link(root, out.Dir); err != nil {
		return fail(err)
	}
	if base != "" && repo.top != "" {
		baseDir := filepath.Join(dir, "base")
		ok, err := c.checkoutAt(ctx, repo, base, baseDir, filepath.Join(dir, "base.index"))
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
	return out, repo, nil
}

// fits refuses, before anything is written, a project whose files add up
// to more than the limit.
func (c *Copier) fits(root string, files []string) error {
	var total int64
	for _, rel := range files {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if total += info.Size(); total > c.maxBytes {
			return fmt.Errorf("%w: more than %d MB", ports.ErrTooLarge, c.maxBytes>>20)
		}
	}
	return nil
}
