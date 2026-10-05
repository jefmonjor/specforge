// Package docturn runs an agent turn whose product is a document: a plan,
// a legacy capability map, a specification drafted from legacy code. The
// pattern is always the same and always verified by SpecForge, never taken
// on the agent's word: only the allowed files may change, directories the
// agent may read must stay untouched, and the document must pass a check;
// otherwise the agent gets the problems and another attempt.
package docturn

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"specforge/internal/app/clarify"
	"specforge/internal/app/conversation"
	"specforge/internal/app/protocol"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// ScopeError reports files the agent changed outside what it may change.
type ScopeError struct {
	Step  string
	Files []string
}

func (e *ScopeError) Error() string {
	return fmt.Sprintf("the %s step may only write its document, but these files changed too: %s", e.Step, strings.Join(e.Files, ", "))
}

// IncompleteError reports a document that still has problems after every
// attempt.
type IncompleteError struct {
	Step     string
	Problems []string
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("the %s is incomplete: %s", e.Step, strings.Join(e.Problems, "; "))
}

// Deps are the collaborators.
type Deps struct {
	Agent     ports.Agent
	Workspace ports.Workspace
	Asker     *clarify.Asker
}

// Job describes one document turn.
type Job struct {
	// Step names the document in messages ("plan", "capability map").
	Step   string
	Root   string
	Origin clarify.Origin
	// Request carries everything but the prompt; Request.ReadDirs are
	// checked to be unchanged afterwards.
	Request ports.AgentRequest
	// Allowed reports whether the agent may change a project-relative path.
	Allowed func(rel string) bool
	// Render builds the prompt; feedback lists the previous problems.
	Render func(feedback string, t conversation.Turn) (string, error)
	// Check returns the problems of the written document; none accepts it.
	Check       func() ([]string, error)
	MaxAttempts int
	Hooks       conversation.Hooks
	// Rejected is told about each rejected attempt.
	Rejected func(problems []string)
}

// NoOptionsFeedback tells the agent its question needed candidate answers.
const NoOptionsFeedback = "Your question had no options. Derive the candidate answers yourself (files, paths, commands, names, alternatives) and put at least two in \"options\"; the developer picks or trims them."

// Run executes the job until the document is accepted.
func Run(ctx context.Context, d Deps, j Job) error {
	if j.MaxAttempts <= 0 {
		j.MaxAttempts = 3
	}
	feedback := ""
	for attempt := 1; ; attempt++ {
		before, err := d.Workspace.Snapshot(ctx, j.Root)
		if err != nil {
			return err
		}
		touchedSince, err := Watch(d.Workspace, j.Request.ReadDirs)
		if err != nil {
			return err
		}
		render := func(t conversation.Turn) (string, error) {
			switch {
			case t.MissingOptions:
				return j.Render(strings.TrimSpace(feedback+"\n"+NoOptionsFeedback), t)
			case t.Retry:
				return j.Render(strings.TrimSpace(feedback+"\nYour last answer did not end with the JSON status object. Do the task again and end with the contract."), t)
			}
			return j.Render(feedback, t)
		}
		resp, err := conversation.Talk(ctx, d.Agent, d.Asker, j.Origin, conversation.Rules{RequireOptions: true}, j.Request, render, j.Hooks)
		if err != nil {
			return err
		}
		if resp.Status == protocol.Blocked {
			return &tdd.AgentBlockedError{Phase: tdd.Phase(strings.ToUpper(j.Step)), Reason: resp.Reason, SuggestedAction: resp.SuggestedAction}
		}
		after, err := d.Workspace.Snapshot(ctx, j.Root)
		if err != nil {
			return err
		}
		outside := slices.DeleteFunc(before.Changed(after), func(rel string) bool {
			return j.Allowed(rel) || j.isLog(rel)
		})
		touched, err := touchedSince()
		if err != nil {
			return err
		}
		if outside = append(outside, touched...); len(outside) > 0 {
			return &ScopeError{Step: j.Step, Files: outside}
		}
		problems, err := j.Check()
		if err != nil {
			return err
		}
		if len(problems) == 0 {
			return nil
		}
		if attempt >= j.MaxAttempts {
			return &IncompleteError{Step: j.Step, Problems: problems}
		}
		if j.Rejected != nil {
			j.Rejected(problems)
		}
		feedback = "The " + j.Step + " is not complete yet:\n- " + strings.Join(problems, "\n- ")
	}
}

// isLog reports whether rel is where SpecForge itself records the
// developer's answers during the turn.
func (j Job) isLog(rel string) bool {
	for _, f := range []string{j.Origin.DecisionsFile, j.Origin.QuestionsFile} {
		if f == "" {
			continue
		}
		if r, err := filepath.Rel(j.Root, f); err == nil && filepath.ToSlash(r) == rel {
			return true
		}
	}
	return false
}

func hashDirs(w ports.Workspace, dirs []string) ([]map[string]string, error) {
	var out []map[string]string
	for _, dir := range dirs {
		h, err := w.HashFiles(dir, func(string) bool { return true })
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// Watch hashes directories the agent may only read; the function it
// returns lists the files changed in them since.
func Watch(w ports.Workspace, dirs []string) (func() ([]string, error), error) {
	before, err := hashDirs(w, dirs)
	if err != nil {
		return nil, err
	}
	return func() ([]string, error) {
		after, err := hashDirs(w, dirs)
		if err != nil {
			return nil, err
		}
		var out []string
		for i, dir := range dirs {
			for _, p := range ports.Snapshot(before[i]).Changed(ports.Snapshot(after[i])) {
				out = append(out, filepath.ToSlash(dir)+"/"+p)
			}
		}
		return out, nil
	}, nil
}

// Outside returns the directories that are not inside root: those the
// agent needs to be given explicitly, and whose files the project's own
// snapshot does not cover.
func Outside(root string, dirs ...string) []string {
	var out []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		rel, err := filepath.Rel(root, d)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			out = append(out, d)
		}
	}
	return out
}
