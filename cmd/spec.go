package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/clarify"
	"specforge/internal/app/prompts"
	"specforge/internal/app/specs"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/ports"
)

func (a *App) specCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "spec",
		Short: "Create, complete, lint and approve specifications",
		Long: `A specification lives in specs/NNNN-slug.md. It says what to build and why,
with the acceptance criteria as Gherkin scenarios. Its lifecycle:

  new → interview (or edit by hand) → clarify → lint → approve → loop

To change an approved specification, edit it and approve it again: the
approval history (specs/NNNN-slug/approvals.md) records which scenarios were
added, modified or removed, and the loop redoes only those.

approve is the review gate: it refuses while a TODO or an open question is
left, records who approved it and seals the content. The loop only runs an
approved, unchanged specification.`,
	}
	c.AddCommand(a.specNewCommand(), a.specListCommand(), a.specLintCommand(), a.specClarifyCommand(), a.specApproveCommand(), a.specInterviewCommand())
	return c
}

func (a *App) specNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "new <title>",
		Short:   "Create the next numbered specification from the template",
		Example: `  specforge spec new "Password reset"`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			e, err := p.specs(a).New(strings.Join(args, " "))
			if err != nil {
				return err
			}
			con := a.console()
			con.OK(con.T("spec.created", e.Rel))
			con.Info(con.T("spec.next.new", e.ID, e.ID))
			con.Data(e.Rel)
			return nil
		},
	}
}

func (a *App) specListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the specifications and their state",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			all, err := p.specs(a).List()
			if err != nil {
				return err
			}
			type row struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				State string `json:"state"`
				Path  string `json:"path"`
			}
			rows := []row{}
			for _, e := range all {
				rows = append(rows, row{e.ID, e.Title, string(e.State), e.Rel})
			}
			if ok, err := a.emit(rows); ok {
				return err
			}
			con := a.console()
			if len(all) == 0 {
				con.Info(con.T("spec.none"))
				return nil
			}
			for _, e := range all {
				con.Data(fmt.Sprintf("%s  %-22s  %s", e.ID, con.T("spec.state."+string(e.State)), e.Title))
			}
			return nil
		},
	}
}

func (a *App) specLintCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [spec]",
		Short: "Check that a specification is ready for approval",
		Long: `lint reports what blocks approval (TODO placeholders, open questions,
scenarios without a When or a Then, Gherkin that does not parse) and advice
that does not block (missing template sections, invariants no scenario
mentions, scenarios with several actions).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			e, err := a.resolveSpec(cmd.Context(), p, argOrEmpty(args))
			if err != nil {
				return err
			}
			issues, err := p.specs(a).Lint(e)
			if err != nil {
				return err
			}
			a.printAdvice(issues)
			if blocking := spec.Blocking(issues); len(blocking) > 0 {
				return &specs.LintError{Path: e.Rel, Issues: blocking}
			}
			con := a.console()
			con.OK(con.T("spec.lint.clean", e.Rel))
			return nil
		},
	}
}

func (a *App) specApproveCommand() *cobra.Command {
	var by string
	c := &cobra.Command{
		Use:   "approve [spec]",
		Short: "Approve and seal a specification (review gate R0)",
		Long: `approve lints the specification and refuses while anything blocks it. Then it
records the approver and the date in the front matter and seals the content
with a SHA-256 hash. The approver is --by, else your git user.name, else a
question. Approving an edited specification again accepts the change.`,
		Example: "  specforge spec approve 0001\n  specforge spec approve 0001 --by \"Ana López\"",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			e, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			approver, err := a.approver(ctx, p.root, by)
			if err != nil {
				return err
			}
			res, err := p.specs(a).Approve(e, approver)
			if err != nil {
				return err
			}
			con := a.console()
			switch {
			case res.Already:
				con.OK(con.T("spec.already", e.Rel, short(res.Hash)))
			default:
				if res.Resealed {
					con.Warn(con.T("spec.resealed", e.Rel))
				}
				con.OK(con.T("spec.approved", e.Rel, approver, short(res.Hash)))
				if res.Resealed {
					for _, c := range res.Delta {
						if c.Change != specs.Unchanged {
							con.Detail(string(c.Change) + " · " + c.Title)
						}
					}
				}
				a.printAdvice(res.Advice)
			}
			con.Info(con.T("spec.next.approved", e.ID))
			return nil
		},
	}
	c.Flags().StringVar(&by, "by", "", "name of the approver (default: git user.name)")
	return c
}

func (a *App) specClarifyCommand() *cobra.Command {
	var by string
	c := &cobra.Command{
		Use:   "clarify [spec]",
		Short: "Answer the open questions of a specification, one at a time",
		Long: `clarify asks every [NEEDS CLARIFICATION] of the specification and writes each
