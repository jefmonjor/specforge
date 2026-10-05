package cmd

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/vcs"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/reviewer"
	"specforge/internal/config"
	"specforge/internal/domain/review"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ui"
)

func (a *App) reviewCommand() *cobra.Command {
	var (
		o      config.Overrides
		base   string
		lenses []string
	)
	c := &cobra.Command{
		Use:   "review [spec]",
		Short: "Review a whole branch with the review lenses (read only)",
		Long: `review runs the review lenses over the changes of your branch, for code that
did not come out of the loop. The lenses follow the branch's risk (none for
documentation, one for ordinary code, four for sensitive paths or large
changes) unless --lens names them.

Every finding must point at a line the branch changed; the others are
discarded with the reason. Inferential findings that would block go to an
independent refuter. The report goes to specs/NNNN-slug/review/branch.json
when a specification is given (deliver shows it), else docs/review/branch.json.

Exit code 2 when a finding blocks or needs your judgement: there is no
automatic correction outside the loop.`,
		Example: "  specforge review\n  specforge review 0001 --base origin/main\n  specforge review --lens risk --lens resilience",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			opts := reviewer.BranchOptions{
				Root: p.root, Base: base, Language: p.settings.Language, Rules: p.settings.Risk,
				LensesAuto: p.settings.LensesAuto, Lenses: p.settings.Lenses, Blind: p.settings.BlindReview,
				Report: filepath.Join(p.root, "docs", "review", "branch.json"),
				Model:  p.settings.ModelFor, Timeout: p.settings.AgentTimeout, Now: a.Now(),
			}
			if len(lenses) > 0 {
				if opts.Lenses, opts.LensesAuto, err = review.ParseLenses(lenses); err != nil {
					return err
				}
			}
			if prof, err := a.resolveStack(ctx, p); err == nil {
				opts.Stack = prof.Name()
			} else if !errors.Is(err, tdd.ErrUnsupportedStack) {
				return err
			}
			if len(args) == 1 {
				entry, err := a.resolveSpec(ctx, p, args[0])
				if err != nil {
					return err
				}
				if err := a.reviewContext(p, entry.Path, &opts); err != nil {
					return err
				}
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("review.title"))
			events := &ui.LoopEvents{C: con, Agent: ag.Name(), Stack: opts.Stack}
			svc := reviewer.New(reviewer.Deps{Agent: ag, Workspace: workspace.New(proc), Events: events})
			res, err := svc.Branch(ctx, vcs.New(proc), fsys.OS{}, opts)
			if ok, jerr := a.emit(res); ok {
				return firstErr(jerr, err)
			}
			if res.Base != "" { // the branch was read: say what the review found
				a.printBranchReview(p, opts, res)
			}
			return err
		},
	}
	f := c.Flags()
	f.StringVar(&base, "base", "", "compare with this ref (default: origin/main, origin/master, main or master)")
	f.StringArrayVar(&lenses, "lens", nil, "run this lens (risk, reliability, readability, resilience); repeatable; default: by risk")
	f.StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	f.StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}

// reviewContext gives the lenses the specification the branch implements.
func (a *App) reviewContext(p project, specPath string, opts *reviewer.BranchOptions) error {
	data, err := fsys.OS{}.ReadFile(specPath)
	if err != nil {
		return err
	}
	md := string(data)
	if doc, err := spec.Parse(md, spec.ParseOptions{Languages: []string{p.settings.Language}}); err == nil {
		opts.SpecTitle = doc.Title
	}
	opts.Invariants = spec.Section(md, spec.InvariantsTitle)
	if plan, err := (fsys.OS{}).ReadFile(p.layout.Plan(specPath)); err == nil {
		opts.Plan = spec.StripSeal(string(plan))
	}
	opts.Report = filepath.Join(p.layout.SpecDir(specPath), "review", "branch.json")
	return nil
}

func (a *App) printBranchReview(p project, opts reviewer.BranchOptions, res reviewer.BranchResult) {
	con := a.console()
	if res.Empty {
		con.OK(con.T("review.empty", res.Base))
		return
	}
	con.Info(con.T("loop.risk", res.Risk.Tier, strings.Join(res.Risk.Reasons, " · ")))
	v := res.Verdict
	for _, f := range v.Blocking {
		con.Bad(con.T("review.finding", f.ID, f.Severity, f.Location.Path, f.Location.Line, f.Claim))
	}
	for _, f := range v.Escalated {
		con.Warn(con.T("review.finding", f.ID, f.Severity, f.Location.Path, f.Location.Line, f.Claim))
	}
	for _, f := range append(v.FollowUps, v.Info...) {
		con.Detail(con.T("review.finding", f.ID, f.Severity, f.Location.Path, f.Location.Line, f.Claim))
	}
	for _, d := range v.Discarded {
		con.Detail(con.T("review.discarded", d.ID, d.Reason))
	}
	con.OK(con.T("review.done", len(res.Lenses), res.Reported, 0, len(v.FollowUps), len(v.Discarded)))
	if opts.Report != "" {
		con.Info(con.T("review.report", p.layout.Rel(opts.Report)))
	}
}
