package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"specforge/internal/domain/risk"
	"strings"
	"testing"
	"time"
)

func TestResolveLayersFlagsOverProjectOverUser(t *testing.T) {
	u := User{Agent: "gemini", Language: "es", Model: "u"}
	var p Project
	p.Agent = "claude"
	p.Timeouts.Tests = 2 * time.Minute
	zero := 0.0
	p.Quality.MaxDuplicationPercent = &zero

	s, err := Resolve(u, p, Overrides{Model: "flag"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.Agent != "claude" || s.Model != "flag" || s.Language != "es" || s.TestTimeout != 2*time.Minute || s.AgentTimeout != DefaultAgentTimeout {
		t.Fatalf("settings = %+v", s)
	}
	if s.Quality.MaxDuplicationPercent != 0 || s.Quality.MinMutationScore != 80 {
		t.Fatalf("quality = %+v", s.Quality)
	}
}

func TestResolveNeverGuessesTheAgent(t *testing.T) {
	if _, err := Resolve(User{}, Project{}, Overrides{}, true); !errors.Is(err, ErrNoAgent) {
		t.Fatalf("want ErrNoAgent, got %v", err)
	}
	if _, err := Resolve(User{}, Project{}, Overrides{}, false); err != nil {
		t.Fatalf("commands without an agent must not require one: %v", err)
	}
	_, err := Resolve(User{Agent: "copilot", Language: "fr"}, Project{}, Overrides{}, true)
	if err == nil || !strings.Contains(err.Error(), `unknown agent "copilot"`) || !strings.Contains(err.Error(), `"fr"`) {
		t.Fatalf("every invalid value must be reported: %v", err)
	}
}

func TestLoadProjectRejectsUnknownKeys(t *testing.T) {
	root := t.TempDir()
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(root, ProjectFile), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("language: es\nstack: go\ntimeouts:\n  agent: 30m\nquality:\n  strict: true\n  max_duplication_percent: 2.5\n")
	p, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Stack != "go" || p.Timeouts.Agent != 30*time.Minute || !*p.Quality.Strict || *p.Quality.MaxDuplicationPercent != 2.5 {
		t.Fatalf("project = %+v", p)
	}
	write("lenguage: es\n")
	if _, err := LoadProject(root); err == nil {
		t.Fatal("a typo must be an error, not a silent default")
	}
	write("# only a comment\n")
	if _, err := LoadProject(root); err != nil {
		t.Fatalf("an empty file is an empty configuration: %v", err)
	}
	if _, err := LoadProject(t.TempDir()); err != nil {
		t.Fatalf("a missing file is an empty configuration: %v", err)
	}
}

func TestUserConfigRoundTripIsPrivate(t *testing.T) {
	t.Setenv(HomeEnv, t.TempDir())
	d, err := UserDirs()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveUser(d, User{Agent: "claude", Language: "es"}); err != nil {
		t.Fatal(err)
	}
	u, err := LoadUser(d)
	if err != nil || u.Agent != "claude" || u.Language != "es" {
		t.Fatalf("LoadUser = %+v %v", u, err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(d.UserFile()); info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
	}
}

func TestModelForResolvesPerPhase(t *testing.T) {
	p := Project{Model: "sonnet", Models: map[string]string{"review": "opus", "green": "haiku"}}
	s, err := Resolve(User{Agent: "claude", Model: "user"}, p, Overrides{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for phase, want := range map[string]string{"review": "opus", "green": "haiku", "red": "sonnet"} {
		if got := s.ModelFor(phase); got != want {
			t.Errorf("ModelFor(%s) = %q, want %q", phase, got, want)
		}
	}
	s, _ = Resolve(User{Agent: "claude"}, p, Overrides{Model: "flag"}, true)
	if s.ModelFor("review") != "flag" {
		t.Error("--model overrides every phase")
	}
	if _, err := Resolve(User{Agent: "claude"}, Project{Models: map[string]string{"deploy": "x"}}, Overrides{}, true); err == nil || !strings.Contains(err.Error(), "models.deploy") {
		t.Errorf("an unknown phase is an error: %v", err)
	}
}

func TestRiskSettings(t *testing.T) {
	s, err := Resolve(User{Agent: "claude"}, Project{}, Overrides{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.Risk.MaxLines != risk.DefaultMaxLines || s.Risk.Floor != risk.Passive || s.MutationFrom != risk.Medium {
		t.Fatalf("defaults: %+v %s", s.Risk, s.MutationFrom)
	}
	p := Project{Review: "risk"}
	p.Risk.Floor = "medium"
	p.Risk.MaxLines = 100
	p.Quality.MutationFrom = "high"
	if s, err = Resolve(User{Agent: "claude"}, p, Overrides{}, true); err != nil || s.Risk.Floor != risk.Medium || s.Risk.MaxLines != 100 || s.MutationFrom != risk.High || s.Review != "risk" {
		t.Fatalf("configured: %+v %v", s, err)
	}
	if s, _ = Resolve(User{Agent: "claude"}, p, Overrides{Strict: true}, true); s.Risk.Floor != risk.High {
		t.Error("--strict raises every change to high")
	}
	bad := Project{}
	bad.Risk.Floor = "extreme"
	bad.Risk.HighPaths = []string{"("}
	if _, err := Resolve(User{Agent: "claude"}, bad, Overrides{}, true); err == nil || !strings.Contains(err.Error(), "extreme") || !strings.Contains(err.Error(), "risk.high_paths") {
		t.Fatalf("every invalid risk setting is reported: %v", err)
	}
}

func TestPlanSurfacesAndDeliveryBudget(t *testing.T) {
	s, err := Resolve(User{Agent: "claude"}, Project{}, Overrides{}, true)
	if err != nil || s.Surfaces != "ask" || s.BudgetLines != 400 {
		t.Fatalf("defaults: %q %d %v", s.Surfaces, s.BudgetLines, err)
	}
	p := Project{}
	p.Plan.Surfaces = "Strict"
	p.Delivery.BudgetLines = 250
	if s, err = Resolve(User{Agent: "claude"}, p, Overrides{}, true); err != nil || s.Surfaces != "strict" || s.BudgetLines != 250 {
		t.Fatalf("configured: %q %d %v", s.Surfaces, s.BudgetLines, err)
	}
	p.Plan.Surfaces = "sometimes"
	if _, err := Resolve(User{Agent: "claude"}, p, Overrides{}, true); err == nil || !strings.Contains(err.Error(), "plan.surfaces") {
		t.Fatalf("invalid mode: %v", err)
	}
}

func TestLensesAreAutoOffOrAList(t *testing.T) {
	root := t.TempDir()
	load := func(yaml string) (Settings, error) {
		if err := os.WriteFile(filepath.Join(root, ProjectFile), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := LoadProject(root)
		if err != nil {
			return Settings{}, err
		}
		return Resolve(User{Agent: "claude"}, p, Overrides{}, true)
	}
	if s, err := load("language: en\n"); err != nil || !s.LensesAuto {
		t.Fatalf("default is auto: %+v %v", s.Lenses, err)
	}
	if s, err := load("lenses: off\n"); err != nil || s.LensesAuto || len(s.Lenses) != 0 {
		t.Fatalf("off: %+v %v", s.Lenses, err)
	}
	if s, err := load("lenses: [risk, reliability]\n"); err != nil || s.LensesAuto || len(s.Lenses) != 2 {
		t.Fatalf("list: %+v %v", s.Lenses, err)
	}
	if _, err := load("lenses: style\n"); err == nil || !strings.Contains(err.Error(), "lenses:") {
		t.Fatalf("unknown lens: %v", err)
	}
}

func TestVerifyMode(t *testing.T) {
	s, err := Resolve(User{Agent: "claude"}, Project{}, Overrides{}, true)
	if err != nil || s.Verify != "high" || s.VerifyMaxBytes != 0 {
		t.Fatalf("default: %q %d %v", s.Verify, s.VerifyMaxBytes, err)
	}
	if s, err = Resolve(User{Agent: "claude"}, Project{Verify: "Feature", VerifyMaxMB: 50}, Overrides{}, true); err != nil || s.Verify != "feature" || s.VerifyMaxBytes != 50<<20 {
		t.Fatalf("configured: %q %d %v", s.Verify, s.VerifyMaxBytes, err)
	}
	if _, err := Resolve(User{Agent: "claude"}, Project{Verify: "sometimes"}, Overrides{}, true); err == nil {
		t.Fatal("unknown mode")
	}
}

func TestGuardSettings(t *testing.T) {
	s, err := Resolve(User{Agent: "claude"}, Project{}, Overrides{}, true)
	if err != nil || s.GuardMode != "block" {
		t.Fatalf("default: %q %v", s.GuardMode, err)
	}
	p := Project{}
	p.Guard.Mode, p.Guard.Allow = "Confirm", []string{"git push --force-with-lease origin claude/*"}
	if s, err = Resolve(User{Agent: "claude"}, p, Overrides{}, true); err != nil || s.GuardMode != "confirm" || len(s.GuardAllow) != 1 {
		t.Fatalf("configured: %+v %v", s.GuardMode, err)
	}
	p.Guard.Mode = "yolo"
	if _, err := Resolve(User{Agent: "claude"}, p, Overrides{}, true); err == nil {
		t.Fatal("unknown mode")
	}
}
