package scratch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/ports"
)

// repository is what a copy needs to know about the project's git.
type repository struct {
	// top is the repository's root ("" outside git); prefix is the
	// project's directory inside it ("" when the project is the root).
	top, prefix string
	// objects is the repository's object directory, which a sandbox
	// borrows ("" outside git).
	objects string
}

// repository inspects root. Outside git, or without git, it is empty.
func (c *Copier) repository(ctx context.Context, root string) (repository, error) {
	top, err := c.output(ctx, root, "rev-parse", "--show-toplevel")
	if errors.Is(err, errNotGit) {
		return repository{}, nil
	}
	if err != nil {
		return repository{}, err
	}
	r := repository{top: top}
	if r.prefix, err = c.output(ctx, root, "rev-parse", "--show-prefix"); err != nil {
		return repository{}, err
	}
	r.prefix = strings.TrimSuffix(r.prefix, "/")
	objects, err := c.output(ctx, root, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err == nil && filepath.IsAbs(objects) {
		r.objects = objects
	} else if err != nil && !errors.Is(err, errNotGit) {
		return repository{}, err
	}
	return r, nil
}

// checkoutAt writes the project as it is at commit ref into dst, straight
// from git through a private index: the project's own index and working
// tree are never touched. It reports false when ref is not a commit.
func (c *Copier) checkoutAt(ctx context.Context, r repository, ref, dst, index string) (bool, error) {
	if strings.HasPrefix(ref, "-") {
		return false, fmt.Errorf("invalid git ref %q", ref)
	}
	commit, err := c.output(ctx, r.top, "rev-parse", "--verify", "-q", "--end-of-options", ref+"^{commit}")
	if errors.Is(err, errNotGit) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tree := commit + "^{tree}"
	if r.prefix != "" {
		tree = commit + ":" + r.prefix // only the project's directory
	}
	env := append([]string{"GIT_INDEX_FILE=" + index}, gitEnv...)
	for _, args := range [][]string{
		{"read-tree", tree},
		{"checkout-index", "-a", "--prefix=" + filepath.ToSlash(dst) + "/"},
	} {
		if err := c.run(ctx, r.top, env, args[0], args); err != nil {
			return false, err
		}
	}
	return true, os.MkdirAll(dst, 0o755)
}

// errNotGit is a git command that failed: outside a repository, an
// unknown ref, or git missing.
var errNotGit = errors.New("not a git repository")

// gitEnv keeps git local and quiet: no LFS download, no credential
// prompt.
var gitEnv = []string{"GIT_LFS_SKIP_SMUDGE=1", "GIT_TERMINAL_PROMPT=0"}

// gitIdentity lets the sandbox commit without the user's identity, hooks
// or signing: its commit only marks where the sandbox started.
var gitIdentity = []string{"-c", "user.name=SpecForge", "-c", "user.email=specforge@localhost",
	"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull}

// git runs a git command in dir with SpecForge's identity.
func (c *Copier) git(ctx context.Context, dir string, args ...string) error {
	return c.run(ctx, dir, gitEnv, args[0], append(append([]string{}, gitIdentity...), args...))
}

// run runs git with args in dir; name is the subcommand, for the error.
func (c *Copier) run(ctx context.Context, dir string, env []string, name string, args []string) error {
	res, err := c.proc.Run(ctx, ports.Command{Name: "git", Args: append([]string{"-C", dir}, args...), Env: env})
	if err != nil {
		return err
	}
	if !res.Success() {
		return fmt.Errorf("git %s: %s", name, strings.TrimSpace(res.Combined()))
	}
	return nil
}

// output runs a read-only git command in dir and returns its output
// without the final newline; errNotGit when git is missing or the command
// fails.
func (c *Copier) output(ctx context.Context, dir string, args ...string) (string, error) {
	res, err := c.proc.Run(ctx, ports.Command{Name: "git", Args: append([]string{"-C", dir}, args...), Env: gitEnv})
	if errors.Is(err, ports.ErrToolNotFound) {
		return "", errNotGit
	}
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", errNotGit
	}
	return strings.TrimRight(res.Stdout, "\n"), nil
}
