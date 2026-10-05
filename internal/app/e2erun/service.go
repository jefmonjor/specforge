// Package e2erun verifies a specification's scenarios in the running
// application, one scenario at a time.
//
// The agent proposes one action per turn. The action is validated against
// the current page before it reaches the browser, and a scenario passes
// only when every Then step has evidence that SpecForge found on the page
// itself. Each step leaves a screenshot and the run leaves report.json.
package e2erun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
	"github.com/jefmonjor/specforge/v6/internal/app/prompts"
	"github.com/jefmonjor/specforge/v6/internal/domain/e2e"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/jsontext"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Options configure a run.
type Options struct {
	Root     string
	SpecPath string
	BaseURL  string
	Language string
	// Scenarios restricts the run to these indexes; empty runs all.
	Scenarios    []int
	MaxSteps     int
	MinPassRate  float64
	Model        string
	AgentEnv     []string
	AgentTimeout time.Duration
}

// Deps are the collaborators of a run.
type Deps struct {
	Agent   ports.Agent
	Browser ports.Browser
	Files   ports.Files
	Asker   *clarify.Asker
	Events  Events
	Log     *slog.Logger
}

// Events lets the presentation layer follow a run.
type Events interface {
	Scenario(index, total int, title string)
	Step(n int, a e2e.Action, outcome string)
	Result(r e2e.ScenarioResult)
}

// BelowThresholdError reports a pass rate under the minimum.
type BelowThresholdError struct {
	Rate, Min float64
}

func (e *BelowThresholdError) Error() string {
	return fmt.Sprintf("E2E pass rate %.0f%% is below the required %.0f%%", e.Rate*100, e.Min*100)
}

// Service runs E2E verifications.
type Service struct{ d Deps }

// New returns an E2E service.
func New(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	if d.Events == nil {
		d.Events = nopEvents{}
	}
	return &Service{d: d}
}

type nopEvents struct{}

func (nopEvents) Scenario(int, int, string)    {}
func (nopEvents) Step(int, e2e.Action, string) {}
func (nopEvents) Result(e2e.ScenarioResult)    {}