answer in place of its question ("- **Decided:** question → answer"), so the
specification says what was decided. The answers also go to the decisions
log. Without a terminal the questions go to questions.md (exit 5): answer
them there and run clarify again.`,
		Example: "  specforge spec clarify 0001",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			e, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			who := strings.TrimSpace(by)
			if who == "" {
				who, _ = a.gitUser(ctx, p.root)
			}
			asker := &clarify.Asker{Prompter: a.prompter(), Files: fsys.OS{}, Now: a.Now, Lang: p.settings.Language}
			n, err := p.specs(a).Clarify(ctx, e, asker, who)
			con := a.console()
			if n > 0 {
				con.OK(con.T("spec.clarified", n, e.Rel))
			}
			if err != nil {
				return err
			}
			con.Info(con.T("spec.next.clarified", e.ID))
			return nil
		},
	}
	c.Flags().StringVar(&by, "by", "", "name recorded with each decision (default: git user.name)")
	return c
}

func (a *App) specInterviewCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "interview [spec]",
		Short: "Complete a specification in a conversation with your agent",
		Long: `interview opens your coding agent with instructions to complete the
specification one question at a time, writing each answer into the file and
recording what you cannot answer yet as an open question. It never approves
or seals: review the result and run spec approve.`,
		Example: `  specforge spec new "Password reset" && specforge spec interview 0001`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			if !a.canAsk() {
				return errors.New(a.console().T("interview.tty"))
			}
			e, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			all, err := p.specs(a).List()
			if err != nil {
				return err
			}
			var others []string
			for _, s := range all {
				if s.Rel != e.Rel {
					others = append(others, "- "+s.Rel+" ("+s.Title+")")
				}
			}
			prompt, err := prompts.RenderInterview(p.settings.Language, prompts.InterviewData{
				ID: e.ID, Title: e.Title, SpecPath: e.Rel, Context: strings.Join(others, "\n"),
			})
			if err != nil {
				return err
			}
			ag, err := a.NewAgent(p.settings.Agent, a.NewProcess(a.log), a.log)
			if err != nil {
				return err
			}
			if err := ag.Interactive(ctx, ports.AgentRequest{Prompt: prompt, Dir: p.root, Model: p.settings.Model}); err != nil {
				return err
			}
			con := a.console()
			con.OK(con.T("interview.done", e.Rel, e.ID))
			return nil
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}

// approver is --by, else git user.name, else the answer to a question.
func (a *App) approver(ctx context.Context, root, by string) (string, error) {
	if by = strings.TrimSpace(by); by != "" {
		return by, nil
	}
	if name, ok := a.gitUser(ctx, root); ok {
		return name, nil
	}
	if !a.canAsk() {
		return "", errors.New("who approves? pass --by \"<name>\" or set git user.name")
	}
	return a.prompter().Ask(ctx, ports.Question{Text: a.console().T("ask.approver")})
}

// gitUser returns git's user.name for the project, if set.
func (a *App) gitUser(ctx context.Context, root string) (string, bool) {
	res, err := a.NewProcess(a.log).Run(ctx, ports.Command{Name: "git", Args: []string{"config", "user.name"}, Dir: root})
	if err != nil || !res.Success() || strings.TrimSpace(res.Stdout) == "" {
		return "", false
	}
	return strings.TrimSpace(res.Stdout), true
}

func (a *App) printAdvice(issues []spec.Issue) {
	con := a.console()
	var advice []string
	for _, i := range issues {
		if !i.Blocking {
			advice = append(advice, i.String())
		}
	}
	if len(advice) > 0 {
		con.Info(con.T("spec.advice"))
		con.Detail(strings.Join(advice, "\n"))
	}
}

func argOrEmpty(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
