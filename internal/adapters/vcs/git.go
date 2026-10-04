// Package vcs reads git for the audit. Every ref that comes from the
// command line is validated as a commit and passed after --end-of-options,
// so a value such as "--output=~/.bashrc" can never become a git option.
package vcs

import (
	"context"
	"fmt"
	"strings"

	"specforge/internal/ports"
)

// Git is the git-backed ports.VCS.
type Git struct {
	proc ports.CommandRunner
}

var _ ports.VCS = (*Git)(nil)

// New returns a git reader.
func New(proc ports.CommandRunner) *Git { return &Git{proc: proc} }

var defaultBases = []string{"origin/main", "origin/master", "main", "master"}

// DefaultBase implements ports.VCS.
func (g *Git) DefaultBase(ctx context.Context, root string) (string, error) {
	for _, b := range defaultBases {
		if ok, err := g.isCommit(ctx, root, b); err != nil {
			return "", err
		} else if ok {
			return b, nil
		}
	}
	return "", fmt.Errorf("no base branch found (tried %s): pass --base <ref>", strings.Join(defaultBases, ", "))
}

// Diff implements ports.VCS.
func (g *Git) Diff(ctx context.Context, root, base string) (string, error) {
	if err := validRef(base); err != nil {
		return "", err
	}
	ok, err := g.isCommit(ctx, root, base)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%q is not a commit in this repository", base)
	}
	res, err := g.git(ctx, root, "diff", "--no-color", "--no-ext-diff", "--end-of-options", base, "--")
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("git diff %s: %s", base, res.Combined())
	}
	return res.Stdout, nil
}

// Files implements ports.VCS.
func (g *Git) Files(ctx context.Context, root string) ([]string, error) {
	res, err := g.git(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, ports.ErrNotARepository
	}
	var files []string
	for _, f := range strings.Split(res.Stdout, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func (g *Git) isCommit(ctx context.Context, root, ref string) (bool, error) {
	res, err := g.git(ctx, root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return false, err
	}
	return res.Success(), nil
}

func (g *Git) git(ctx context.Context, root string, args ...string) (ports.CommandResult, error) {
	return g.proc.Run(ctx, ports.Command{Name: "git", Args: append([]string{"-C", root}, args...)})
}

// validRef rejects values git could read as options or that are not refs.
func validRef(ref string) error {
	switch {
	case ref == "":
		return fmt.Errorf("empty git ref")
	case strings.HasPrefix(ref, "-"):
		return fmt.Errorf("invalid git ref %q: refs cannot start with '-'", ref)
	case strings.ContainsAny(ref, " \t\n\x00"):
		return fmt.Errorf("invalid git ref %q", ref)
	}
	return nil
}
