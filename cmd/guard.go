package cmd

import (
	"errors"
	"fmt"
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

It is lexical recognition, not a sandbox: scripts, programs and variable
expansions are not inspected. specforge setup installs the hook; doctor
checks it; --selftest proves it blocks.`,
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
			command, err := guardhook.Command(hook, a.In)
			if err != nil {
				// An unreadable request is refused: a guard never fails open.
				fmt.Fprintln(a.Err, "SpecForge guard:", err)
				return &exitError{code: 2}
			}
			d := guardhook.Decide(command, mode, allow, a.Getenv(guardhook.LoopEnv) == "")
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
