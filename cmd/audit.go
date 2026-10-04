package cmd

import (
	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/vcs"
	"specforge/internal/app/audit"
	"specforge/internal/app/clarify"
	"specforge/internal/config"
	"specforge/internal/domain/security"
	"specforge/internal/ui"
)

func (a *App) auditCommand() *cobra.Command {
	var (
		o      config.Overrides
		full   bool
		base   string
		failOn string
	)
	c := &cobra.Command{
		Use:   "audit",
		Short: "Adversarial security review of your changes (or the whole project)",
		Long: `audit runs three passes with your agent over each chunk of code:
reconnaissance, a red-team hunter that proposes findings with evidence, and a
blue-team validator that confirms or rejects each one. Every answer must
match a JSON schema; anything else stops the audit with an error, never with
an empty passing report (fail-closed).

Findings the validator cannot settle are questions for you. A confirmed
finding at or above --fail-on fails the command with exit code 2. Reports go
to docs/security/ with owner-only permissions.`,
		Example: "  specforge audit                 # changes since the default branch\n  specforge audit --base v1.2.0\n  specforge audit --full --fail-on medium",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			threshold, err := security.ParseSeverity(failOn)
			if err != nil {
				return err
			}
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("audit.title"))
			events := &ui.AuditEvents{C: con}
			defer events.Done()
			files := fsys.OS{}
			svc := audit.New(audit.Deps{
				Agent:  ag,
				VCS:    vcs.New(proc),
				Files:  files,
				Lister: listFiles,
				Asker:  &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Events: events,
				Log:    a.log,
				Now:    a.Now,
			})
			scope := audit.ScopeDiff
			if full {
				scope = audit.ScopeFull
			}
			res, err := svc.Run(cmd.Context(), audit.Options{
				Root:         p.root,
				Scope:        scope,
				Base:         base,
				Threshold:    threshold,
				Language:     p.settings.Language,
				Model:        p.settings.Model,
				AgentTimeout: p.settings.AgentTimeout,
			})
			events.Done()
			if res.Dir != "" && !res.Empty {
				con.Info(con.T("audit.report", p.layout.Rel(res.Dir)))
			}
			if err != nil {
				return err
			}
			if res.Empty {
				con.OK(con.T("audit.empty"))
				return nil
			}
			r := res.Report
			con.OK(con.T("audit.passed", threshold, r.TotalConfirmed, r.TotalNeedsValidation, r.TotalRejected))
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&full, "full", false, "audit the whole project instead of your changes")
	f.StringVar(&base, "base", "", "compare with this ref (default: origin/main, origin/master, main or master)")
	f.StringVar(&failOn, "fail-on", "high", "lowest severity that fails: critical | high | medium | low | info")
	f.StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	f.StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}
