package vcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// checkpointRefs is where checkpoints live: refs no branch, push or clone
// ever carries.
const checkpointRefs = "refs/specforge/checkpoints/"

// KeepCheckpoints is how many checkpoints are kept; older ones are
// dropped as new ones are saved.
var KeepCheckpoints = 50

var _ ports.Checkpoints = (*Git)(nil)

// Save implements ports.Checkpoints. The working tree goes through a
// private copy of the index (git add -A on it), so only what changed is
// hashed and the developer's index is untouched; the tree is committed on
// top of HEAD and referenced under refs/specforge/checkpoints/.
func (g *Git) Save(ctx context.Context, root, label string) (ports.Checkpoint, error) {
	if res, err := g.git(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return ports.Checkpoint{}, err
	} else if !res.Success() {
		return ports.Checkpoint{}, ports.ErrNotARepository
	}
	index, cleanup, err := g.privateIndex(ctx, root)
	if err != nil {
		return ports.Checkpoint{}, err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := g.must(ctx, root, env, "add", "-A", "--", "."); err != nil {
		return ports.Checkpoint{}, err
	}
	tree, err := g.must(ctx, root, env, "write-tree")
	if err != nil {
		return ports.Checkpoint{}, err
	}
	if last, ok, err := g.latest(ctx, root); err != nil {
		return ports.Checkpoint{}, err
	} else if ok && last.tree == tree {
		return last.Checkpoint, nil
	}
	args := []string{"commit-tree", tree, "-m", label}
	if ok, err := g.isCommit(ctx, root, "HEAD"); err != nil {
		return ports.Checkpoint{}, err
	} else if ok {
		args = append(args, "-p", "HEAD")
	}
	commit, err := g.must(ctx, root, identity, args...)
	if err != nil {
		return ports.Checkpoint{}, err
	}
	now := time.Now().UTC()
	id := now.Format("20060102T150405.000000000Z")
	if _, err := g.must(ctx, root, nil, "update-ref", checkpointRefs+id, commit); err != nil {
		return ports.Checkpoint{}, err
	}
	return ports.Checkpoint{ID: id, Label: label, At: now}, g.prune(ctx, root)
}

// List implements ports.Checkpoints.
func (g *Git) List(ctx context.Context, root string) ([]ports.Checkpoint, error) {
	all, err := g.checkpoints(ctx, root)
	out := make([]ports.Checkpoint, len(all))
	for i, c := range all {
		out[i] = c.Checkpoint
	}
	return out, err
}

// Restore implements ports.Checkpoints: git restore in overlay mode, from
// the checkpoint, of the project's directory, working tree only.
func (g *Git) Restore(ctx context.Context, root, id string) error {
	if strings.ContainsAny(id, " \t\n/\\:~^") || strings.HasPrefix(id, "-") || id == "" {
		return fmt.Errorf("invalid checkpoint %q", id)
	}
	ref := checkpointRefs + id
	if ok, err := g.isCommit(ctx, root, ref); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("no checkpoint %q: run `specforge restore` to list them", id)
	}
	_, err := g.must(ctx, root, nil, "restore", "--overlay", "--worktree", "--source="+ref, "--", ".")
	return err
}

type savedCheckpoint struct {
	ports.Checkpoint
	tree string
}

func (g *Git) checkpoints(ctx context.Context, root string) ([]savedCheckpoint, error) {
	text, err := g.must(ctx, root, nil, "for-each-ref", "--sort=-refname",
		"--format=%(refname)%09%(tree)%09%(subject)", checkpointRefs)
	if err != nil || text == "" {
		return nil, err
	}
	var out []savedCheckpoint
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		id := strings.TrimPrefix(parts[0], checkpointRefs)
		at, _ := time.Parse("20060102T150405.000000000Z", id)
		out = append(out, savedCheckpoint{Checkpoint: ports.Checkpoint{ID: id, Label: parts[2], At: at}, tree: parts[1]})
	}
	return out, nil
}

func (g *Git) latest(ctx context.Context, root string) (savedCheckpoint, bool, error) {
	all, err := g.checkpoints(ctx, root)
	if err != nil || len(all) == 0 {
		return savedCheckpoint{}, false, err
	}
	return all[0], true, nil
}

// prune drops all but the newest KeepCheckpoints.
func (g *Git) prune(ctx context.Context, root string) error {
	all, err := g.checkpoints(ctx, root)
	if err != nil || len(all) <= KeepCheckpoints {
		return err
	}
	for _, c := range all[KeepCheckpoints:] {
		if _, err := g.must(ctx, root, nil, "update-ref", "-d", checkpointRefs+c.ID); err != nil {
			return err
		}
	}
	return nil
}

// privateIndex copies the repository's index to a temporary file, so
// git add -A rehashes only what changed and the real index stays as it is.
func (g *Git) privateIndex(ctx context.Context, root string) (string, func(), error) {
	path, err := g.must(ctx, root, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp("", "specforge-index-*")
	if err != nil {
		return "", nil, err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	data, err := os.ReadFile(filepath.Clean(path))
	if err == nil {
		_, err = f.Write(data)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil // a repository without an index yet
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return tmp, cleanup, nil
}

// identity commits checkpoints as SpecForge, without the user's hooks or
// signing.
var identity = []string{"GIT_AUTHOR_NAME=SpecForge", "GIT_AUTHOR_EMAIL=specforge@localhost",
	"GIT_COMMITTER_NAME=SpecForge", "GIT_COMMITTER_EMAIL=specforge@localhost"}

// must runs git with env and returns its trimmed output, or an error with
// git's message.
func (g *Git) must(ctx context.Context, root string, env []string, args ...string) (string, error) {
	res, err := g.proc.Run(ctx, ports.Command{Name: "git", Args: append([]string{"-C", root}, args...), Env: env})
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(res.Combined()))
	}
	return strings.TrimSpace(res.Stdout), nil
}
