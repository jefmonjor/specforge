package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/vcs"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/planning"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
	"specforge/internal/ui"
)

func (a *App) planCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "plan [spec]",
		Short: "Draft the technical plan of an approved specification (review gate R1)",
		Long: `plan asks your agent where the code goes before any code exists: the
approach, the files to create or change, one planned test per scenario, the
interfaces and the risks. It writes specs/NNNN-slug/plan.md and nothing
else; SpecForge rejects a draft that touches other files or leaves a
scenario without a planned test. The agent asks when an architectural
choice is not settled.

Review the plan, edit it if you like, then approve it with plan approve.
When plan.md exists the loop requires it approved and follows it.`,
		Example: "  specforge plan 0001\n  specforge plan approve 0001",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			entry, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			svc := p.specs(a)
			doc, err := svc.Approved(entry)
			if err != nil {
				return err
			}
			text, err := os.ReadFile(entry.Path)
			if err != nil {
				return err
			}
			var prof *stack.Profile
			if chosen, err := a.resolveStack(ctx, p); err == nil {
				prof = &chosen
			} else if !errors.Is(err, tdd.ErrUnsupportedStack) {
				return err // ambiguous: never guess
			}

			legacyDir, err := a.rewriteLegacy(p)
			if err != nil {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("plan.title", entry.Title))
			events := &ui.PlanEvents{C: con, Agent: ag.Name()}
			defer events.Done()
			files := fsys.OS{}
			git := vcs.New(proc)
			path, err := planning.Draft(ctx, planning.Deps{
				Agent:     ag,
				Workspace: workspace.New(proc),
				Files:     files,
				Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Lister: func(ctx context.Context, root string) ([]string, error) {
					list, err := git.Files(ctx, root)
					if errors.Is(err, ports.ErrNotARepository) {
						return listFiles(root)
					}
					return list, err
				},
				Events: events,
				Log:    a.log,
			}, planning.Options{
				Root: p.root, SpecPath: entry.Path, SpecID: entry.ID, Doc: doc, SpecText: string(text),
				Profile: prof, Language: p.settings.Language, Model: p.settings.ModelFor("plan"),
				AgentTimeout: p.settings.AgentTimeout, MaxAttempts: p.settings.MaxAttempts,
				Legacy: legacyDir, JavaRelease: p.settings.Migration.JavaRelease, ForbiddenImports: p.settings.Migration.ForbiddenImports,
			})
			events.Done()
			if err != nil {
				return err
			}
			con.OK(con.T("plan.written", p.layout.Rel(path)))
			con.Info(con.T("plan.next", entry.ID))
			con.Data(p.layout.Rel(path))
			return nil
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	c.Flags().StringVar(&o.Stack, "stack", "", "go | maven | gradle | node | python (default: specforge.yaml or detected)")
	c.AddCommand(a.planApproveCommand())
	return c
}

func (a *App) planApproveCommand() *cobra.Command {
	var by string
	c := &cobra.Command{
		Use:   "approve [spec]",
		Short: "Approve and seal the plan (review gate R1)",
		Long: `approve checks that the plan places a test for every scenario and leaves
no TODO, records the approver and seals it. Approving an edited plan again
accepts the change.`,
		Args: cobra.MaximumNArgs(1),
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
			approver, err := a.approver(ctx, p.root, by)
			if err != nil {
				return err
			}
			res, err := p.specs(a).ApprovePlan(entry, approver)
			if err != nil {
				return err
			}
			con := a.console()
			rel := p.layout.Rel(p.layout.Plan(entry.Path))
			if res.Already {
				con.OK(con.T("plan.already", rel, short(res.Hash)))
			} else {
				con.OK(con.T("plan.approved", rel, approver, short(res.Hash)))
			}
			con.Info(con.T("spec.next.approved", entry.ID))
			return nil
		},
	}
	c.Flags().StringVar(&by, "by", "", "name of the approver (default: git user.name)")
	return c
}
