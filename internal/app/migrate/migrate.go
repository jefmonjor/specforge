// Package migrate turns a legacy system into specifications for its
// rewrite. SpecForge measures the legacy code itself (the inventory), and
// the agent reads it to describe what it does: first a capability map,
// then one specification per capability. The agent may read the legacy
// repository but never change it, and every source it cites is opened by
// SpecForge: a citation that does not resolve is an invented source and
// the document goes back to the agent.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/conversation"
	"github.com/jefmonjor/specforge/v6/internal/app/docturn"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
	"github.com/jefmonjor/specforge/v6/internal/app/prompts"
	"github.com/jefmonjor/specforge/v6/internal/domain/legacy"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Where the migration documents live, project-relative.
const (
	InventoryRel    = "docs/legacy/INVENTORY.md"
	CapabilitiesRel = "docs/legacy/CAPABILITIES.md"
	// questionsRel collects the questions asked while mapping.
	questionsRel = "docs/legacy/questions.md"
	decisionsRel = "docs/legacy/decisions.md"
)

// capabilityHead is a capability of the map.
var capabilityHead = regexp.MustCompile(`(?m)^##\s+\S`)

// ErrNoLegacy reports a legacy directory that does not exist.
var ErrNoLegacy = errors.New("legacy repository not found")

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
	Events    Events
	Log       *slog.Logger
}

// Options locate the project and the legacy code.
type Options struct {
	Root string
	// Legacy is the legacy repository, absolute.
	Legacy          string
	Language, Model string
	AgentTimeout    time.Duration
	MaxAttempts     int
	// JavaRelease and ForbiddenImports are the targets of the rewrite.
	JavaRelease      int
	ForbiddenImports []string
}

// CheckLegacy verifies that dir is a directory.
func CheckLegacy(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNoLegacy, dir)
	}
	return nil
}

// Scan measures the legacy code and writes docs/legacy/INVENTORY.md.
func Scan(files ports.Files, o Options) (legacy.Inventory, string, error) {
	if err := CheckLegacy(o.Legacy); err != nil {
		return legacy.Inventory{}, "", err
	}
	inv, err := legacy.Scan(os.DirFS(o.Legacy))
	if err != nil {
		return inv, "", err
	}
	path := filepath.Join(o.Root, filepath.FromSlash(InventoryRel))
	return inv, path, files.WriteFile(path, []byte(inv.Markdown(o.Language, o.Legacy)))
}

// Map asks the agent for the capability map, docs/legacy/CAPABILITIES.md.
func Map(ctx context.Context, d Deps, o Options) (string, error) {
	inv, _, err := Scan(d.Files, o)
	if err != nil {
		return "", err
	}
	path := filepath.Join(o.Root, filepath.FromSlash(CapabilitiesRel))
	data := o.data()
	data.Inventory = inv.Markdown(o.Language, o.Legacy)
	data.DocPath = CapabilitiesRel
	origin := clarify.Origin{Phase: "LEGACY", DecisionsFile: filepath.Join(o.Root, decisionsRel), QuestionsFile: filepath.Join(o.Root, questionsRel)}
	job := d.job(o, "capability map", origin, func(rel string) bool { return rel == CapabilitiesRel || rel == InventoryRel },
		func(feedback string, t conversation.Turn) (string, error) {
			dd := data
			dd.Feedback, dd.Decisions = feedback, d.Asker.Decisions(origin)
			dd.AnsweredQuestion, dd.Answer = t.Question, t.Answer
			return prompts.Render(o.Language, prompts.LegacyMap, dd)
		},
		func() ([]string, error) {
			content, err := d.Files.ReadFile(path)
			if err != nil {
				return []string{CapabilitiesRel + " was not written"}, nil
			}
			return checkMap(os.DirFS(o.Legacy), string(content)), nil
		})
	return path, docturn.Run(ctx, d.docturn(), job)
}

// checkMap reports what keeps a capability map from being accepted.
func checkMap(legacyFS fs.FS, content string) []string {
	var problems []string
	if !capabilityHead.MatchString(content) {
		problems = append(problems, "the map has no capability: write one `## <capability>` section per capability")
	}
	cs := legacy.Citations(content)
	if len(cs) == 0 {
		problems = append(problems, "the map cites no legacy source: cite each rule as `path:line`")
	}
	return append(problems, legacy.Verify(legacyFS, cs)...)
}

