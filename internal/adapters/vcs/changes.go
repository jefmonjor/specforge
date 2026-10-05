package vcs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"specforge/internal/domain/change"
	"specforge/internal/domain/review"
	"specforge/internal/ports"
)

// Changes implements ports.VCS. Tracked paths are measured with
// `git diff --numstat` against HEAD (the empty tree before the first
// commit); untracked ones are counted as added lines.
func (g *Git) Changes(ctx context.Context, root string, paths []string) ([]change.File, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if res, err := g.git(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, err
	} else if !res.Success() {
		return nil, ports.ErrNotARepository
	}
	base, err := g.head(ctx, root)
	if err != nil {
		return nil, err
	}
	args := append([]string{"--literal-pathspecs", "diff", "--numstat", "--no-renames", "--no-color", "--no-ext-diff", base, "--"}, paths...)
	res, err := g.git(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, fmt.Errorf("git diff --numstat: %s", res.Combined())
	}
	files := parseNumstat(res.Stdout)
	for _, p := range paths {
		if slices.ContainsFunc(files, func(f change.File) bool { return f.Path == p }) {
			continue
		}
		if f, ok := untracked(root, p); ok {
			files = append(files, f)
		}
	}
	slices.SortFunc(files, func(a, b change.File) int { return strings.Compare(a.Path, b.Path) })
	return files, nil
}

// CommitChanges implements ports.VCS.
func (g *Git) CommitChanges(ctx context.Context, root, commit string) ([]change.File, error) {
	if err := validRef(commit); err != nil {
		return nil, err
	}
	res, err := g.git(ctx, root, "show", "--numstat", "--format=", "--no-renames", "--no-color", "--end-of-options", commit, "--")
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, fmt.Errorf("git show %s: %s", commit, res.Combined())
	}
	return parseNumstat(res.Stdout), nil
}

// head is HEAD, or the empty tree in a repository without commits.
func (g *Git) head(ctx context.Context, root string) (string, error) {
	if ok, err := g.isCommit(ctx, root, "HEAD"); err != nil || ok {
		return "HEAD", err
	}
	res, err := g.proc.Run(ctx, ports.Command{Name: "git", Args: []string{"-C", root, "hash-object", "-t", "tree", "--stdin"}, Stdin: strings.NewReader("")})
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("git hash-object: %s", res.Combined())
	}
	return strings.TrimSpace(res.Stdout), nil
}

// parseNumstat reads "added<TAB>deleted<TAB>path" lines; binary files
// show "-" for both counts.
func parseNumstat(out string) []change.File {
	var files []change.File
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		f := change.File{Path: filepath.ToSlash(parts[2])}
		if parts[0] == "-" && parts[1] == "-" {
			f.Binary = true
		} else {
			f.Added, _ = strconv.Atoi(parts[0])
			f.Deleted, _ = strconv.Atoi(parts[1])
		}
		files = append(files, f)
	}
	return files
}

// untracked measures a file git does not know yet: every line is added.
func untracked(root, rel string) (change.File, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return change.File{}, false
	}
	return change.FromContent(rel, data), true
}

// Patch implements ports.VCS.
func (g *Git) Patch(ctx context.Context, root string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	if res, err := g.git(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", err
	} else if !res.Success() {
		return "", ports.ErrNotARepository
	}
	base, err := g.head(ctx, root)
	if err != nil {
		return "", err
	}
	args := append([]string{"--literal-pathspecs", "diff", "--no-color", "--no-ext-diff", "--no-renames", base, "--"}, paths...)
	res, err := g.git(ctx, root, args...)
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("git diff: %s", res.Combined())
	}
	var b strings.Builder
	b.WriteString(res.Stdout)
	tracked, err := g.git(ctx, root, append([]string{"--literal-pathspecs", "ls-files", "--"}, paths...)...)
	if err != nil {
		return "", err
	}
	known := strings.Split(strings.TrimSpace(tracked.Stdout), "\n")
	for _, p := range paths {
		if slices.Contains(known, p) {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p))); err == nil {
			b.WriteString(review.NewFileDiff(p, data))
		}
	}
	return b.String(), nil
}
