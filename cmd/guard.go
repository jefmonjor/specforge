package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"specforge/internal/app/guardhook"
	"specforge/internal/config"
)

// exitError ends a command with a code and no diagnosis: the command
// already said what it had to (a hook's verdict).
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func (a *App) guardCommand() *cobra.Command {
	var (
		hook     string
		selfTest bool
	)
	c := &cobra.Command{
		Use:   "guard",
		Short: "The destructive-command guard, run by the agents' pre-tool hooks",
		Long: `guard reads the command an agent is about to run, as the agent's hook sends
it on stdin, and blocks it (exit 2, the reason on stderr for the agent)
when it would destroy work: recursive deletes, git reset --hard, clean -f,
push --force, branch -D, stash drop, SQL that drops or empties data, or a
command touching secrets (.env, .ssh/, *.pem, credentials). Deleting /, ~
or the project is never allowed.

guard.mode in specforge.yaml: block (default) | confirm (Claude Code asks
you; inside the loop nobody can answer, so it blocks) | off. guard.allow
lists command patterns to let through, * matching anything.

It also reads what the command runs: a script (sh x.sh, ./x.sh, source),
the file given to an interpreter (python x.py, node x.js, go run x.go),
code given inline (python -c, node -e), an npm/pnpm/yarn/bun script, a
make target, or a file the same command writes before running it. It is not a sandbox: what a program imports, a compiled
binary or a variable's value at run time are out of its reach; the loop's
checkpoints (specforge restore) recover from what gets past it.
specforge setup installs the hook; doctor checks it; --selftest proves it
blocks.`,
		Example: "  specforge guard --hook claude < input.json\n  specforge guard --selftest",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			mode, allow, cfgErr := a.guardSettings()
			if selfTest {
				if err := guardhook.SelfTest(mode, allow); err != nil {
					return err
				}
				fmt.Fprintln(a.Out, "guard: ok (mode "+mode+")")
				return nil
			}
			if !slices.Contains(guardhook.Agents, hook) {
				return fmt.Errorf("--hook must be one of %v", guardhook.Agents)
			}
			if cfgErr != nil {
				fmt.Fprintln(a.Err, "SpecForge guard: specforge.yaml is invalid, blocking every destructive command:", cfgErr)
			}
			req, err := guardhook.Command(hook, a.In)
			if err != nil {
				// An unreadable request is refused: a guard never fails open.
				fmt.Fprintln(a.Err, "SpecForge guard:", err)
				return &exitError{code: 2}
			}
			if req.Dir == "" {
				req.Dir, _ = a.Getwd()
			}
			read := guardhook.Files(req.Dir, func(p string) (io.ReadCloser, error) { return os.Open(p) })
			d := guardhook.Decide(req.Command, read, mode, allow, a.Getenv(guardhook.LoopEnv) == "")
			if code := guardhook.Respond(hook, d, a.Out, a.Err); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
	c.Flags().StringVar(&hook, "hook", "", "the agent whose hook calls it: claude | gemini")
	c.Flags().BoolVar(&selfTest, "selftest", false, "check that the guard blocks a destructive command")
	return c
}

// guardSettings reads guard.mode and guard.allow. With an invalid
// configuration the guard blocks, without any allowed pattern.
func (a *App) guardSettings() (mode string, allow []string, err error) {
	dir := a.Getenv("CLAUDE_PROJECT_DIR")
	if dir == "" {
		if dir, err = a.Getwd(); err != nil {
			return guardhook.Block, nil, err
		}
	}
	p, err := config.LoadProject(findRoot(dir))
	if err == nil {
		var s config.Settings
		if s, err = config.Resolve(config.User{}, p, config.Overrides{}, false); err == nil {
			return s.GuardMode, s.GuardAllow, nil
		}
	}
	return guardhook.Block, nil, errors.Join(err)
}
