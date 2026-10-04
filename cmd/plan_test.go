package cmd

import (
	"strings"
	"testing"
)

const planBody = "## Approach\nA pure function.\n\n## Tests per scenario\n| Marker | Test file |\n| SDD_0001_001 | reset/reset_test.go |\n"

func TestPlanDraftApproveAndLoopFollowsIt(t *testing.T) {
	h := loopProject(t)
	h.agent.rules = []rule{{when: "# Task: PLAN", files: map[string]string{"specs/0001-reset/plan.md": planBody}, reply: done("specs/0001-reset/plan.md")}}
	h.expect(0, "plan")
	plan := h.read("specs/0001-reset/plan.md")
	if !strings.HasPrefix(plan, "---\nspec: \"0001\"\nstatus: \"draft\"") || !strings.Contains(plan, "SDD_0001_001") {
		t.Fatalf("plan.md:\n%s", plan)
	}
	if !strings.Contains(h.agent.prompts[0], "Every marker must appear: `SDD_0001_001`") {
		t.Fatalf("plan prompt:\n%s", h.agent.prompts[0])
	}

	// A draft plan stops the loop before any agent call.
	calls := len(h.agent.prompts)
	h.expect(3, "loop")
	if len(h.agent.prompts) != calls || !strings.Contains(h.err.String(), "plan approve") {
		t.Fatalf("stderr:\n%s", h.err)
	}

	h.expect(0, "plan", "approve", "--by", "Ana")
	h.agent.rules = []rule{{when: "# Task: RED", reply: "```json\n{\"status\":\"blocked\",\"reason\":\"stop\"}\n```"}}
	h.expect(2, "loop")
	if red := h.agent.prompts[len(h.agent.prompts)-1]; !strings.Contains(red, "## Approved technical plan") || !strings.Contains(red, "A pure function.") {
		t.Fatalf("RED prompt lacks the plan:\n%s", red)
	}

	// Editing the approved plan stops the loop until it is approved again.
	h.write("specs/0001-reset/plan.md", strings.Replace(h.read("specs/0001-reset/plan.md"), "A pure function.", "A struct.", 1))
	h.expect(3, "loop", "--restart")
}

func TestPlanRejectsWorkOutsideThePlan(t *testing.T) {
	h := loopProject(t)
	h.agent.rules = []rule{{when: "# Task: PLAN", files: map[string]string{
		"specs/0001-reset/plan.md": planBody, "reset/reset.go": "package reset\n",
	}, reply: done("specs/0001-reset/plan.md", "reset/reset.go")}}
	h.expect(2, "plan")
	if !strings.Contains(h.err.String(), "reset/reset.go") {
		t.Fatalf("stderr:\n%s", h.err)
	}
}

func TestPlanMustPlaceEveryScenario(t *testing.T) {
	h := loopProject(t)
	h.agent.rules = []rule{{when: "# Task: PLAN", files: map[string]string{"specs/0001-reset/plan.md": "## Approach\nSomething.\n"}, reply: done("specs/0001-reset/plan.md")}}
	h.expect(2, "plan")
	if len(h.agent.prompts) != 3 || !strings.Contains(h.agent.prompts[1], "no planned test for SDD_0001_001") {
		t.Fatalf("calls=%d second prompt:\n%s", len(h.agent.prompts), h.agent.prompts[1])
	}
	h.expect(3, "plan", "approve", "--by", "Ana")
}

func TestPlanNeedsAnApprovedSpec(t *testing.T) {
	h := newHarness(t)
	h.write("specs/0001-reset.md", readySpec)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(3, "plan")
	h.expect(3, "plan", "approve", "--by", "Ana")
}
