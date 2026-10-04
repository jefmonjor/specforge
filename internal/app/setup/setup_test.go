package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/layout"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
)

func TestRunIsIdempotentAndKeepsTheDevelopersContent(t *testing.T) {
	root := t.TempDir()
	l := layout.Layout{Root: root}
	mine := "# My project notes\n\nUse tabs.\n"
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(mine), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules"), 0o644)

	goStack := &stack.Profile{Kind: stack.Go, Runner: stack.RunnerGo, Standard: "go"}
	o := Options{Layout: l, Language: "en", Agents: []string{"claude", "gemini"}, Stack: goStack}
	changes, err := Run(fsys.OS{}, o)
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{config.ProjectFile, Created}, {"CLAUDE.md", Updated}, {"GEMINI.md", Created}, {".gitignore", Updated}}
	if len(changes) != len(want) {
		t.Fatalf("changes %v", changes)
	}
	for i := range want {
		if changes[i] != want[i] {
			t.Fatalf("changes %v, want %v", changes, want)
		}
	}

	claude, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if !strings.HasPrefix(string(claude), mine) || !strings.Contains(string(claude), "Ask, don't invent") ||
		!strings.Contains(string(claude), "### Go") || !strings.HasSuffix(string(claude), EndMarker+"\n") {
		t.Fatalf("CLAUDE.md:\n%s", claude)
	}
	ignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(ignore) != "node_modules\n.specforge/\n" {
		t.Fatalf(".gitignore: %q", ignore)
	}
	cfg, err := config.LoadProject(root)
	if err != nil || cfg.Language != "en" || cfg.Stack != "go" {
		t.Fatalf("specforge.yaml: %+v %v", cfg, err)
	}

	again, err := Run(fsys.OS{}, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range again {
		if c.Action != Unchanged {
			t.Fatalf("second run changed %v", again)
		}
	}

	// A new language replaces the block in place, keeping the rest.
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), append(claude, []byte("\n## After\n")...), 0o644)
	o.Language = "es"
	if _, err := Run(fsys.OS{}, o); err != nil {
		t.Fatal(err)
	}
	claude, _ = os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if strings.Count(string(claude), beginPrefix) != 1 || !strings.Contains(string(claude), "Pregunta, no inventes") ||
		!strings.HasPrefix(string(claude), mine) || !strings.HasSuffix(string(claude), "## After\n") {
		t.Fatalf("CLAUDE.md after language change:\n%s", claude)
	}
}

func TestRunRefusesADamagedBlock(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(BeginMarker+"\nhalf\n"), 0o644)
	_, err := Run(fsys.OS{}, Options{Layout: layout.Layout{Root: root}, Language: "en", Agents: []string{"claude"}})
	if err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("want damaged block error, got %v", err)
	}
}

func TestRunValidatesAgents(t *testing.T) {
	l := layout.Layout{Root: t.TempDir()}
	if _, err := Run(fsys.OS{}, Options{Layout: l, Language: "en"}); err == nil {
		t.Fatal("want error without agents")
	}
	if _, err := Run(fsys.OS{}, Options{Layout: l, Language: "en", Agents: []string{"copilot"}}); err == nil {
		t.Fatal("want error for an unknown agent")
	}
}

func TestBlockWithoutStackHasOnlyRules(t *testing.T) {
	b, err := Block("en", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b, "### Go") || !strings.HasPrefix(b, BeginMarker) {
		t.Fatalf("block:\n%s", b)
	}
}

func TestProjectConfigLoadsForARewrite(t *testing.T) {
	for _, legacy := range []string{"", "../old system"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, config.ProjectFile), []byte(ProjectConfig("en", nil, legacy, 21)), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := config.LoadProject(root)
		if err != nil {
			t.Fatal(err)
		}
		if legacy != "" && (p.Migration.Legacy != legacy || p.Migration.JavaRelease != 21) {
			t.Fatalf("%+v", p.Migration)
		}
	}
}
