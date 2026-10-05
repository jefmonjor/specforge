package cmd

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/scratch"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/verifier"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/domain/verification"
	"specforge/internal/ui"
)

func (a *App) verifyCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "verify [spec]",
		Short: "Check a specification independently, in a copy of the project",
		Long: `verify asks an independent verifier to check the specification itself, not
the tests that were written for it: in a disposable copy of the project it
derives its own probes from every invariant and scenario, runs them, and
compares with a copy of the last commit.

SpecForge requires a verdict for every invariant and scenario, the command
and its observed output for everything broken, and the project untouched.
The report goes to specs/NNNN-slug/verify/feature.json (deliver shows it).

Exit code 2 when a requirement is broken: each blocker comes with the
command that reproduces it.`,
		Example: "  specforge verify 0001",
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
			files := fsys.OS{}
			data, err := files.ReadFile(entry.Path)
			if err != nil {
				return err
			}
			md := string(data)
			if err := spec.Verify(md); err != nil {
				return err
			}
			doc, err := spec.Parse(md, spec.ParseOptions{Languages: []string{p.settings.Language}})
			if err != nil {
				return err
			}
			required := spec.InvariantIDs(md)
			id := spec.IDFromPath(entry.Path)
			for _, sc := range doc.Scenarios {
				required = append(required, spec.Marker(id, sc.Index))
			}
			stackName := ""
			if prof, err := a.resolveStack(ctx, p); err == nil {
				stackName = prof.Name()
			} else if !errors.Is(err, tdd.ErrUnsupportedStack) {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("verify.title", doc.Title))
			events := &ui.LoopEvents{C: con, Agent: ag.Name(), Stack: stackName}
			report := p.layout.FeatureVerify(entry.Path)
			svc := verifier.New(verifier.Deps{Agent: ag, Proc: proc, Scratch: scratch.New(proc, p.settings.VerifyMaxBytes),
				Workspace: workspace.New(proc), Files: files, Events: events})
			res, err := svc.Verify(ctx, verifier.Request{
				Root: p.root, Language: p.settings.Language, Stack: stackName, Base: "HEAD",
				SpecTitle: doc.Title, Spec: strings.TrimSpace(spec.StripSeal(md)), Required: required, Report: report,
				Model: p.settings.ModelFor, Timeout: p.settings.AgentTimeout, Now: a.Now(),
			})
			if err != nil {
				return err
			}
			if ok, err := a.emit(res); ok {
				return err
			}
			events.Verified(tdd.ScenarioRef{}, tdd.VerifyRecord{Report: res.Report, Skipped: res.Skipped})
			con.Info(con.T("verify.report", p.layout.Rel(report)))
			if res.Report.Count(verification.Unmet) > 0 || len(res.Report.Blockers) > 0 {
				return &verifier.BlockedError{Blockers: res.Report.Blockers}
			}
			return nil
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}
