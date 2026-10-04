package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/gates"
	"specforge/internal/adapters/testrun"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/tddloop"
	"specforge/internal/config"
	"specforge/internal/ui"
)

func (a *App) loopCommand() *cobra.Command {
	var (
		o               config.Overrides
		resume, restart bool
	)
	c := &cobra.Command{
		Use:   "loop [spec]",
		Short: "Run Red → Green → Refactor over an approved specification",
		Long: `loop takes the scenarios of an approved specification one by one:

  RED       the agent writes a test named with the scenario marker; SpecForge
            runs it and accepts it only if it compiles and fails on an assertion
  GREEN     the agent writes the code; the test files are fingerprinted and may
            not change; the scenario's test must pass
  REFACTOR  the whole suite and the quality gates of the stack must pass

The agent may answer with a question instead of guessing: you answer it at
the terminal and it is recorded in specs/NNNN-slug/decisions.md. Without a
terminal the question goes to questions.md and loop exits with code 5.
The state is saved after every step: --resume continues where it stopped.`,
		Example: "  specforge loop 0001\n  specforge loop --resume\n  specforge loop 0001 --restart --strict",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if resume && restart {
				return errors.New("--resume and --restart exclude each other")
			}
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			arg := argOrEmpty(args)
			if arg == "" && resume {
				arg = a.onlySavedLoop(p)
			}
			entry, err := a.resolveSpec(ctx, p, arg)
			if err != nil {
				return err
			}
			profile, err := a.resolveStack(ctx, p)
			if err != nil {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			files := fsys.OS{}
			events := &ui.LoopEvents{C: con, Agent: ag.Name(), Stack: profile.Name()}
			svc := tddloop.New(tddloop.Deps{
				Agent:     ag,
				Tests:     testrun.New(proc),
				Gates:     gates.ForProfile(profile, proc, p.settings.Quality),
				Workspace: workspace.New(proc),
				Files:     files,
				Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Events:    events,
				Log:       a.log,
				Now:       a.Now,
			})
			_, err = svc.Run(ctx, tddloop.Options{
				Root:         p.root,
				SpecPath:     entry.Path,
				Profile:      profile,
				Language:     p.settings.Language,
				Resume:       resume,
				Restart:      restart,
				MaxAttempts:  p.settings.MaxAttempts,
				AgentTimeout: p.settings.AgentTimeout,
				TestTimeout:  p.settings.TestTimeout,
				Model:        p.settings.Model,
				Strict:       p.settings.Quality.Strict,
			})
			return err
		},
	}
	f := c.Flags()
	f.BoolVar(&resume, "resume", false, "continue the saved loop of this specification")
	f.BoolVar(&restart, "restart", false, "discard the saved loop and start from the first scenario")
	f.StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	f.StringVar(&o.Model, "model", "", "model passed to the agent")
	f.StringVar(&o.Stack, "stack", "", "go | maven | gradle | node | python (default: specforge.yaml or detected)")
	f.BoolVar(&o.Strict, "strict", false, "a quality gate that cannot run blocks instead of warning")
	return c
}

// onlySavedLoop returns the specification with saved loop state when there
// is exactly one, so `loop --resume` needs no argument in the usual case.
func (a *App) onlySavedLoop(p project) string {
	all, err := p.specs(a).List()
	if err != nil {
		return ""
	}
	found := ""
	for _, e := range all {
		if _, err := os.Stat(p.layout.State(e.Path)); err == nil {
			if found != "" {
				return ""
			}
			found = e.Rel
		}
	}
	return found
}
