// Package workspace observes a project's files so SpecForge can verify what
// an agent actually changed. Inside a git repository it hashes only the
// files git reports as changed or untracked; outside git it hashes every
// file, skipping dependency and build directories.
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// maxWalkedFiles bounds a snapshot outside git.
const maxWalkedFiles = 50_000

// deleted is the hash recorded for a path git reports but that is gone.
const deleted = "deleted"

// Git is a ports.Workspace that prefers git and falls back to walking.
type Git struct {
	proc ports.CommandRunner
}

var _ ports.Workspace = (*Git)(nil)

// New returns a workspace observer.
func New(proc ports.CommandRunner) *Git { return &Git{proc: proc} }

// Snapshot implements ports.Workspace.
func (g *Git) Snapshot(ctx context.Context, root string) (ports.Snapshot, error) {
	paths, ok, err := g.dirtyPaths(ctx, root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return walkAll(root)
	}
	snap := ports.Snapshot{}
	for _, p := range paths {
		if ignored(p) {
			continue
		}
		h, err := hashFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			h = deleted
		}
		snap[p] = h
	}
	return snap, nil
}

// HashFiles implements ports.Workspace.
func (g *Git) HashFiles(root string, match func(rel string) bool) (map[string]string, error) {
	out := map[string]string{}
	err := walk(root, func(rel, abs string) error {
		if !match(rel) {
			return nil
		}
		h, err := hashFile(abs)
		if err != nil {
			return err
		}
		out[rel] = h
		return nil
	})
	return out, err
}

// dirtyPaths lists changed and untracked files. ok is false when root is
// not inside a git work tree or git is not installed.
func (g *Git) dirtyPaths(ctx context.Context, root string) ([]string, bool, error) {
	res, err := g.proc.Run(ctx, ports.Command{
		Name: "git",
		Args: []string{"-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=all"},
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, err
		}
		return nil, false, nil // git missing: fall back to walking
	}
	if !res.Success() {
		return nil, false, nil // not a repository
	}
	return parsePorcelainZ(res.Stdout), true, nil
}

// parsePorcelainZ reads `git status --porcelain=v1 -z`. Each entry is
// "XY path\0"; renames and copies carry an extra "orig\0" after the path.
func parsePorcelainZ(out string) []string {
	var paths []string
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		status, path := f[:2], f[3:]
		paths = append(paths, path)
		if status[0] == 'R' || status[0] == 'C' {
			if i+1 < len(fields) && fields[i+1] != "" {
				paths = append(paths, fields[i+1])
			}
			i++
		}
	}
	return paths
}

func walkAll(root string) (ports.Snapshot, error) {
	snap := ports.Snapshot{}
	err := walk(root, func(rel, abs string) error {
		if len(snap) >= maxWalkedFiles {
			return fmt.Errorf("more than %d files outside git; initialise a git repository so SpecForge can track changes", maxWalkedFiles)
		}
		h, err := hashFile(abs)
		if err != nil {
			return err
		}
		snap[rel] = h
		return nil
	})
	return snap, err
}

func walk(root string, visit func(rel, abs string) error) error {
	return filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			if abs == root {
				return err
			}
			return nil // unreadable entry: skip it, as git would
		}
		if d.IsDir() {
			if abs != root && stack.IgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(rel), abs)
	})
}

func ignored(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if stack.IgnoredDirs[seg] {
			return true
		}
	}
	return false
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }() // read only: nothing to lose
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
