// Package config resolves SpecForge's settings from three layers, each
// overriding the previous one: the developer's user configuration, the
// project's committed specforge.yaml, and command-line flags.
//
// Nothing here guesses: an agent that is not configured anywhere is an
// error that tells the developer how to configure it.
package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"specforge/internal/domain/quality"
	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
)

// Supported values.
var (
	Agents    = []string{"claude", "gemini"}
	Languages = []string{"es", "en"}
	Reviews   = []string{"scenario", "risk", "off"}
	// SurfaceModes are the values of plan.surfaces.
	SurfaceModes = []string{"ask", "strict", "off"}
	// VerifyModes are the values of verify.
	VerifyModes = []string{"high", "always", "feature", "off"}
	// GuardModes are the values of guard.mode.
	GuardModes = []string{"block", "confirm", "off"}
	// Phases are the steps of work a model can be chosen for.
	Phases = []string{"interview", "plan", "legacy", "red", "green", "refactor", "review", "review2", "refute", "verify", "audit", "e2e"}
)

// ProjectFile is the committed per-repository configuration file.
const ProjectFile = "specforge.yaml"

// ErrNoAgent reports that no layer chose an agent.
var ErrNoAgent = errors.New("no coding agent configured: run `specforge init` or pass --agent claude|gemini")

// User is the developer's configuration, shared by every project.
type User struct {
	Agent    string `yaml:"agent,omitempty"`
	Model    string `yaml:"model,omitempty"`
	Language string `yaml:"language,omitempty"`
}

// Project is specforge.yaml. Pointer fields distinguish "not set" from an
// explicit zero, which matters for thresholds such as 0% duplication.
type Project struct {
	Language string `yaml:"language,omitempty"`
	Agent    string `yaml:"agent,omitempty"`
	Model    string `yaml:"model,omitempty"`
	// Models picks a model per phase of work, over Model.
	Models      map[string]string `yaml:"models,omitempty"`
	Stack       string            `yaml:"stack,omitempty"`
	MaxAttempts int               `yaml:"max_attempts,omitempty"`
	Review      string            `yaml:"review,omitempty"`
	Commit      *bool             `yaml:"commit,omitempty"`
	Timeouts    struct {
		Agent time.Duration `yaml:"agent,omitempty"`
		Tests time.Duration `yaml:"tests,omitempty"`
	} `yaml:"timeouts,omitempty"`
	Quality struct {
		Strict                *bool    `yaml:"strict,omitempty"`
		MaxDuplicationPercent *float64 `yaml:"max_duplication_percent,omitempty"`
		MinMutationScore      *float64 `yaml:"min_mutation_score,omitempty"`
		// MutationFrom is the lowest risk tier that runs the mutation gate.
		MutationFrom string `yaml:"mutation_from,omitempty"`
	} `yaml:"quality,omitempty"`
	Migration Migration `yaml:"migration,omitempty"`
	Risk      Risk      `yaml:"risk,omitempty"`
	// Lenses are the review lenses after REFACTOR: auto (by risk), off, or
	// a list of lenses.
	Lenses StringList `yaml:"lenses,omitempty"`
	// Loop tunes the loop: Parallel is how many scenarios with disjoint
	// plan surfaces run side by side (1 by default).
	Loop struct {
		Parallel int `yaml:"parallel,omitempty"`
	} `yaml:"loop,omitempty"`
	// BlindReview runs the lenses of high-risk changes twice,
	// independently (models.review2 for the second pass).
	BlindReview bool `yaml:"blind_review,omitempty"`
	// Verify is when the independent verifier runs: high (default),
	// always, feature or off. VerifyMaxMB bounds the copy it works in.
	Verify      string `yaml:"verify,omitempty"`
	VerifyMaxMB int    `yaml:"verify_max_mb,omitempty"`
	// Guard is the destructive-command guard installed as the agents'
	// pre-tool hook.
	Guard struct {
		// Mode: block (default), confirm (ask when someone can answer) or
		// off.
		Mode string `yaml:"mode,omitempty"`
		// Allow are command patterns (* matches anything) let through.
		Allow []string `yaml:"allow,omitempty"`
	} `yaml:"guard,omitempty"`
	Plan struct {
		// Surfaces: what happens when the agent edits a file the approved
		// plan does not name (ask, strict or off).
		Surfaces string `yaml:"surfaces,omitempty"`
	} `yaml:"plan,omitempty"`
	Delivery struct {
		// BudgetLines is the size of a reviewable pull request; deliver
		// proposes slices above it.
		BudgetLines int `yaml:"budget_lines,omitempty"`
	} `yaml:"delivery,omitempty"`
}

// StringList reads a YAML scalar or a sequence of scalars.
type StringList []string

// UnmarshalYAML implements yaml.Unmarshaler.
func (l *StringList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = StringList{n.Value}
		return nil
	}
	var list []string
	if err := n.Decode(&list); err != nil {
		return err
	}
	*l = list
	return nil
}

