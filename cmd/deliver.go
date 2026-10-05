package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/vcs"
	"specforge/internal/app/deliver"
	"specforge/internal/config"
	"specforge/internal/domain/delivery"
	"specforge/internal/domain/tdd"
)

func (a *App) deliverCommand() *cobra.Command {
	var slices bool
	c := &cobra.Command{
		Use:   "deliver [spec]",
		Short: "Write the hand-over: DELIVERY.md, trace.json and PR_BODY.md (gate R4)",
		Long: `deliver gathers what SpecForge recorded for an approved specification into
specs/NNNN-slug/:

  DELIVERY.md   for people: approvals, one row per scenario (tests, commit,
                gates, review notes), decisions, open questions, lessons,
                audit and E2E results, and what is out of scope
  trace.json    the same, for tools
  PR_BODY.md    ready for the pull request; it keeps the repository's PR
                template when there is one

Each scenario's size is counted from its commit (lock files excluded).
Above delivery.budget_lines (400 by default) DELIVERY.md proposes slices:
consecutive scenarios that fit the budget, each with the command to create
its branch; --slices also writes PR_BODY-1.md … PR_BODY-n.md. A scenario is
never cut in half.

Nothing comes from the agent: only from the sealed documents, the loop
state, the decisions log and the reports. What is not done is said first.`,
		Example: "  specforge deliver 0001\n  gh pr create --body-file specs/0001-password-reset/PR_BODY.md",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			entry, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			o := deliver.Options{Root: p.root, SpecPath: entry.Path, Language: p.settings.Language, Now: a.Now(),
				Budget: p.settings.BudgetLines, SliceBodies: slices}
			if prof, err := a.resolveStack(ctx, p); err == nil {
				o.Profile = &prof
			} else if !errors.Is(err, tdd.ErrUnsupportedStack) {
				return err // ambiguous: never guess
			}
			files := fsys.OS{}
			trace, err := deliver.Build(ctx, files, vcs.New(a.NewProcess(a.log)), o)
			if err != nil {
				return err
			}
			paths, err := deliver.Write(files, o, trace)
			if err != nil {
				return err
			}
			var rel []string
			for _, path := range paths {
				rel = append(rel, p.layout.Rel(path))
			}
			if ok, err := a.emit(map[string]any{"complete": trace.Complete(), "files": rel, "trace": trace}); ok {
				return err
			}
			con := a.console()
			finished := trace.Count(delivery.Done) + trace.Count(delivery.Satisfied)
			if trace.Complete() {
				con.OK(con.T("deliver.complete", finished, len(trace.Scenarios)))
			} else {
				con.Warn(con.T("deliver.incomplete", finished, len(trace.Scenarios), len(trace.Pending)))
			}
			if len(trace.Slices) > 0 {
				con.Warn(con.T("deliver.slices", trace.Lines(), trace.Budget, len(trace.Slices)))
			}
			for _, path := range rel {
				con.Data(path)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&slices, "slices", false, "also write one PR_BODY-n.md per proposed slice")
	return c
}
