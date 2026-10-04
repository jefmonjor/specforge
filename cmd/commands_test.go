package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/config"
	"specforge/internal/domain/spec"
)

func TestVersion(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "version")
	if !strings.HasPrefix(h.out.String(), "specforge ") {
		t.Fatalf("version output %q", h.out)
	}
}

func TestInit(t *testing.T) {
	h := newHarness(t)
	h.expect(1, "init") // no terminal and no --agent: nothing to guess from
	h.expect(1, "init", "--agent", "copilot")

	h.expect(0, "init", "--agent", "Claude", "--language", "es")
	u, err := config.LoadUser(h.home)
	if err != nil || u.Agent != "claude" || u.Language != "es" {
		t.Fatalf("saved %+v %v", u, err)
	}
	if info, err := os.Stat(h.home.UserFile()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config file mode: %v %v", info, err)
	}

	// Interactive: numbered answers.
	h.tty, h.stdin = true, "2\n2\n"
	h.expect(0, "init")
	u, _ = config.LoadUser(h.home)
	if u.Agent != "gemini" || u.Language != "en" {
		t.Fatalf("interactive init saved %+v", u)
	}
}

func TestSetupNeedsAnAgent(t *testing.T) {
	h := newHarness(t)
	h.expect(4, "setup")
	if !strings.Contains(h.err.String(), "specforge init") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	h.expect(0, "setup", "--agents", "gemini")
	if !strings.Contains(h.read("GEMINI.md"), "Ask, don't invent") {
		t.Fatal("GEMINI.md has no rules")
	}
}

func TestSetupAsksWhichStackWhenSeveral(t *testing.T) {
	h := newHarness(t)
	h.write("go.mod", "module x\n")
	h.write("package.json", "{}")
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(1, "setup")
	if !strings.Contains(h.err.String(), "several stacks") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	h.tty, h.stdin = true, "node\n"
	h.expect(0, "setup")
	if !strings.Contains(h.read("specforge.yaml"), "stack: node") {
		t.Fatalf("specforge.yaml:\n%s", h.read("specforge.yaml"))
	}
}

func TestSpecLifecycle(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(0, "setup")
	h.expect(0, "spec", "new", "Password", "reset")
	if strings.TrimSpace(h.out.String()) != "specs/0001-password-reset.md" {
		t.Fatalf("stdout %q", h.out)
	}
	h.expect(3, "spec", "lint")
	h.expect(3, "spec", "approve", "1", "--by", "Ana")

	h.write("specs/0001-password-reset.md", readySpec)
	h.expect(0, "spec", "lint", "1")
	h.expect(0, "spec", "approve", "0001", "--by", "Ana")
	sealed := h.read("specs/0001-password-reset.md")
	if err := spec.Verify(sealed); err != nil {
		t.Fatal(err)
	}
	m, _, _ := spec.ReadMeta(sealed)
	if m.ApprovedBy != "Ana" || m.Status != spec.StatusApproved {
		t.Fatalf("meta %+v", m)
	}

	h.expect(0, "spec", "new", "Payments")
	h.expect(0, "spec", "list")
	lines := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "approved") || !strings.Contains(lines[1], "draft") {
		t.Fatalf("list:\n%s", h.out)
	}
	// Two specifications and no argument: ask, never pick.
	h.expect(3, "spec", "lint")
	if !strings.Contains(h.err.String(), "Which specification?") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	h.expect(3, "spec", "lint", "42")
}

func TestSpecApproveAsksForTheApprover(t *testing.T) {
	h := newHarness(t)
	h.write("specs/0001-reset.md", readySpec)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "none"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	h.expect(1, "spec", "approve")
	h.tty, h.stdin = true, "Luis\n"
	h.expect(0, "spec", "approve")
	m, _, _ := spec.ReadMeta(h.read("specs/0001-reset.md"))
	if m.ApprovedBy != "Luis" {
		t.Fatalf("approver %q", m.ApprovedBy)
	}
}