// Risk tunes how the risk of each scenario's change is classified.
type Risk struct {
	// MaxLines: a change larger than this is high risk (default 400).
	MaxLines int `yaml:"max_lines,omitempty"`
	// HighPaths and PassivePaths are regular expressions over the
	// slash-separated path; when set they replace the defaults.
	HighPaths    []string `yaml:"high_paths,omitempty"`
	PassivePaths []string `yaml:"passive_paths,omitempty"`
	// Floor is the lowest tier any change gets (default passive).
	Floor string `yaml:"floor,omitempty"`
}

// Migration describes a rewrite of a legacy system: where the legacy code
// is (read-only for the agent) and what the new code must comply with.
type Migration struct {
	// Legacy is the legacy repository, relative to the project or absolute.
	Legacy string `yaml:"legacy,omitempty"`
	// JavaRelease is the release the build must declare (e.g. 21).
	JavaRelease int `yaml:"java_release,omitempty"`
	// ForbiddenImports are package prefixes the new code may not import.
	ForbiddenImports []string `yaml:"forbidden_imports,omitempty"`
}

// DefaultForbiddenImports are the Java EE packages that Jakarta EE renamed
// and the APIs a Java 21 codebase replaces; used when a Java migration
// sets no list of its own.
var DefaultForbiddenImports = []string{
	"javax.servlet", "javax.persistence", "javax.validation", "javax.ejb",
	"javax.jms", "javax.ws.rs", "javax.xml.bind", "javax.xml.rpc", "javax.annotation",
	"javax.inject", "javax.faces", "javax.transaction",
	"org.apache.log4j", "junit.framework", "java.util.Vector", "java.util.Hashtable",
}

// Overrides are command-line flags; empty values do not override.
type Overrides struct {
	Agent    string
	Model    string
	Stack    string
	Strict   bool
	Review   string
	NoCommit bool
}

// Settings are the resolved values every command uses.
type Settings struct {
	Agent        string
	Model        string
	Language     string
	Stack        string
	MaxAttempts  int
	Review       string
	Commit       bool
	AgentTimeout time.Duration
	TestTimeout  time.Duration
	Quality      quality.Thresholds
	Migration    Migration
	modelFlag    string
	// Models are the per-phase choices; ModelFor resolves them.
	Models map[string]string
	// Risk classifies each scenario's change; MutationFrom is the lowest
	// tier that runs the mutation gate.
	Risk         risk.Rules
	MutationFrom risk.Tier
	// Surfaces is plan.surfaces; BudgetLines is delivery.budget_lines.
	Surfaces    string
	BudgetLines int
	// LensesAuto picks the review lenses by risk; otherwise Lenses are the
	// lenses of every scenario (none: no lens review).
	LensesAuto bool
	Lenses     []review.Lens
	// BlindReview doubles the lenses of high-risk changes.
	BlindReview bool
	// Parallel is loop.parallel.
	Parallel int
	// Verify is when the verifier runs; VerifyMaxBytes bounds its copy.
	Verify         string
	VerifyMaxBytes int64
	// GuardMode and GuardAllow configure the destructive-command guard.
	GuardMode  string
	GuardAllow []string
}

// ModelFor resolves the model of a phase: --model, then models.<phase>,
// then model (project, then user). "" lets the agent decide.
func (s Settings) ModelFor(phase string) string {
	if s.modelFlag != "" {
		return s.modelFlag
	}
	if m := strings.TrimSpace(s.Models[phase]); m != "" {
		return m
	}
	return s.Model
}

// ModelChoice is ModelFor as the use cases take it.
func (s Settings) ModelChoice() func(string) string { return s.ModelFor }

// Defaults applied when no layer sets a value.
const (
	DefaultLanguage     = "en"
	DefaultMaxAttempts  = 3
	DefaultReview       = "scenario"
	DefaultAgentTimeout = 20 * time.Minute
	DefaultTestTimeout  = 10 * time.Minute
	// DefaultMutationFrom: mutation testing is slow; documentation-only
	// changes do not pay for it.
	DefaultMutationFrom = risk.Medium
	DefaultSurfaces     = "ask"
	// DefaultBudgetLines is the size of a pull request a person reviews well.
	DefaultBudgetLines = 400
)

