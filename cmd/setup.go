package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/setup"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
)

func (a *App) setupCommand() *cobra.Command {
	var (
		o      config.Overrides
		agents []string
	)
	c := &cobra.Command{
		Use:   "setup",
		Short: "Prepare this repository (your code is never touched)",
		Long: `setup writes three things and nothing else:

  specforge.yaml        project settings, shared by the team (kept if it exists)
  CLAUDE.md/GEMINI.md   a managed block with the working rules and the coding
                        standard of your stack; the rest of the file is yours
  .gitignore            adds .specforge/, SpecForge's local working state

Running it again only refreshes the managed block.`,
		Example: "  specforge setup\n  specforge setup --agents claude,gemini --stack go",
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

			var prof *stack.Profile
			if candidates := stack.Detect(os.DirFS(p.root)); len(candidates) > 0 || p.settings.Stack != "" {
				chosen, err := a.resolveStack(cmd.Context(), p)
				if err != nil {
					return err
				}
				prof = &chosen
			}

			changes, err := setup.Run(fsys.OS{}, setup.Options{Layout: p.layout, Language: p.settings.Language, Agents: agents, Stack: prof})
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
			con.Info(con.T("setup.next"))
			return nil
		},
	}
	c.Flags().StringSliceVar(&agents, "agents", nil, "agents to write rules for (default: your configured agent)")
	c.Flags().StringVar(&o.Stack, "stack", "", "go | maven | gradle | node | python (default: detected)")
	return c
}
