// Package reviewer runs the review lenses over a change and judges what
// they report. Each lens is one read-only agent turn whose answer must
// validate against a JSON Schema; every proof is checked against the diff;
// inferential findings that would block go to an independent refuter; a
// correction is checked by a targeted validation of the corrected findings
// only. Nothing here trusts the agent: an answer that does not validate is
// retried once and then fails closed, and a turn that changes a file is
// refused.
package reviewer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"specforge/internal/app/answer"
	"specforge/internal/app/prompts"
	"specforge/internal/domain/review"
	"specforge/internal/ports"
)

// StepError reports a review turn whose answer never matched its schema.
// The review fails closed: no answer is not "no findings".
type StepError struct{ Step string }

func (e *StepError) Error() string {
	return fmt.Sprintf("the %s review step did not return the JSON its schema requires (the review fails closed rather than pass)", e.Step)
}

// ReadOnlyError reports a review turn that changed files.
type ReadOnlyError struct {
	Step  string
	Files []string
}

func (e *ReadOnlyError) Error() string {
	return fmt.Sprintf("the %s review step changed files it may only read: %s", e.Step, strings.Join(e.Files, ", "))
}

// Deps are the collaborators of a review.
type Deps struct {
	Agent     ports.Agent
	Workspace ports.Workspace
	Events    Events
}

// Events lets the presentation layer follow a review.
type Events interface {
	Lens(lens review.Lens)
	Refuting(n int)
	Validating(n int)
	Retried(step string)
}

// NopEvents ignores every event.
type NopEvents struct{}

func (NopEvents) Lens(review.Lens) {}
func (NopEvents) Refuting(int)     {}
func (NopEvents) Validating(int)   {}
func (NopEvents) Retried(string)   {}

// Request describes the change under review.
type Request struct {
	Root, Language, Stack string
	Lenses                []review.Lens
	// Diff is the unified diff of the change.
	Diff string
	// Context: what the change implements.
	SpecTitle, Marker, Scenario, Invariants, Plan string

	// Blind runs every lens twice, independently (the second pass with the
	// "review2" model); only what both prove on the same hunk skips the
	// refuter.
	Blind bool

	Model   ports.ModelFor
	Env     []string
	Timeout time.Duration
}

// Result is a finished review.
type Result struct {
	Lenses   []review.Lens  `json:"lenses"`
	Blind    bool           `json:"blind,omitempty"`
	Reported int            `json:"reported"`
	Verdict  review.Verdict `json:"verdict"`
}

// Service runs reviews.
type Service struct{ d Deps }

// New returns a review service.
func New(d Deps) *Service {
	if d.Events == nil {
		d.Events = NopEvents{}
	}
	return &Service{d: d}
}

var prefixes = map[review.Lens]string{
	review.LensRisk: "RSK", review.LensReliability: "REL", review.LensReadability: "RDB", review.LensResilience: "RES",
}

// Review runs each lens, verifies every proof against the diff and asks
// the refuter about the inferential findings that would block.
func (s *Service) Review(ctx context.Context, req Request) (Result, error) {
	diff, err := review.ParseDiff(req.Diff)
	if err != nil {
		return Result{}, fmt.Errorf("reading the diff under review: %w", err)
	}
	res := Result{Lenses: req.Lenses, Blind: req.Blind}
	found, err := s.pass(ctx, req, "review")
	if err != nil {
		return res, err
	}
	if req.Blind {
		second, err := s.pass(ctx, req, "review2")
		if err != nil {
			return res, err
		}
		found = review.Blind(found, second, diff)
	}
	res.Reported = len(found)
	res.Verdict = review.Verify(found, diff)
	if inferential := res.Verdict.Inferential(); len(inferential) > 0 {
		s.d.Events.Refuting(len(inferential))
		confirmed, reasons, err := s.refute(ctx, req, inferential)
		if err != nil {
			return res, err
		}
		res.Verdict = res.Verdict.Refute(confirmed, reasons)
	}
	return res, nil
}

// pass runs every lens once with the model of phase.
func (s *Service) pass(ctx context.Context, req Request, phase string) ([]review.Finding, error) {
	var found []review.Finding
	for _, lens := range req.Lenses {
		s.d.Events.Lens(lens)
		fs, err := s.lens(ctx, req, lens, phase)
		if err != nil {
			return nil, err
		}
		found = append(found, fs...)
	}
	return found, nil
}

type lensAnswer struct {
	Lens     review.Lens      `json:"lens"`
	Findings []review.Finding `json:"findings"`
}

