package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"specforge/internal/domain/spec"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "--local", "user.name", "Test"},
		{"config", "--local", "user.email", "test@example.com"},
		{"config", "--local", "commit.gpgsign", "false"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func done(files ...string) string {
	return "```json\n{\"status\":\"done\",\"files_written\":[\"" + strings.Join(files, `","`) + "\"]}\n```"
}

// loopProject is a Go module with an approved single-scenario spec.
func loopProject(t *testing.T) *harness {
	t.Helper()
	requireTool(t, "go")
	requireTool(t, "git")
	h := newHarness(t)
	h.write("go.mod", "module example.com/reset\n\ngo 1.22\n")
	h.write("specs/0001-reset.md", readySpec)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(0, "setup")
	h.expect(0, "spec", "approve", "--by", "Ana")
	gitInit(t, h.root)
	return h
}

// cleanLens is a review lens that finds nothing.
var cleanLens = rule{when: "# Task: REVIEW · reliability lens", reply: "```json\n{\"lens\":\"reliability\",\"findings\":[],\"evidence\":[\"read the diff\"]}\n```"}

func TestLoopRunsRedGreenRefactor(t *testing.T) {
	h := loopProject(t)
	h.agent.rules = []rule{
		{when: "# Task: RED", files: map[string]string{
			"reset/reset_test.go": "package reset\n\nimport \"testing\"\n\nfunc TestSDD_0001_001(t *testing.T) {\n\tif Link(\"ana\") == \"\" {\n\t\tt.Fatal(\"no link\")\n\t}\n}\n",
			"reset/reset.go":      "package reset\n\nfunc Link(user string) string { return \"\" }\n",
		}, reply: done("reset/reset_test.go", "reset/reset.go")},
		{when: "# Task: GREEN", files: map[string]string{
			"reset/reset.go": "package reset\n\nfunc Link(user string) string { return \"https://example.com/reset/\" + user }\n",
		}, reply: done("reset/reset.go")},
		cleanLens,
	}
	// R2: without a terminal the review is a question in questions.md.
	h.expect(5, "loop")
	if !strings.Contains(h.read("specs/0001-reset/questions.md"), "Review scenario 1 (Request a link)") {
		t.Fatalf("questions.md:\n%s", h.read("specs/0001-reset/questions.md"))
	}
	calls := len(h.agent.prompts)
	if calls != 3 || !strings.Contains(h.agent.prompts[2], "# Task: REVIEW · reliability lens") {
		t.Fatalf("want RED, GREEN and the reliability lens, got %d calls", calls)
	}
	if !strings.Contains(h.read("specs/0001-reset/review/SDD_0001_001.json"), `"reliability"`) {
		t.Fatal("the lens review is kept next to the specification")
	}
	if got := h.read("reset/reset.go"); !strings.Contains(got, "https://example.com/reset/") {
		t.Fatalf("implementation:\n%s", got)
	}

	// Accepting in the file and resuming records the scenario as a commit.
	q := h.read("specs/0001-reset/questions.md")
	h.write("specs/0001-reset/questions.md", strings.Replace(q, "_awaiting an answer_", "1", 1))
	h.expect(0, "loop", "--resume")
	if !strings.Contains(h.err.String(), "1 scenario(s) passed") || !strings.Contains(h.err.String(), "committed ") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	log := gitOut(t, h.root, "log", "-1", "--name-only", "--format=%s")
	for _, want := range []string{"feat(SDD_0001_001): Request a link", "reset/reset.go", "reset/reset_test.go"} {
		if !strings.Contains(log, want) {
			t.Fatalf("last commit lacks %q:\n%s", want, log)
		}
	}
	if len(h.agent.prompts) != calls {
		t.Fatal("accepting a review must not call the agent")
	}

	// A finished, unchanged loop is reported, not redone.
	h.expect(0, "loop")
	if len(h.agent.prompts) != calls {
		t.Fatal("a finished loop must not call the agent again")
	}
}

func TestLoopRefusesAnEditedSpecification(t *testing.T) {
	h := loopProject(t)
	h.write("specs/0001-reset.md", strings.Replace(h.read("specs/0001-reset.md"), "gets a link", "gets two links", 1))
	h.expect(3, "loop")
	if len(h.agent.prompts) != 0 {
		t.Fatal("no agent call may happen on a tampered specification")
	}
}

func TestLoopQuestionWithoutTerminalExitsFive(t *testing.T) {
	h := loopProject(t)
	h.agent.rules = []rule{{when: "# Task: RED", reply: "```json\n{\"status\":\"needs_clarification\",\"question\":\"Which channel sends the link?\",\"options\":[\"email\",\"sms\"]}\n```"}}
	h.expect(5, "loop")
	q := h.read("specs/0001-reset/questions.md")
	if !strings.Contains(q, "Which channel sends the link?") {
		t.Fatalf("questions.md:\n%s", q)
	}
	// The answer is given at a terminal on resume and recorded.
	h.tty, h.stdin = true, "1\n"
	h.agent.rules = []rule{{when: "# Task: RED", reply: "```json\n{\"status\":\"blocked\",\"reason\":\"stop here\"}\n```"}}
	h.expect(2, "loop", "--resume")
	if _, err := os.Stat(h.root + "/.specforge/state/0001-reset.json"); err != nil {
		t.Fatalf("state not saved: %v", err)
	}
}

func TestLoopFlagsExcludeEachOther(t *testing.T) {
	h := newHarness(t)
	h.expect(1, "loop", "--resume", "--restart")
}

func TestApprovedSpecStillVerifies(t *testing.T) {
	h := loopProject(t)
	if err := spec.Verify(h.read("specs/0001-reset.md")); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverTracesTheFinishedLoop(t *testing.T) {
	h := loopProject(t)
	h.expect(3, "deliver", "9")

	// Before the loop: every scenario is pending and the delivery says so.
	h.expect(0, "deliver")
	if !strings.Contains(h.err.String(), "incomplete") || !strings.Contains(h.read("specs/0001-reset/DELIVERY.md"), "⏳ not finished") {
		t.Fatalf("stderr:\n%s\nDELIVERY.md:\n%s", h.err, h.read("specs/0001-reset/DELIVERY.md"))
	}

	h.agent.rules = []rule{
		{when: "# Task: RED", files: map[string]string{
			"reset/reset_test.go": "package reset\n\nimport \"testing\"\n\nfunc TestSDD_0001_001_SendsALink(t *testing.T) {\n\tif Link(\"ana\") == \"\" {\n\t\tt.Fatal(\"no link\")\n\t}\n}\n",
			"reset/reset.go":      "package reset\n\nfunc Link(user string) string { return \"\" }\n",
		}, reply: done("reset/reset_test.go", "reset/reset.go")},
		{when: "# Task: GREEN", files: map[string]string{
			"reset/reset.go": "package reset\n\nfunc Link(user string) string { return \"https://example.com/reset/\" + user }\n",
		}, reply: done("reset/reset.go")},
		cleanLens,
	}
	h.tty, h.stdin = true, "Accept\n"
	h.expect(0, "loop")
	h.tty = false
	h.write(".github/pull_request_template.md", "## Why\n\n## Checklist\n- [ ] reviewed\n")
	h.expect(0, "deliver")
	if strings.TrimSpace(h.out.String()) != "specs/0001-reset/DELIVERY.md\nspecs/0001-reset/trace.json\nspecs/0001-reset/PR_BODY.md" {
		t.Fatalf("stdout:\n%s", h.out)
	}
	sha := strings.TrimSpace(gitOut(t, h.root, "rev-parse", "--short=7", "HEAD"))
	md := h.read("specs/0001-reset/DELIVERY.md")
	for _, want := range []string{
		"# Delivery · 0001 Password reset",
		"approved by Ana on 2026-10-04",
		"1/1 finished · 1 through RED → GREEN → REFACTOR",
		"| 1 | Request a link | `reset/reset_test.go` · `TestSDD_0001_001_SendsALink` | `" + sha + "` |",
		"## Decisions taken during development\n\n- none",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("DELIVERY.md lacks %q:\n%s", want, md)
		}
	}
	body := h.read("specs/0001-reset/PR_BODY.md")
	if !strings.HasPrefix(body, "## Why\n\nImplements specification 0001") || !strings.Contains(body, "- [ ] reviewed") {
		t.Fatalf("PR_BODY.md:\n%s", body)
	}
	if !strings.Contains(h.read("specs/0001-reset/trace.json"), `"marker": "SDD_0001_001"`) {
		t.Fatalf("trace.json:\n%s", h.read("specs/0001-reset/trace.json"))
	}
}
