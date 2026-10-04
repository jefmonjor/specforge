package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/scaffold"
	"specforge/internal/app/setup"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
)

func (a *App) setupCommand() *cobra.Command {
	var (
		o                   config.Overrides
		agents              []string
		newKind, name, prev string
		javaRelease         int
	)
	c := &cobra.Command{
		Use:   "setup",
		Short: "Prepare this repository (your code is never touched)",
		Long: `setup writes three things and nothing else:

  specforge.yaml        project settings, shared by the team (kept if it exists)
  CLAUDE.md/GEMINI.md   a managed block with the working rules and the coding
                        standard of your stack; the rest of the file is yours
  .gitignore            adds .specforge/, SpecForge's local working state

Running it again only refreshes the managed block.

--new starts an empty project with the tooling SpecForge drives already in
place, then sets it up:

  java    Java 21 Maven: JUnit, AssertJ, ArchUnit layer rules, PMD
  react   Vite + React + TypeScript: Vitest, Testing Library, ESLint,
          Knip, jscpd, Stryker
  python  pyproject (src layout): pytest, ruff
  go      go.mod and golangci-lint

--legacy makes the project the rewrite of a legacy system: specforge.yaml
gets the migration section (see specforge legacy).`,
		Example: "  specforge setup\n  specforge setup --agents claude,gemini --stack go\n  specforge setup --new java --legacy ../payroll-legacy\n  specforge setup --new react --name shop",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.openProject(o, len(agents) == 0)
			if err != nil {
				return err
			}
			if len(agents) == 0 {
				agents = []string{p.settings.Agent}
			}
			con := a.console()
			con.Title(con.T("setup.title"))
			if newKind != "" {
				if name == "" {
					name = filepath.Base(p.root)
				}
				data, err := scaffold.NewData(name)
				if err != nil {
					return err
				}
				created, err := scaffold.Create(fsys.OS{}, p.root, newKind, data)
				for _, ch := range created {
					if ch.Action == scaffold.Created {
						con.OK(con.T("setup.created") + " · " + ch.Path)
					} else {
						con.Info(con.T("setup.unchanged") + " · " + ch.Path)
					}
				}
				if err != nil {
					return err
				}
				if newKind == "java" && javaRelease == 0 && prev != "" {
					javaRelease = 21
				}
			}

			var prof *stack.Profile
			if candidates := stack.Detect(os.DirFS(p.root)); len(candidates) > 0 || p.settings.Stack != "" {
				chosen, err := a.resolveStack(cmd.Context(), p)
				if err != nil {
					return err
				}
				prof = &chosen
			}

			changes, err := setup.Run(fsys.OS{}, setup.Options{Layout: p.layout, Language: p.settings.Language, Agents: agents, Stack: prof, Legacy: filepath.ToSlash(prev), JavaRelease: javaRelease})
			for _, ch := range changes {
				line := con.T("setup."+string(ch.Action)) + " · " + ch.Path
				if ch.Action == setup.Unchanged {
					con.Info(line)
				} else {
					con.OK(line)
				}
			}
			if err != nil {
				return err
			}
			if prof != nil {
				con.Info(con.T("setup.stack", prof.Name()))
			} else {
				con.Warn(con.T("setup.nostack"))
			}
			if newKind != "" {
				con.Info(con.T("setup.new." + newKind))
			}
			con.Info(con.T("setup.next"))
			return nil
		},
	}
	c.Flags().StringSliceVar(&agents, "agents", nil, "agents to write rules for (default: your configured agent)")
	c.Flags().StringVar(&o.Stack, "stack", "", "go | maven | gradle | node | python (default: detected)")
	c.Flags().StringVar(&newKind, "new", "", "start a new project: java | react | python | go")
	c.Flags().StringVar(&name, "name", "", "with --new: the project name (default: the directory name)")
	c.Flags().StringVar(&prev, "legacy", "", "the legacy repository this project rewrites (written to specforge.yaml)")
	c.Flags().IntVar(&javaRelease, "java-release", 0, "with --legacy: the Java release the rewrite targets (default 21 with --new java)")
	return c
}