func TestSpecInterviewTurnByTurn(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(0, "spec", "new", "Password reset")
	h.agent.rules = []rule{
		{when: "Answer: **Any registered user**", files: map[string]string{"specs/0001-password-reset.md": readySpec},
			reply: "```json\n{\"status\":\"done\",\"files_written\":[\"specs/0001-password-reset.md\"],\"unknowns\":[]}\n```"},
		{when: "# Task: INTERVIEW", reply: "```json\n{\"status\":\"needs_clarification\",\"question\":\"Who can reset a password?\",\"section\":\"2. Actors\",\"unknowns\":[\"actors\"]}\n```"},
	}
	// No terminal: the question waits in questions.md.
	h.expect(5, "spec", "interview")
	if !strings.Contains(h.err.String(), "section 2. Actors · 1 unknown(s) left") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	if !strings.Contains(h.read("specs/0001-password-reset/questions.md"), "Who can reset a password?") {
		t.Fatalf("questions.md:\n%s", h.read("specs/0001-password-reset/questions.md"))
	}
	// At a terminal the same run continues with the answer.
	h.tty, h.stdin = true, "Any registered user\n"
	h.expect(0, "spec", "interview")
	if !strings.Contains(h.err.String(), "interview closed") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	if !strings.Contains(h.read("specs/0001-password-reset/interview.jsonl"), `"answer":"Any registered user"`) {
		t.Fatalf("interview.jsonl:\n%s", h.read("specs/0001-password-reset/interview.jsonl"))
	}
	h.expect(0, "spec", "approve", "--by", "Ana")

	// --chat hands the terminal over, so it needs one.
	h.tty = false
	h.expect(1, "spec", "interview", "--chat")
	h.tty = true
	h.expect(0, "spec", "interview", "--chat")
}

func TestE2EValidatesBeforeLaunchingABrowser(t *testing.T) {
	h := newHarness(t)
	h.write("specs/0001-reset.md", readySpec)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(1, "e2e")
	h.expect(1, "e2e", "--url", "localhost:3000")
	h.expect(1, "e2e", "--url", "http://localhost:3000", "--min-pass-rate", "120")
	h.expect(3, "e2e", "--url", "http://localhost:3000") // not approved
	if h.browserLaunched {
		t.Fatal("no browser may start for an unapproved specification")
	}
	h.expect(0, "spec", "approve", "--by", "Ana")
	h.expect(1, "e2e", "--url", "http://localhost:3000")
	if !h.browserLaunched {
		t.Fatal("the browser should have been launched")
	}
}

func TestAuditValidatesTheThreshold(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(1, "audit", "--fail-on", "hihg")
}

func TestAuditWithNoChanges(t *testing.T) {
	requireTool(t, "git")
	h := newHarness(t)
	h.write("main.go", "package main\n")
	gitInit(t, h.root)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(0, "audit", "--base", "HEAD")
	if !strings.Contains(h.err.String(), "no changes to audit") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	if len(h.agent.prompts) != 0 {
		t.Fatal("the agent must not be called when there is nothing to audit")
	}
}

const readySpec = "# Password reset\n\n" +
	"```gherkin\nFeature: Password reset\n\n" +
	"  Scenario: Request a link\n    Given a registered user\n    When she asks for a reset\n    Then she gets a link\n```\n"

func TestSpecClarifyThenApproveShowsTheDelta(t *testing.T) {
	h := newHarness(t)
	h.write("specs/0001-reset.md", readySpec+"\n- [NEEDS CLARIFICATION]: Which channel?\n")
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(5, "spec", "clarify", "--by", "Ana")
	h.tty, h.stdin = true, "email\n"
	h.expect(0, "spec", "clarify", "--by", "Ana")
	if !strings.Contains(h.read("specs/0001-reset.md"), "**Decided:** Which channel? → email") {
		t.Fatalf("spec:\n%s", h.read("specs/0001-reset.md"))
	}
	h.tty = false
	h.expect(0, "spec", "approve", "--by", "Ana")
	h.write("specs/0001-reset.md", strings.Replace(h.read("specs/0001-reset.md"), "she gets a link", "she gets an email", 1))
	h.expect(0, "spec", "approve", "--by", "Ana")
	if !strings.Contains(h.err.String(), "MODIFIED · Request a link") {
		t.Fatalf("stderr:\n%s", h.err)
	}
}

func TestJSONOutput(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "version", "--json")
	var v map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &v); err != nil || v["version"] == "" {
		t.Fatalf("version json %q: %v", h.out, err)
	}
	h.write("specs/0001-reset.md", readySpec)
	h.expect(0, "spec", "list", "--json")
	var rows []map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0]["state"] != "draft" || rows[0]["path"] != "specs/0001-reset.md" {
		t.Fatalf("list json %q: %v", h.out, err)
	}
	h.expect(4, "loop", "--json") // no agent configured
	var d map[string]any
	if err := json.Unmarshal(h.err.Bytes(), &d); err != nil || d["exit"] != float64(4) || d["action"] != "run `specforge init`" {
		t.Fatalf("error json %q: %v", h.err, err)
	}
}