// Draft fills the specification at specPath with the behaviour of one
// capability as the legacy code implements it. Open questions are allowed:
// they are what the developer must decide before approving.
func Draft(ctx context.Context, d Deps, o Options, specPath, capability string) error {
	if err := CheckLegacy(o.Legacy); err != nil {
		return err
	}
	inv, err := legacy.Scan(os.DirFS(o.Legacy))
	if err != nil {
		return err
	}
	lay := layout.Layout{Root: o.Root}
	specRel := lay.Rel(specPath)
	data := o.data()
	data.SpecPath, data.Capability = specRel, capability
	data.Inventory = inv.Markdown(o.Language, o.Legacy)
	if c, err := d.Files.ReadFile(filepath.Join(o.Root, filepath.FromSlash(CapabilitiesRel))); err == nil {
		data.Capabilities = string(c)
	}
	origin := clarify.Origin{Phase: "FROM-LEGACY", DecisionsFile: lay.Decisions(specPath), QuestionsFile: lay.Questions(specPath)}
	job := d.job(o, "specification", origin, func(rel string) bool { return rel == specRel },
		func(feedback string, t conversation.Turn) (string, error) {
			dd := data
			dd.Feedback, dd.Decisions = feedback, d.Asker.Decisions(origin)
			dd.AnsweredQuestion, dd.Answer = t.Question, t.Answer
			if current, err := d.Files.ReadFile(specPath); err == nil {
				dd.Draft = string(current)
			}
			return prompts.Render(o.Language, prompts.FromLegacy, dd)
		},
		func() ([]string, error) {
			content, err := d.Files.ReadFile(specPath)
			if err != nil {
				return nil, err
			}
			return CheckSpec(os.DirFS(o.Legacy), string(content), o.Language), nil
		})
	return docturn.Run(ctx, d.docturn(), job)
}

// CheckSpec reports what keeps a specification drafted from legacy code
// from being handed to the developer: anything that blocks approval except
// open questions, a missing legacy sources section, and citations that do
// not resolve.
func CheckSpec(legacyFS fs.FS, content, lang string) []string {
	var problems []string
	for _, i := range spec.Blocking(spec.Lint(content, spec.ParseOptions{Languages: []string{lang}})) {
		if i.Rule != spec.RuleOpenQuestion {
			problems = append(problems, i.String())
		}
	}
	sources := spec.Section(content, spec.LegacySourcesTitle)
	cited := legacy.Citations(sources)
	if len(cited) == 0 {
		problems = append(problems, "section `13. Legacy sources` is missing or cites no legacy code: cite the source of each rule and scenario as `path:line`")
	}
	// Elsewhere only line citations are taken as legacy sources; a bare
	// file name in a sentence may well be a file of the new system.
	for _, c := range legacy.Citations(content) {
		if c.From > 0 {
			cited = append(cited, c)
		}
	}
	return append(problems, legacy.Verify(legacyFS, dedupe(cited))...)
}

func dedupe(cs []legacy.Citation) []legacy.Citation {
	seen := map[string]bool{}
	var out []legacy.Citation
	for _, c := range cs {
		if !seen[c.Reference] {
			seen[c.Reference] = true
			out = append(out, c)
		}
	}
	return out
}

func (o Options) data() prompts.Data {
	return prompts.Data{Legacy: o.Legacy, JavaRelease: o.JavaRelease, ForbiddenImports: o.ForbiddenImports, MaxAttempts: o.maxAttempts()}
}

func (o Options) maxAttempts() int {
	if o.MaxAttempts <= 0 {
		return 3
	}
	return o.MaxAttempts
}

// ReadDirs are the directories the agent may read outside the project:
// the legacy repository, unless it is inside the project already.
func (o Options) ReadDirs() []string { return docturn.Outside(o.Root, o.Legacy) }

func (d Deps) docturn() docturn.Deps {
	return docturn.Deps{Agent: d.Agent, Workspace: d.Workspace, Asker: d.Asker}
}

func (d Deps) job(o Options, step string, origin clarify.Origin, allowed func(string) bool,
	render func(string, conversation.Turn) (string, error), check func() ([]string, error)) docturn.Job {
	return docturn.Job{
		Step:        step,
		Root:        o.Root,
		Origin:      origin,
		Request:     ports.AgentRequest{Dir: o.Root, Model: o.Model, Timeout: o.AgentTimeout, ReadDirs: o.ReadDirs()},
		Allowed:     allowed,
		Render:      render,
		Check:       check,
		MaxAttempts: o.maxAttempts(),
		Hooks:       conversation.Hooks{Working: d.Events.Working, Answered: func(q, a string) error { d.Events.Answered(q, a); return nil }},
		Rejected:    func(p []string) { d.Events.Rejected(strings.Join(p, "; ")) },
	}
}
