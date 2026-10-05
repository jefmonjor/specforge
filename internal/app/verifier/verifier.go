// Package verifier runs the independent verifier: an agent that checks the
// specification itself, not the writer's work, in a disposable copy of the
// project where it can build, run and probe anything. SpecForge requires
// a verdict for every invariant and scenario asked, a command and its
// observed output for every failure, and the real project untouched.
package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"specforge/internal/app/answer"
	"specforge/internal/app/prompts"
	"specforge/internal/domain/verification"
	"specforge/internal/ports"
)

// StepError reports a verifier whose answer never was a complete report.
// The verification fails closed: no answer is not "all met".
type StepError struct{ Problems []string }

func (e *StepError) Error() string {
	return "the verifier did not return a complete report (the verification fails closed): " + strings.Join(e.Problems, "; ")
}

// TouchedError reports a verifier that changed the real project instead
// of its copy.
type TouchedError struct{ Files []string }

func (e *TouchedError) Error() string {
	return "the verifier changed the project instead of its copy: " + strings.Join(e.Files, ", ")
}

// BlockedError reports requirements the verifier showed broken.
type BlockedError struct{ Blockers []verification.Blocker }

func (e *BlockedError) Error() string {
	ids := make([]string, len(e.Blockers))
	for i, b := range e.Blockers {
		ids[i] = b.ID
	}
	return "the verifier showed requirements broken: " + strings.Join(ids, ", ")
}

// Deps are the collaborators of a verification.
type Deps struct {
	Agent ports.Agent
	// Proc runs each blocker's command again, in the copy.
	Proc      ports.CommandRunner
	Scratch   ports.Scratch
	Workspace ports.Workspace
	Files     ports.Files
	Events    Events
}

// Events lets the presentation layer follow a verification.
type Events interface {
	Verifying(required int)
	Retried(step string)
}

type nopEvents struct{}

func (nopEvents) Verifying(int)  {}
func (nopEvents) Retried(string) {}

// Request describes one verification.
type Request struct {
	Root, Language, Stack string
	// Base is the commit the base copy is taken at ("" for none).
	Base string
	// SpecTitle and Spec are the specification; Required the invariants
	// and scenario markers that need a verdict.
	SpecTitle, Spec string
	Required        []string
	// Report, when set, is where the result is written.
	Report  string
	Model   ports.ModelFor
	Env     []string
	Timeout time.Duration
	Now     time.Time
}

// Result is a finished verification.
type Result struct {
	At       time.Time           `json:"at"`
	Required []string            `json:"required"`
	Report   verification.Report `json:"report"`
	// Skipped says why the verifier did not run (a project too large to
	// copy); never silent.
	Skipped string `json:"skipped,omitempty"`
}

// Service runs verifications.
type Service struct{ d Deps }

// New returns a verification service.
func New(d Deps) *Service {
	if d.Events == nil {
		d.Events = nopEvents{}
	}
	return &Service{d: d}
}

var schemaPath = "review/verify-schema.json"

// Verify runs the verifier in a copy of the project.
func (s *Service) Verify(ctx context.Context, req Request) (Result, error) {
	res := Result{At: req.Now.UTC(), Required: req.Required}
	copies, err := s.d.Scratch.Copy(ctx, req.Root, req.Base)
	if errors.Is(err, ports.ErrTooLarge) {
		res.Skipped = err.Error()
		return res, s.write(req, res)
	}
	if err != nil {
		return res, fmt.Errorf("copying the project for the verifier: %w", err)
	}
	defer func() { _ = copies.Remove() }()

	before, err := s.d.Workspace.Snapshot(ctx, req.Root)
	if err != nil {
		return res, err
	}
	s.d.Events.Verifying(len(req.Required))
	report, verr := s.ask(ctx, req, copies)
	after, err := s.d.Workspace.Snapshot(ctx, req.Root)
	if err != nil {
		return res, err
	}
	if changed := before.Changed(after); len(changed) > 0 {
		return res, &TouchedError{Files: changed}
	}
	if verr != nil {
		return res, verr
	}
	res.Report = report
	return res, s.write(req, res)
}

func (s *Service) ask(ctx context.Context, req Request, c ports.Copies) (verification.Report, error) {
	schema, err := answer.Load(schemaPath)
	if err != nil {
		return verification.Report{}, err
	}
	data := prompts.VerifyData{Stack: req.Stack, SpecTitle: req.SpecTitle, Spec: req.Spec, Required: req.Required, BaseDir: c.BaseDir}
	var problems []string
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			s.d.Events.Retried("verify")
			data.Feedback = strings.Join(problems, "\n")
		}
		prompt, err := prompts.RenderVerify(req.Language, data)
		if err != nil {
			return verification.Report{}, err
		}
		out, err := s.d.Agent.Run(ctx, ports.AgentRequest{Prompt: prompt, Dir: c.Dir, Model: req.Model.For("verify"), Commands: true, Env: req.Env, Timeout: req.Timeout})
		if err != nil {
			return verification.Report{}, fmt.Errorf("agent %s: %w", s.d.Agent.Name(), err)
		}
		var r verification.Report
		if !schema.Decode(out, &r) {
			problems = []string{"the answer did not end with a JSON object valid for the verification schema"}
			continue
		}
		if problems = r.Problems(req.Required); len(problems) > 0 {
			continue
		}
		if problems = s.reproduce(ctx, c.Dir, r.Blockers); len(problems) == 0 {
			return r, nil
		}
	}
	return verification.Report{}, &StepError{Problems: problems}
}

// reproduceTimeout bounds each blocker's command when SpecForge runs it.
const reproduceTimeout = 2 * time.Minute

// reproduce runs every blocker's command again in the copy and requires
// its output to contain the first line the verifier says it observed. A
// failure SpecForge cannot reproduce is not evidence.
func (s *Service) reproduce(ctx context.Context, dir string, blockers []verification.Blocker) []string {
	if s.d.Proc == nil {
		return nil
	}
	var problems []string
	for _, b := range blockers {
		name, args := shell(b.Command)
		res, err := s.d.Proc.Run(ctx, ports.Command{Name: name, Args: args, Dir: dir, Timeout: reproduceTimeout})
		want := normalize(firstLine(b.Observed))
		if err == nil && strings.Contains(normalize(res.Combined()), want) {
			continue
		}
		got := firstLine(res.Combined())
		if err != nil {
			got = err.Error()
		}
		problems = append(problems, fmt.Sprintf("blocker %s: SpecForge ran `%s` and it did not print %q (it printed %q); give the exact command you ran and paste its output", b.ID, b.Command, firstLine(b.Observed), got))
	}
	return problems
}

func shell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

func (s *Service) write(req Request, res Result) error {
	if req.Report == "" || s.d.Files == nil {
		return nil
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return s.d.Files.WriteFile(req.Report, append(data, '\n'))
}
