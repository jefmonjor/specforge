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
	"specforge/internal/app/docturn"
	"specforge/internal/app/layout"
	"specforge/internal/app/prompts"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// maxTreeFiles bounds the file list shown to the agent.
const maxTreeFiles = 400

// ScopeError and IncompleteError are the errors of a document turn.
type (
	ScopeError      = docturn.ScopeError
	IncompleteError = docturn.IncompleteError
)

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
	// Legacy, JavaRelease and ForbiddenImports describe a rewrite: the
	// agent reads the legacy code (never changes it) to plan the new one.
	Legacy           string
	JavaRelease      int
	ForbiddenImports []string
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
		Legacy:      o.Legacy, JavaRelease: o.JavaRelease, ForbiddenImports: o.ForbiddenImports,
	}
	if o.Legacy != "" {
		data.LegacySources = spec.Section(o.SpecText, spec.LegacySourcesTitle)
	}
	job := docturn.Job{
		Step:    "plan",
		Root:    o.Root,
		Origin:  origin,
		Request: ports.AgentRequest{Dir: o.Root, Model: o.Model, Timeout: o.AgentTimeout, ReadDirs: docturn.Outside(o.Root, o.Legacy)},
		Allowed: func(rel string) bool { return rel == planRel },
		Render: func(feedback string, t conversation.Turn) (string, error) {
			dd := data
			dd.Feedback = feedback
			dd.Decisions = d.Asker.Decisions(origin)
			dd.Draft = ""
			if current, err := d.Files.ReadFile(planPath); err == nil {
				dd.Draft = spec.StripSeal(string(current))
			}
			dd.AnsweredQuestion, dd.Answer = t.Question, t.Answer
			return prompts.Render(o.Language, prompts.Plan, dd)
		},
		Check: func() ([]string, error) {
			content, err := d.Files.ReadFile(planPath)
			if err != nil {
				return []string{planRel + " was not written"}, nil
			}
			var problems []string
			for _, i := range spec.Blocking(spec.LintPlan(string(content), markers)) {
				problems = append(problems, i.String())
			}
			return problems, nil
		},
		MaxAttempts: o.MaxAttempts,
		Hooks:       conversation.Hooks{Working: d.Events.Working, Answered: func(q, a string) error { d.Events.Answered(q, a); return nil }},
		Rejected:    func(p []string) { d.Events.Rejected(strings.Join(p, "; ")) },
	}
	deps := docturn.Deps{Agent: d.Agent, Workspace: d.Workspace, Asker: d.Asker}
	if err := docturn.Run(ctx, deps, job); err != nil {
		return "", err
	}
	content, err := d.Files.ReadFile(planPath)
	if err != nil {
		return "", err
	}
	return planPath, ensureFrontMatter(d.Files, planPath, string(content), o.SpecID)
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
