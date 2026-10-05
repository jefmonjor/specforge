package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"specforge/internal/buildinfo"
)

func (a *App) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "specforge",
		Short: "Spec-driven, test-first guardrails for AI coding agents",
		Long: `SpecForge drives Claude Code or Gemini CLI through a Red → Green → Refactor
loop against an approved specification, and verifies every step itself.

  specforge init                     choose your agent and language (once)
  specforge setup                    prepare this repository
  specforge setup --new java         or start a new project (java, react, python, go)
  specforge doctor                   check the agent, git, test runner and gate tools
  specforge spec new "<title>"       start a specification from the template
  specforge spec interview 0001      complete it with the agent
  specforge spec approve 0001        lint, record the approver and seal it
  specforge plan 0001                draft where the code goes; then plan approve
  specforge loop 0001                Red → Green → Refactor, review, one commit per scenario
  specforge deliver 0001             DELIVERY.md, trace.json and PR_BODY.md
  specforge review                   the review lenses over your branch (read only)
  specforge verify 0001              an independent check of the specification
  specforge audit                    adversarial security review of your branch
  specforge e2e 0001 --url <url>     verify the scenarios in a real browser

Rewriting a legacy system (the legacy code stays read-only):

  specforge legacy scan ../old       measure it: docs/legacy/INVENTORY.md
  specforge legacy map               its business capabilities, sources checked
  specforge spec from-legacy "<x>"   one specification per capability

Exit codes: 0 ok · 1 error · 2 a gate said no · 3 the specification or
loop state needs attention · 4 a tool or setting is missing · 5 a question
awaits your answer · 130 interrupted.`,
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			a.startLogging()
		},
	}
	root.SetVersionTemplate("specforge {{.Version}}\n")

	f := root.PersistentFlags()
	f.BoolVar(&a.flags.verbose, "verbose", false, "show progress details and stream the agent's output")
	f.BoolVar(&a.flags.debug, "debug", false, "write debug records to the log file")
	f.BoolVar(&a.flags.traceIO, "trace-io", false, "record every prompt and agent answer in the log file")
	f.BoolVarP(&a.flags.quiet, "quiet", "q", false, "print only warnings, errors and data")
	f.BoolVar(&a.flags.nonInteractive, "non-interactive", false, "never ask: write questions to a file and exit with code 5")
	f.BoolVar(&a.flags.json, "json", false, "print data (version, spec list, deliver) and errors as JSON")

	root.AddCommand(
		a.initCommand(),
		a.setupCommand(),
		a.doctorCommand(),
		a.specCommand(),
		a.legacyCommand(),
		a.planCommand(),
		a.loopCommand(),
		a.deliverCommand(),
		a.reviewCommand(),
		a.verifyCommand(),
		a.guardCommand(),
		a.auditCommand(),
		a.e2eCommand(),
		a.versionCommand(),
	)
	return root
}

func (a *App) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if ok, err := a.emit(map[string]string{"version": buildinfo.Version, "commit": buildinfo.Commit, "date": buildinfo.Date}); ok {
				return err
			}
			fmt.Fprintln(a.Out, "specforge "+buildinfo.String())
			return nil
		},
	}
}
