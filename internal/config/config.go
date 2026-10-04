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

	"specforge/internal/domain/quality"
)

// Supported values.
var (
	Agents    = []string{"claude", "gemini"}
	Languages = []string{"es", "en"}
	Reviews   = []string{"scenario", "off"}
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
	Language    string `yaml:"language,omitempty"`
	Agent       string `yaml:"agent,omitempty"`
	Model       string `yaml:"model,omitempty"`
	Stack       string `yaml:"stack,omitempty"`
	MaxAttempts int    `yaml:"max_attempts,omitempty"`
	Review      string `yaml:"review,omitempty"`
	Commit      *bool  `yaml:"commit,omitempty"`
	Timeouts    struct {
		Agent time.Duration `yaml:"agent,omitempty"`
		Tests time.Duration `yaml:"tests,omitempty"`
	} `yaml:"timeouts,omitempty"`
	Quality struct {
		Strict                *bool    `yaml:"strict,omitempty"`
		MaxDuplicationPercent *float64 `yaml:"max_duplication_percent,omitempty"`
		MinMutationScore      *float64 `yaml:"min_mutation_score,omitempty"`
	} `yaml:"quality,omitempty"`
	Migration Migration `yaml:"migration,omitempty"`
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
	"org.apache.log4j", "junit.framework",
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
}

// Defaults applied when no layer sets a value.
const (
	DefaultLanguage     = "en"
	DefaultMaxAttempts  = 3
	DefaultReview       = "scenario"
	DefaultAgentTimeout = 20 * time.Minute
	DefaultTestTimeout  = 10 * time.Minute
)

// Resolve merges the layers and validates the result. requireAgent is false
// for commands that never call an agent.
func Resolve(u User, p Project, f Overrides, requireAgent bool) (Settings, error) {
	s := Settings{
		Agent:        first(f.Agent, p.Agent, u.Agent),
		Model:        first(f.Model, p.Model, u.Model),
		Language:     first(p.Language, u.Language, DefaultLanguage),
		Stack:        first(f.Stack, p.Stack),
		MaxAttempts:  DefaultMaxAttempts,
		Review:       first(f.Review, p.Review, DefaultReview),
		Commit:       !f.NoCommit && (p.Commit == nil || *p.Commit),
		AgentTimeout: DefaultAgentTimeout,
		TestTimeout:  DefaultTestTimeout,
		Quality:      quality.DefaultThresholds(),
		Migration:    p.Migration,
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

	s.Review = strings.ToLower(s.Review)
	s.Agent = strings.ToLower(s.Agent)
	s.Language = strings.ToLower(s.Language)
	s.Stack = strings.ToLower(s.Stack)
	return s, s.validate(requireAgent)
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
