// Package planning drafts the technical plan of an approved specification:
// one agent turn that writes specs/NNNN-slug/plan.md and nothing else.
// SpecForge checks the result (only the plan changed, every scenario has a
// planned test) before the developer reviews and approves it (gate R1).
package planning

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"specforge/internal/app/clarify"
	"specforge/internal/app/conversation"
	"specforge/internal/app/layout"
	"specforge/internal/app/prompts"
	"specforge/internal/app/protocol"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// maxTreeFiles bounds the file list shown to the agent.
const maxTreeFiles = 400

// ScopeError reports files the agent changed besides the plan.
type ScopeError struct{ Files []string }

func (e *ScopeError) Error() string {
	return "the plan step may only write plan.md, but these files changed too: " + strings.Join(e.Files, ", ")
}

// IncompleteError reports a plan that still fails its lint after every
// attempt.
type IncompleteError struct{ Issues []spec.Issue }

func (e *IncompleteError) Error() string {
	var lines []string
	for _, i := range e.Issues {
		lines = append(lines, i.String())
	}
	return "the plan is incomplete: " + strings.Join(lines, "; ")
}

// Events reports progress.
type Events interface {
	Working()
	Rejected(reason string)
	Answered(question, answer string)
}

// Deps are the collaborators.
type Deps struct {
	Agent     ports.Agent
	Workspace ports.Workspace
	Files     ports.Files
	Asker     *clarify.Asker
	// Lister returns the project's files, project-relative.
	Lister func(ctx context.Context, root string) ([]string, error)
	Events Events
	Log    *slog.Logger
}

// Options select the specification.
type Options struct {
	Root, SpecPath, SpecID string
	Doc                    *spec.Document
	SpecText               string
	Profile                *stack.Profile
	Language, Model        string
	AgentTimeout           time.Duration
	MaxAttempts            int
}

// Draft writes the plan and returns its path. An existing plan is the
// starting point: the agent revises it instead of starting over.
func Draft(ctx context.Context, d Deps, o Options) (string, error) {
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 3
	}
	lay := layout.Layout{Root: o.Root}
	planPath := lay.Plan(o.SpecPath)
	planRel := lay.Rel(planPath)
	markers := make([]string, len(o.Doc.Scenarios))
	for i, sc := range o.Doc.Scenarios {
		markers[i] = spec.Marker(o.SpecID, sc.Index)
	}
	origin := clarify.Origin{Phase: "PLAN", DecisionsFile: lay.Decisions(o.SpecPath), QuestionsFile: lay.Questions(o.SpecPath)}

	tree, err := d.Lister(ctx, o.Root)
	if err != nil {
		return "", err
	}
	tree = slices.DeleteFunc(tree, func(p string) bool { return strings.HasPrefix(p, "specs/") || strings.HasPrefix(p, ".specforge/") })
	if len(tree) > maxTreeFiles {
		tree = append(tree[:maxTreeFiles], fmt.Sprintf("… and %d more", len(tree)-maxTreeFiles))
	}
	stackName := "new"
	if o.Profile != nil {
		stackName = o.Profile.Name()
	}
	data := prompts.Data{
		SpecTitle: o.Doc.Title, SpecPath: lay.Rel(o.SpecPath), Stack: stackName,
		Spec: o.SpecText, Tree: strings.Join(tree, "\n"), PlanPath: planRel, Markers: markers,
		MaxAttempts: o.MaxAttempts,
	}
	_, err = d.Files.ReadFile(planPath)
	revising := err == nil

	feedback := ""
	for attempt := 1; ; attempt++ {
		data.Feedback, data.Attempt = feedback, attempt-1
		data.Draft = ""
		if current, err := d.Files.ReadFile(planPath); err == nil {
			data.Draft = spec.StripSeal(string(current))
		}
		data.Decisions = d.Asker.Decisions(origin)
		before, err := d.Workspace.Snapshot(ctx, o.Root)
		if err != nil {
			return "", err
		}
		render := func(t conversation.Turn) (string, error) {
			dd := data
			if t.Retry {
				dd.Feedback = "Your answer did not end with the JSON status object. Do the task again and end with the contract."
			}
			if t.Answer != "" {
				dd.Decisions = d.Asker.Decisions(origin)
				dd.AnsweredQuestion, dd.Answer = t.Question, t.Answer
			}
			return prompts.Render(o.Language, prompts.Plan, dd)
		}
		resp, err := conversation.Talk(ctx, d.Agent, d.Asker, origin, 0,
			ports.AgentRequest{Dir: o.Root, Model: o.Model, Timeout: o.AgentTimeout}, render,
			conversation.Hooks{Working: d.Events.Working, Answered: func(q, a string) error { d.Events.Answered(q, a); return nil }})
		if err != nil {
			return "", err
		}
		if resp.Status == protocol.Blocked {
			return "", &tdd.AgentBlockedError{Phase: "PLAN", Reason: resp.Reason, SuggestedAction: resp.SuggestedAction}
		}
		after, err := d.Workspace.Snapshot(ctx, o.Root)
		if err != nil {
			return "", err
		}
		changed := before.Changed(after)
		if outside := slices.DeleteFunc(slices.Clone(changed), func(p string) bool { return p == planRel }); len(outside) > 0 {
			return "", &ScopeError{Files: outside}
		}

		content, err := d.Files.ReadFile(planPath)
		var issues []spec.Issue
		switch {
		case err != nil || (!slices.Contains(changed, planRel) && !revising):
			issues = []spec.Issue{{Rule: spec.RulePlanMarker, Message: planRel + " was not written", Blocking: true}}
		default:
			issues = spec.Blocking(spec.LintPlan(string(content), markers))
		}
		if len(issues) == 0 {
			return planPath, ensureFrontMatter(d.Files, planPath, string(content), o.SpecID)
		}
		if attempt >= o.MaxAttempts {
			return "", &IncompleteError{Issues: issues}
		}
		var lines []string
		for _, i := range issues {
			lines = append(lines, "- "+i.String())
		}
		feedback = "The plan is not complete yet:\n" + strings.Join(lines, "\n")
		d.Events.Rejected(strings.Join(lines, "; "))
	}
}

// ensureFrontMatter gives the plan the same lifecycle fields as a
// specification; a new draft is never approved.
func ensureFrontMatter(files ports.Files, path, content, specID string) error {
	m, ok, err := spec.ReadMeta(content)
	if err != nil {
		return err
	}
	values := map[string]string{"spec": specID, "status": string(spec.StatusDraft), "approved_by": "", "approved_at": ""}
	if ok && m.Status == spec.StatusDraft {
		delete(values, "approved_by")
		delete(values, "approved_at")
	}
	// A seal left by an earlier approval no longer applies to a redraft.
	out, err := spec.SetMeta(spec.StripSeal(content), values, "spec", "status", "approved_by", "approved_at")
	if err != nil {
		return err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return files.WriteFile(path, []byte(out))
}
