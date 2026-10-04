package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
