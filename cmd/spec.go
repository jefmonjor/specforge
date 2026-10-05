package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/interview"
	"specforge/internal/app/prompts"
	"specforge/internal/app/specs"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/ports"
	"specforge/internal/ui"
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
	c.AddCommand(a.specNewCommand(), a.specListCommand(), a.specLintCommand(), a.specClarifyCommand(), a.specApproveCommand(), a.specInterviewCommand(), a.specChangeCommand(), a.specFromLegacyCommand())
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
					a.printDelta(res.Delta)
				}
				a.printAdvice(res.Advice)
			}
			con.Info(a.nextAfterApproval(p, e, res))
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
	var (
		o    config.Overrides
		chat bool
	)
	c := &cobra.Command{
		Use:   "interview [spec]",
		Short: "Complete a specification, one question at a time",
		Long: `interview completes the specification in a conversation SpecForge runs.
Each turn your agent writes your last answer into the file and returns the
single most important next question, with why it matters and what is still
unknown; SpecForge asks you, records the question and answer in
specs/NNNN-slug/interview.jsonl and the decisions log, and rejects any change
outside the specification. It ends when no TODO or missing structure is
left; what you could not answer stays as an open question for spec clarify.
It never approves or seals.

Without a terminal the question goes to questions.md (exit 5): answer it
there and run interview again. --chat instead hands your terminal to the
agent for a free conversation.`,
		Example: "  specforge spec new \"Password reset\" && specforge spec interview 0001\n  specforge spec interview 0001 --chat",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			e, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			if chat {
				return a.chatInterview(ctx, p, e, ag)
			}
			con := a.console()
			if data, err := (fsys.OS{}).ReadFile(e.Path); err == nil && spec.Verify(string(data)) == nil {
				// Complete and sealed: there is nothing to interview about.
				con.Info(con.T("interview.approved", e.Rel, e.ID))
				return nil
			}
			con.Title(con.T("interview.title", e.Title))
			events := &ui.InterviewEvents{C: con, Agent: ag.Name()}
			defer events.Done()
			files := fsys.OS{}
			res, err := interview.Run(ctx, interview.Deps{
				Agent:     ag,
				Workspace: workspace.New(proc),
				Files:     files,
				Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Events:    events,
				Log:       a.log,
				Now:       a.Now,
			}, interview.Options{
				Root: p.root, SpecPath: e.Path, Language: p.settings.Language, Model: p.settings.ModelFor("interview"),
				AgentTimeout: p.settings.AgentTimeout,
			})
			events.Done()
			if err != nil {
				return err
			}
			con.OK(con.T("interview.done", e.Rel, e.ID))
			if len(res.Open) > 0 {
				con.Warn(con.T("interview.open", len(res.Open), e.ID))
			}
			a.printAdvice(res.Advice)
			return nil
		},
	}
	c.Flags().BoolVar(&chat, "chat", false, "hand the terminal to the agent for a free conversation instead")
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}

// chatInterview hands the terminal to the agent, seeded with instructions
// to complete the specification.
func (a *App) chatInterview(ctx context.Context, p project, e specs.Entry, ag ports.Agent) error {
	if !a.canAsk() {
		return errors.New(a.console().T("interview.tty"))
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
	if err := ag.Interactive(ctx, ports.AgentRequest{Prompt: prompt, Dir: p.root, Model: p.settings.ModelFor("interview")}); err != nil {
		return err
	}
	con := a.console()
	con.OK(con.T("interview.done", e.Rel, e.ID))
	return nil
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

// printDelta shows how the scenarios changed: their markers, which tests
// and records carry, never move.
func (a *App) printDelta(delta []spec.ScenarioChange) {
	con := a.console()
	for _, c := range delta {
		switch c.Change {
		case spec.Unchanged:
		case spec.Renamed:
			con.Detail(con.T("spec.delta.renamed", c.Marker, c.Title, c.Was))
		case spec.Removed:
			con.Warn(con.T("spec.delta.removed", c.Marker, c.Title))
		default:
			con.Detail(string(c.Change) + " · " + c.Marker + " · " + c.Title)
		}
	}
}

// nextAfterApproval names the next step: the plan when scenarios were
// added or removed and a plan exists, else the loop.
func (a *App) nextAfterApproval(p project, e specs.Entry, res specs.Approval) string {
	con := a.console()
	moved := slices.ContainsFunc(res.Delta, func(c spec.ScenarioChange) bool {
		return c.Change == spec.Added || c.Change == spec.Removed
	})
	if res.Resealed && moved && (fsys.OS{}).Exists(p.layout.Plan(e.Path)) {
		return con.T("spec.next.replan", e.ID)
	}
	return con.T("spec.next.approved", e.ID)
}

func (a *App) specChangeCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "change <spec> <request>",
		Short: "Apply a change request to a specification, asking what it leaves open",
		Long: `change applies what you ask to a specification, approved or not, in a
conversation SpecForge runs like the interview: your agent writes the change
into every section it touches (scenarios, invariants, data contracts,
errors), asks what the request leaves open or contradicts, and changes no
other file. The conversation is kept in specs/NNNN-slug/change.jsonl.

Each scenario keeps its marker, its tests and its history while its title
stays, even if its steps change; a renamed scenario keeps them while its
steps stay. change shows what the next approval will record, then:

  specforge spec approve <spec>   seal the change
  specforge plan <spec>           when scenarios were added or removed
  specforge loop <spec>           redo what changed, nothing else

Without a terminal a question goes to questions.md (exit 5): answer it there
and run the same change again.`,
		Example: "  specforge spec change 0001 \"Links expire after 15 minutes, not 30\"\n  specforge spec change 0001 \"Admins can revoke every link of a user\"",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			e, err := a.resolveSpec(ctx, p, args[0])
			if err != nil {
				return err
			}
			request := strings.TrimSpace(args[1])
			if request == "" {
				return errors.New("say what to change: specforge spec change <spec> \"<request>\"")
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("change.title", e.Title))
			events := &ui.InterviewEvents{C: con, Agent: ag.Name()}
			defer events.Done()
			files := fsys.OS{}
			res, err := interview.Run(ctx, interview.Deps{
				Agent:     ag,
				Workspace: workspace.New(proc),
				Files:     files,
				Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Events:    events,
				Log:       a.log,
				Now:       a.Now,
			}, interview.Options{
				Root: p.root, SpecPath: e.Path, Language: p.settings.Language, Model: p.settings.ModelFor("interview"),
				AgentTimeout: p.settings.AgentTimeout, Request: request,
			})
			events.Done()
			if err != nil {
				return err
			}
			if !res.Changed {
				con.OK(con.T("change.none", e.Rel))
				return nil
			}
			con.OK(con.T("change.done", e.Rel))
			if preview, err := p.specs(a).Preview(e); err == nil {
				a.printDelta(preview)
			}
			if len(res.Open) > 0 {
				con.Warn(con.T("interview.open", len(res.Open), e.ID))
			}
			a.printAdvice(res.Advice)
			con.Info(con.T("change.next", e.ID))
			return nil
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}
