package cmd

import (
	"strings"
	"testing"
)

func hookInput(command string) string {
	return `{"tool_name":"Bash","tool_input":{"command":` + quote(command) + `}}`
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func TestGuardHookBlocksAndExplains(t *testing.T) {
	h := newHarness(t)
	h.stdin = hookInput("git reset --hard HEAD~3")
	h.expect(2, "guard", "--hook", "claude")
	if !strings.Contains(h.err.String(), "git reset --hard discards uncommitted work") || strings.Contains(h.err.String(), "SpecForge stopped") {
		t.Fatalf("the agent sees the reason, without a diagnosis box:\n%s", h.err)
	}
	h.stdin = hookInput("go test ./...")
	h.expect(0, "guard", "--hook", "claude")
}

func TestGuardConfirmAsksOutsideTheLoopOnly(t *testing.T) {
	h := newHarness(t)
	h.write("specforge.yaml", "guard:\n  mode: confirm\n")
	h.stdin = hookInput("git clean -fdx")
	h.expect(0, "guard", "--hook", "claude")
	if !strings.Contains(h.out.String(), `"permissionDecision":"ask"`) {
		t.Fatalf("Claude Code asks the developer:\n%s", h.out)
	}
	h.env = map[string]string{"SPECFORGE_LOOP": "1"}
	h.stdin = hookInput("git clean -fdx")
	h.expect(2, "guard", "--hook", "claude")
}

func TestGuardAllowListAndFailClosed(t *testing.T) {
	h := newHarness(t)
	h.write("specforge.yaml", "guard:\n  allow: [\"git push --force-with-lease origin claude/*\"]\n")
	h.stdin = hookInput("git push --force-with-lease origin claude/fix")
	h.expect(0, "guard", "--hook", "claude")
	h.stdin = "not json"
	h.expect(2, "guard", "--hook", "claude")
	h.write("specforge.yaml", "guard:\n  mode: yolo\n")
	h.stdin = hookInput("git push --force-with-lease origin claude/fix")
	h.expect(2, "guard", "--hook", "claude") // invalid configuration: no allow list
	h.expect(0, "guard", "--selftest")
}

func TestSetupInstallsTheGuardHook(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "setup", "--agents", "claude")
	if !strings.Contains(h.read(".claude/settings.json"), "guard --hook claude") {
		t.Fatalf("settings:\n%s", h.read(".claude/settings.json"))
	}
	h.expect(0, "setup", "--agents", "claude")
	if strings.Count(h.read(".claude/settings.json"), "guard --hook claude") != 1 {
		t.Fatal("setup is idempotent")
	}
}
