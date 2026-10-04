package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/layout"
	"specforge/internal/app/specs"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// project is the repository a command works on, with its settings.
type project struct {
	root     string
	layout   layout.Layout
	settings config.Settings
}

// openProject finds the project root and resolves the settings. The root
// is the nearest directory with specforge.yaml, else the nearest git
// repository, else the working directory.
func (a *App) openProject(o config.Overrides, requireAgent bool) (project, error) {
	wd, err := a.Getwd()
	if err != nil {
		return project{}, err
	}
	root := findRoot(wd)
	dirs, err := a.UserDirs()
	if err != nil {
		return project{}, err
	}
	u, err := config.LoadUser(dirs)
	if err != nil {
		return project{}, err
	}
	p, err := config.LoadProject(root)
	if err != nil {
		return project{}, err
	}
	s, err := config.Resolve(u, p, o, requireAgent)
	if s.Language != "" && err == nil {
		a.lang = s.Language
	}
	if err != nil {
		return project{}, err
	}
	a.log.Debug("project", "root", root, "agent", s.Agent, "language", s.Language, "stack", s.Stack)
	return project{root: root, layout: layout.Layout{Root: root}, settings: s}, nil
}

func findRoot(wd string) string {
	for _, marker := range []string{config.ProjectFile, ".git"} {
		for dir := wd; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return wd
}

func (p project) specs(a *App) specs.Service {
	return specs.Service{Layout: p.layout, FS: os.DirFS(p.root), Files: fsys.OS{}, Now: a.Now, Language: p.settings.Language}
}

// resolveSpec finds the specification arg names. When it is ambiguous and
// someone is at the terminal, it asks instead of picking one.
func (a *App) resolveSpec(ctx context.Context, p project, arg string) (specs.Entry, error) {
	svc := p.specs(a)
	e, err := svc.Resolve(arg)
	var amb *specs.AmbiguousError
	if !errors.As(err, &amb) || !a.canAsk() {
		return e, err
	}
	answer, err := a.prompter().Ask(ctx, ports.Question{Text: a.console().T("ask.spec"), Options: amb.Candidates})
	if err != nil {
		return specs.Entry{}, err
	}
	return svc.Resolve(answer)
}

// resolveStack returns the stack to drive: the configured one if the
// project has it, the only one detected, or the one the developer picks.
func (a *App) resolveStack(ctx context.Context, p project) (stack.Profile, error) {
	candidates := stack.Detect(os.DirFS(p.root))
	if name := p.settings.Stack; name != "" {
		if prof, ok := stack.ByName(candidates, name); ok {
			return prof, nil
		}
		return stack.Profile{}, fmt.Errorf("%w: stack %q is configured but its build file is not at %s", tdd.ErrUnsupportedStack, name, p.root)
	}
	switch len(candidates) {
	case 0:
		return stack.Profile{}, fmt.Errorf("%w: no go.mod, pom.xml, build.gradle, package.json or pyproject.toml at %s", tdd.ErrUnsupportedStack, p.root)
	case 1:
		return candidates[0], nil
	}
	var names []string
	for _, c := range candidates {
		names = append(names, string(c.Kind))
	}
	if !a.canAsk() {
		return stack.Profile{}, fmt.Errorf("several stacks detected (%s): set `stack:` in specforge.yaml or pass --stack", strings.Join(names, ", "))
	}
	answer, err := a.prompter().Ask(ctx, ports.Question{Text: a.console().T("ask.stack"), Options: names})
	if err != nil {
		return stack.Profile{}, err
	}
	if prof, ok := stack.ByName(candidates, answer); ok {
		return prof, nil
	}
	return stack.Profile{}, fmt.Errorf("%q is not one of the detected stacks (%s)", answer, strings.Join(names, ", "))
}

// listFiles walks root for the audit when it is not a git repository.
func listFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (stack.IgnoredDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out, err
}
