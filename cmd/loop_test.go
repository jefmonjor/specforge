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
	}
	h.expect(0, "loop")
	if !strings.Contains(h.err.String(), "1 scenario(s) passed") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	calls := len(h.agent.prompts)
	if calls != 2 {
		t.Fatalf("want one RED and one GREEN call, got %d", calls)
	}
	if got := h.read("reset/reset.go"); !strings.Contains(got, "https://example.com/reset/") {
		t.Fatalf("implementation:\n%s", got)
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