// agentReply is either an action or a protocol status.
type agentReply struct {
	e2e.Action
	Status   string   `json:"status"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Context  string   `json:"context"`
}

// Run verifies the scenarios and writes docs/e2e/<spec>/.
func (s *Service) Run(ctx context.Context, o Options) (e2e.Report, error) {
	if o.MaxSteps <= 0 {
		o.MaxSteps = 15
	}
	lay := layout.Layout{Root: o.Root}
	data, err := s.d.Files.ReadFile(o.SpecPath)
	if err != nil {
		return e2e.Report{}, err
	}
	doc, err := spec.Parse(string(data), spec.ParseOptions{Languages: []string{o.Language}})
	if err != nil {
		return e2e.Report{}, err
	}
	outDir := filepath.Join(o.Root, "docs", "e2e", filepath.Base(lay.SpecDir(o.SpecPath)))
	report := e2e.Report{BaseURL: o.BaseURL, Spec: lay.Rel(o.SpecPath)}

	selected := doc.Scenarios
	if len(o.Scenarios) > 0 {
		selected = nil
		for _, sc := range doc.Scenarios {
			for _, i := range o.Scenarios {
				if sc.Index == i {
					selected = append(selected, sc)
				}
			}
		}
	}
	for i, sc := range selected {
		s.d.Events.Scenario(i+1, len(selected), sc.Title)
		res, err := s.scenario(ctx, o, lay, doc.Title, sc, outDir)
		if err != nil {
			return report, err
		}
		s.d.Events.Result(res)
		report.Scenarios = append(report.Scenarios, res)
	}

	if err := s.write(outDir, report); err != nil {
		return report, err
	}
	if rate := report.PassRate(); rate < o.MinPassRate || len(report.Scenarios) == 0 {
		return report, &BelowThresholdError{Rate: rate, Min: o.MinPassRate}
	}
	return report, nil
}

func (s *Service) scenario(ctx context.Context, o Options, lay layout.Layout, app string, sc spec.Scenario, outDir string) (e2e.ScenarioResult, error) {
	res := e2e.ScenarioResult{Index: sc.Index, Title: sc.Title}
	for _, t := range sc.StepsOf(spec.Then) {
		res.Thens = append(res.Thens, e2e.ThenResult{Text: t.Keyword + t.Text})
	}
	if len(res.Thens) == 0 {
		res.Status, res.Reason = e2e.Error, "the scenario has no Then step to verify"
		return res, nil
	}
	if err := s.d.Browser.Navigate(ctx, o.BaseURL); err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		res.Status, res.Reason = e2e.Error, "cannot open "+o.BaseURL+": "+err.Error()
		return res, nil
	}

	origin := clarify.Origin{Phase: "E2E", Scenario: sc.Index, DecisionsFile: lay.Decisions(o.SpecPath), QuestionsFile: lay.Questions(o.SpecPath)}
	var history []string
	retriedFormat := false
	for step := 1; step <= o.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		snap, err := s.d.Browser.Snapshot(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.Status, res.Reason = e2e.Error, "cannot read the page: "+err.Error()
			return res, nil
		}

		reply, err := s.ask(ctx, o, app, sc, res, snap, history, origin)
		if err != nil {
			return res, err
		}
		if reply == nil {
			if retriedFormat {
				res.Status, res.Reason = e2e.Error, "the agent did not answer with a valid action twice"
				return res, nil
			}
			retriedFormat = true
			history = append(history, "Your previous answer was not a valid JSON action. Reply with exactly one JSON object.")
			step--
			continue
		}
		a := reply.Action
		shot := filepath.Join(outDir, fmt.Sprintf("scenario-%02d", sc.Index), fmt.Sprintf("step-%02d.png", step))
		log := e2e.StepLog{N: step, Action: a}

		if err := e2e.Validate(a, snap, o.BaseURL, len(res.Thens)); err != nil {
			log.Outcome = "rejected: " + err.Error()
			history = append(history, fmt.Sprintf("Step %d %s rejected: %v", step, a.Type, err))
			res.Steps = append(res.Steps, log)
			s.d.Events.Step(step, a, log.Outcome)
			continue
		}

		switch a.Type {
		case e2e.Assert:
			then := &res.Thens[a.ThenIndex-1]
			if e2e.Verify(*a.Evidence, snap) {
				then.Verified, then.Evidence = true, a.Evidence
				log.Outcome = "verified"
				history = append(history, fmt.Sprintf("Step %d: Then %d verified (%s %q)", step, a.ThenIndex, a.Evidence.Kind, a.Evidence.Value))
			} else {
				log.Outcome = "evidence not found on the page"
				history = append(history, fmt.Sprintf("Step %d: evidence %s %q for Then %d is NOT on the page", step, a.Evidence.Kind, a.Evidence.Value, a.ThenIndex))
			}
		case e2e.Fail:
			log.Outcome = "failed: " + a.Explanation
			res.Steps = append(res.Steps, s.withShot(ctx, log, shot))
			s.d.Events.Step(step, a, log.Outcome)
			res.Status, res.Reason = e2e.Failed, fmt.Sprintf("Then %d: %s", a.ThenIndex, a.Explanation)
			return res, nil
		default:
			if a.Type == e2e.Navigate {
				a.Value = resolve(o.BaseURL, a.Value)
			}
			if err := s.d.Browser.Do(ctx, a); err != nil {
				if ctx.Err() != nil {
					return res, ctx.Err()
				}
				log.Outcome = "error: " + err.Error()
				history = append(history, fmt.Sprintf("Step %d %s %s failed: %v", step, a.Type, a.Selector, err))
			} else {
				log.Outcome = "done"
				history = append(history, fmt.Sprintf("Step %d: %s %s %s", step, a.Type, a.Selector, describe(a)))
			}
		}
		res.Steps = append(res.Steps, s.withShot(ctx, log, shot))
		s.d.Events.Step(step, a, log.Outcome)

		if res.Done() {
			res.Status = e2e.Passed
			return res, nil
		}
	}
	res.Status, res.Reason = e2e.Failed, fmt.Sprintf("not every Then step was verified within %d steps", o.MaxSteps)
	return res, nil
}

// ask returns the agent's next action, routing questions to the developer.
// It returns nil when the answer is not a valid reply.
func (s *Service) ask(ctx context.Context, o Options, app string, sc spec.Scenario, res e2e.ScenarioResult, snap e2e.Snapshot, history []string, origin clarify.Origin) (*agentReply, error) {
	for questions := 0; ; questions++ {
		prompt, err := prompts.RenderE2E(o.Language, s.data(app, sc, res, snap, history, origin))
		if err != nil {
			return nil, err
		}
		out, err := s.d.Agent.Run(ctx, ports.AgentRequest{Prompt: prompt, Dir: o.Root, Model: o.Model, Env: o.AgentEnv, Timeout: o.AgentTimeout})
		if err != nil {
			return nil, fmt.Errorf("agent %s during E2E: %w", s.d.Agent.Name(), err)
		}
		reply, ok := jsontext.Decode(out, func(r agentReply) bool {
			return r.Type != "" || (r.Status == "needs_clarification" && r.Question != "")
		})
		if !ok {
			return nil, nil
		}
		if reply.Status != "needs_clarification" {
			return &reply, nil
		}
		if questions >= 3 {
			return nil, errors.New("the agent keeps asking for information during E2E; add the test data to the decisions log")
		}
		if _, err := s.d.Asker.Ask(ctx, origin, ports.Question{Text: reply.Question, Context: reply.Context, Options: reply.Options}); err != nil {
			return nil, err
		}
	}
}

func (s *Service) data(app string, sc spec.Scenario, res e2e.ScenarioResult, snap e2e.Snapshot, history []string, origin clarify.Origin) prompts.E2EData {
	d := prompts.E2EData{App: app, Index: sc.Index, Title: sc.Title, Scenario: sc.Source,
		URL: snap.URL, PageTitle: snap.Title, Text: snap.Text, Decisions: s.d.Asker.Decisions(origin)}
	for i, t := range res.Thens {
		d.Thens = append(d.Thens, prompts.E2EThen{N: i + 1, Text: t.Text, Verified: t.Verified})
	}
	for _, e := range snap.Elements {
		d.Elements = append(d.Elements, prompts.E2EElement{Selector: e.Selector, Tag: e.Tag, Type: e.Type, Text: e.Text, Placeholder: e.Placeholder, AriaLabel: e.AriaLabel})
	}
	if len(history) > 12 {
		history = history[len(history)-12:]
	}
	d.History = history
	return d
}

func (s *Service) withShot(ctx context.Context, l e2e.StepLog, path string) e2e.StepLog {
	if err := s.d.Browser.Screenshot(ctx, path); err == nil {
		l.Screenshot = path
	}
	return l
}

func (s *Service) write(dir string, r e2e.Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return errors.Join(
		s.d.Files.WriteFile(filepath.Join(dir, "report.json"), data),
		s.d.Files.WriteFile(filepath.Join(dir, "REPORT.md"), []byte(r.Markdown())),
	)
}

func describe(a e2e.Action) string {
	if a.Type == e2e.Type {
		return fmt.Sprintf("%q", a.Value)
	}
	return strings.TrimSpace(a.Value)
}

// resolve makes a validated, same-origin target absolute.
func resolve(base, target string) string {
	b, err := url.Parse(base)
	if err != nil {
		return target
	}
	t, err := url.Parse(target)
	if err != nil {
		return target
	}
	return b.ResolveReference(t).String()
}