// Resolve merges the layers and validates the result. requireAgent is false
// for commands that never call an agent.
func Resolve(u User, p Project, f Overrides, requireAgent bool) (Settings, error) {
	s := Settings{
		Agent:          first(f.Agent, p.Agent, u.Agent),
		Model:          first(f.Model, p.Model, u.Model),
		Language:       first(p.Language, u.Language, DefaultLanguage),
		Stack:          first(f.Stack, p.Stack),
		MaxAttempts:    DefaultMaxAttempts,
		Review:         first(f.Review, p.Review, DefaultReview),
		Commit:         !f.NoCommit && (p.Commit == nil || *p.Commit),
		AgentTimeout:   DefaultAgentTimeout,
		TestTimeout:    DefaultTestTimeout,
		Quality:        quality.DefaultThresholds(),
		Migration:      p.Migration,
		Models:         p.Models,
		modelFlag:      strings.TrimSpace(f.Model),
		Surfaces:       strings.ToLower(first(p.Plan.Surfaces, DefaultSurfaces)),
		Verify:         strings.ToLower(first(p.Verify, "high")),
		GuardMode:      strings.ToLower(first(p.Guard.Mode, "block")),
		GuardAllow:     p.Guard.Allow,
		BlindReview:    p.BlindReview,
		Parallel:       max(p.Loop.Parallel, 1),
		VerifyMaxBytes: int64(max(p.VerifyMaxMB, 0)) << 20,
		BudgetLines:    DefaultBudgetLines,
	}
	if p.Delivery.BudgetLines > 0 {
		s.BudgetLines = p.Delivery.BudgetLines
	}
	if s.Migration.JavaRelease > 0 && s.Migration.ForbiddenImports == nil {
		s.Migration.ForbiddenImports = DefaultForbiddenImports
	}
	if p.MaxAttempts > 0 {
		s.MaxAttempts = p.MaxAttempts
	}
	if p.Timeouts.Agent > 0 {
		s.AgentTimeout = p.Timeouts.Agent
	}
	if p.Timeouts.Tests > 0 {
		s.TestTimeout = p.Timeouts.Tests
	}
	if p.Quality.Strict != nil {
		s.Quality.Strict = *p.Quality.Strict
	}
	if p.Quality.MaxDuplicationPercent != nil {
		s.Quality.MaxDuplicationPercent = *p.Quality.MaxDuplicationPercent
	}
	if p.Quality.MinMutationScore != nil {
		s.Quality.MinMutationScore = *p.Quality.MinMutationScore
	}
	if f.Strict {
		s.Quality.Strict = true
	}
	var errs []error
	floor, err := risk.ParseTier(p.Risk.Floor)
	errs = append(errs, err)
	if s.Quality.Strict {
		floor = risk.High // strict: every change gets every check
	}
	s.Risk, err = risk.NewRules(p.Risk.MaxLines, p.Risk.HighPaths, p.Risk.PassivePaths, floor)
	errs = append(errs, err)
	s.MutationFrom = DefaultMutationFrom
	if p.Quality.MutationFrom != "" {
		s.MutationFrom, err = risk.ParseTier(p.Quality.MutationFrom)
		errs = append(errs, err)
	}
	s.Lenses, s.LensesAuto, err = review.ParseLenses(p.Lenses)
	if err != nil {
		errs = append(errs, fmt.Errorf("lenses: %w", err))
	}
	for phase := range p.Models {
		if !slices.Contains(Phases, phase) {
			errs = append(errs, fmt.Errorf("models.%s: unknown phase (supported: %s)", phase, strings.Join(Phases, ", ")))
		}
	}

	s.Review = strings.ToLower(s.Review)
	s.Agent = strings.ToLower(s.Agent)
	s.Language = strings.ToLower(s.Language)
	s.Stack = strings.ToLower(s.Stack)
	return s, errors.Join(append(errs, s.validate(requireAgent))...)
}

func (s Settings) validate(requireAgent bool) error {
	var errs []error
	switch {
	case s.Agent == "" && requireAgent:
		errs = append(errs, ErrNoAgent)
	case s.Agent != "" && !slices.Contains(Agents, s.Agent):
		errs = append(errs, fmt.Errorf("unknown agent %q (supported: %s)", s.Agent, strings.Join(Agents, ", ")))
	}
	if !slices.Contains(Languages, s.Language) {
		errs = append(errs, fmt.Errorf("unsupported language %q (supported: %s)", s.Language, strings.Join(Languages, ", ")))
	}
	if !slices.Contains(GuardModes, s.GuardMode) {
		errs = append(errs, fmt.Errorf("unknown guard.mode %q (supported: %s)", s.GuardMode, strings.Join(GuardModes, ", ")))
	}
	if !slices.Contains(VerifyModes, s.Verify) {
		errs = append(errs, fmt.Errorf("unknown verify mode %q (supported: %s)", s.Verify, strings.Join(VerifyModes, ", ")))
	}
	if !slices.Contains(SurfaceModes, s.Surfaces) {
		errs = append(errs, fmt.Errorf("unknown plan.surfaces %q (supported: %s)", s.Surfaces, strings.Join(SurfaceModes, ", ")))
	}
	if !slices.Contains(Reviews, s.Review) {
		errs = append(errs, fmt.Errorf("unknown review mode %q (supported: %s)", s.Review, strings.Join(Reviews, ", ")))
	}
	if s.Quality.MaxDuplicationPercent < 0 || s.Quality.MaxDuplicationPercent > 100 {
		errs = append(errs, fmt.Errorf("quality.max_duplication_percent must be between 0 and 100"))
	}
	if s.Quality.MinMutationScore < 0 || s.Quality.MinMutationScore > 100 {
		errs = append(errs, fmt.Errorf("quality.min_mutation_score must be between 0 and 100"))
	}
	if r := s.Migration.JavaRelease; r != 0 && (r < 6 || r > 99) {
		errs = append(errs, fmt.Errorf("migration.java_release must be a Java release such as 17 or 21, not %d", r))
	}
	return errors.Join(errs...)
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