func (s *Service) lens(ctx context.Context, req Request, lens review.Lens, phase string) ([]review.Finding, error) {
	data := s.data(req)
	data.Lens, data.Prefix = lens, prefixes[lens]
	var a lensAnswer
	step := "lens " + string(lens)
	valid := func() bool { return a.Lens == lens }
	if err := s.turn(ctx, req, step, phase, "review/lens-schema.json", prompts.Review, data, &a, valid); err != nil {
		return nil, err
	}
	for i := range a.Findings {
		a.Findings[i].Lens = string(lens)
	}
	return a.Findings, nil
}

func (s *Service) refute(ctx context.Context, req Request, fs []review.Finding) (map[string]bool, map[string]string, error) {
	data := s.data(req)
	data.Findings = fs
	var a struct {
		Verdicts []struct {
			ID, Verdict, Reason string
		} `json:"verdicts"`
	}
	if err := s.turn(ctx, req, "refute", "refute", "review/refute-schema.json", prompts.Refute, data, &a, nil); err != nil {
		return nil, nil, err
	}
	confirmed, reasons := map[string]bool{}, map[string]string{}
	for _, v := range a.Verdicts {
		confirmed[v.ID] = v.Verdict == "confirmed"
		reasons[v.ID] = v.Reason
	}
	return confirmed, reasons, nil
}

// Validate checks only the corrected findings against the code as it is
// now. A finding the validator does not answer is a regression: nothing is
// resolved without saying so.
func (s *Service) Validate(ctx context.Context, req Request, fixed []review.Finding) (map[string]review.Check, error) {
	s.d.Events.Validating(len(fixed))
	data := s.data(req)
	data.Findings = fixed
	var a struct {
		Results []struct {
			ID, Status, Reason string
		} `json:"results"`
	}
	if err := s.turn(ctx, req, "validation", "review", "review/validate-schema.json", prompts.Validate, data, &a, nil); err != nil {
		return nil, err
	}
	out := map[string]review.Check{}
	for _, f := range fixed {
		out[f.ID] = review.Check{Status: "regression", Reason: "the validator did not answer for it"}
	}
	for _, r := range a.Results {
		if _, ok := out[r.ID]; ok {
			out[r.ID] = review.Check{Status: r.Status, Reason: r.Reason}
		}
	}
	return out, nil
}

// turn runs one read-only turn and decodes its answer into v against the
// schema, retrying once.
func (s *Service) turn(ctx context.Context, req Request, step, phase, schemaPath string, name prompts.Name, data prompts.ReviewData, v any, valid func() bool) error {
	schema, err := answer.Load(schemaPath)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			s.d.Events.Retried(step)
			data.Feedback = retryFeedback(req.Language)
		}
		prompt, err := prompts.RenderReview(req.Language, name, data)
		if err != nil {
			return err
		}
		out, err := s.readOnly(ctx, req, step, phase, prompt)
		if err != nil {
			return err
		}
		if schema.Decode(out, v) && (valid == nil || valid()) {
			return nil
		}
	}
	return &StepError{Step: step}
}

// readOnly runs the agent and refuses the turn if any file changed.
func (s *Service) readOnly(ctx context.Context, req Request, step, phase, prompt string) (string, error) {
	before, err := s.d.Workspace.Snapshot(ctx, req.Root)
	if err != nil {
		return "", err
	}
	out, err := s.d.Agent.Run(ctx, ports.AgentRequest{Prompt: prompt, Dir: req.Root, Model: req.Model.For(phase), Env: req.Env, Timeout: req.Timeout})
	if err != nil {
		return "", fmt.Errorf("agent %s: %w", s.d.Agent.Name(), err)
	}
	after, err := s.d.Workspace.Snapshot(ctx, req.Root)
	if err != nil {
		return "", err
	}
	if changed := before.Changed(after); len(changed) > 0 {
		return "", &ReadOnlyError{Step: step, Files: changed}
	}
	return out, nil
}

func (s *Service) data(req Request) prompts.ReviewData {
	return prompts.ReviewData{
		Stack: req.Stack, SpecTitle: req.SpecTitle, Marker: req.Marker, Scenario: req.Scenario,
		Invariants: req.Invariants, Plan: req.Plan, Diff: req.Diff,
	}
}

func retryFeedback(lang string) string {
	if lang == "es" {
		return "Tu respuesta no terminaba con un objeto JSON válido para el esquema pedido. Responde de nuevo y termina con él."
	}
	return "Your answer did not end with a JSON object valid for the required schema. Answer again and end with it."
}
