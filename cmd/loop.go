package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/gates"
	"specforge/internal/adapters/scratch"
	"specforge/internal/adapters/testrun"
	"specforge/internal/adapters/vcs"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/guardhook"
	"specforge/internal/app/reviewer"
	"specforge/internal/app/tddloop"
	"specforge/internal/app/verifier"
	"specforge/internal/config"
	"specforge/internal/domain/legacy"
	"specforge/internal/domain/tdd"
	"specforge/internal/ui"
)

func (a *App) loopCommand() *cobra.Command {
	var (
		o               config.Overrides
		resume, restart bool
		scenario        int
		from            string
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
  REVIEW    review lenses read the scenario's diff (none for a passive change,
            one for a medium one, four for a high one); every finding must
            point at a changed line; what blocks gets one correction, judged
            again by REFACTOR and validated on those findings only
  VERIFY    for high-risk scenarios (verify: high), an independent verifier
            probes the specification in a copy of the project and must give
            a verdict, with the command and its output, for every invariant

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
			phase := tdd.Phase(strings.ToUpper(from))
			if from != "" && scenario == 0 {
				return errors.New("--from needs --scenario")
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
			files := fsys.OS{}
			events := &ui.LoopEvents{C: con, Agent: ag.Name(), Stack: profile.Name()}
			ws := workspace.New(proc)
			sandboxes := scratch.New(proc, p.settings.VerifyMaxBytes)
			svc := tddloop.New(tddloop.Deps{
				Agent: ag,
				Tests: testrun.New(proc),
				Gates: append(gates.ForProfile(profile, proc, p.settings.Quality),
					&gates.Migration{Target: legacy.Target{JavaRelease: p.settings.Migration.JavaRelease, ForbiddenImports: p.settings.Migration.ForbiddenImports}}),
				Workspace: ws,
				Files:     files,
				Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				VCS:       vcs.New(proc),
				Reviewer:  reviewer.New(reviewer.Deps{Agent: ag, Workspace: ws, Events: events}),
				Verifier:  verifier.New(verifier.Deps{Agent: ag, Proc: proc, Scratch: sandboxes, Workspace: ws, Files: files, Events: events}),
				Scratch:   sandboxes,
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
				Scenario:     scenario,
				From:         phase,
				Review:       p.settings.Review,
				Commit:       p.settings.Commit,
				MaxAttempts:  p.settings.MaxAttempts,
				AgentTimeout: p.settings.AgentTimeout,
				TestTimeout:  p.settings.TestTimeout,
				Models:       p.settings.ModelChoice(),
				Risk:         p.settings.Risk,
				MutationFrom: p.settings.MutationFrom,
				Surfaces:     p.settings.Surfaces,
				LensesAuto:   p.settings.LensesAuto,
				Lenses:       p.settings.Lenses,
				Verify:       p.settings.Verify,
				Blind:        p.settings.BlindReview,
				Parallel:     p.settings.Parallel,
				// The agents the loop runs cannot answer the guard's questions.
				AgentEnv: []string{guardhook.LoopEnv + "=1"},
				Strict:   p.settings.Quality.Strict,

				Legacy:           legacyDir,
				JavaRelease:      p.settings.Migration.JavaRelease,
				ForbiddenImports: p.settings.Migration.ForbiddenImports,
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
	f.IntVar(&scenario, "scenario", 0, "redo this scenario number (keeps the others)")
	f.StringVar(&from, "from", "", "with --scenario: start at red | green | refactor (default red)")
	f.StringVar(&o.Review, "review", "", "scenario: review every finished scenario | risk: only medium and high risk | off (default: specforge.yaml, else scenario)")
	f.BoolVar(&o.NoCommit, "no-commit", false, "do not record each finished scenario as a commit")
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
