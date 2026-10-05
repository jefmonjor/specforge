package cmd

import (
	"strings"
	"testing"
)

const payrollServlet = "package com.acme.payroll;\n\nimport javax.servlet.http.HttpServlet;\n\npublic class PayrollServlet extends HttpServlet {\n  // net = gross - 15% tax, rounded down\n  int net(int gross) { return gross - gross * 15 / 100; }\n}\n"

const payrollSpec = "---\nid: \"0001\"\ntitle: \"Pay an employee\"\nstatus: draft\n---\n\n# 0001 · Pay an employee\n\n" +
	"## 4. Invariants\n\n- **INV-01**: the tax withheld is 15% of the gross, rounded down.\n\n## 6. Scenarios\n\n" +
	"```gherkin\nFeature: Pay an employee\n\n  Scenario: INV-01 net pay withholds 15%\n    Given an employee with a gross of 1000\n    When the employee is paid\n    Then the net is 850\n```\n\n" +
	"## 12. Open questions\n\n- [NEEDS CLARIFICATION]: is rounding down on purpose?\n\n" +
	"## 13. Legacy sources\n\n- INV-01: `src/com/acme/payroll/PayrollServlet.java:6-7`\n"

func TestLegacyRewriteFromScanToApproval(t *testing.T) {
	h := newHarness(t)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	// Without a legacy repository there is nothing to scan.
	h.expect(4, "legacy", "scan")
	h.write("specforge.yaml", "migration:\n  legacy: old\n  java_release: 21\n")
	h.write("old/build.xml", "<project><javac source=\"1.6\" target=\"1.6\"/></project>\n")
	h.write("old/src/com/acme/payroll/PayrollServlet.java", payrollServlet)

	h.expect(0, "legacy", "scan")
	if inv := h.read("docs/legacy/INVENTORY.md"); !strings.Contains(inv, "javax.servlet") || !strings.Contains(h.err.String(), "build ant · Java 1.6") {
		t.Fatalf("inventory:\n%s\nstderr:\n%s", inv, h.err)
	}

	h.agent.rules = []rule{
		{when: "# Task: LEGACY MAP", files: map[string]string{"docs/legacy/CAPABILITIES.md": "# Capability map\n\n## Pay an employee\n- net pay `src/com/acme/payroll/PayrollServlet.java:7`\n"},
			reply: "```json\n{\"status\":\"done\",\"files_written\":[\"docs/legacy/CAPABILITIES.md\"]}\n```"},
		{when: "# Task: SPECIFICATION FROM LEGACY", files: map[string]string{"specs/0001-pay-an-employee.md": payrollSpec},
			reply: "```json\n{\"status\":\"done\",\"files_written\":[\"specs/0001-pay-an-employee.md\"]}\n```"},
	}
	h.expect(0, "legacy", "map")
	h.expect(0, "spec", "from-legacy", "Pay an employee")
	if !strings.Contains(h.err.String(), "1 open question(s)") {
		t.Fatalf("stderr:\n%s", h.err)
	}
	prompt := h.agent.prompts[len(h.agent.prompts)-1]
	if !strings.Contains(prompt, "Java 21") || !strings.Contains(prompt, "## Pay an employee") || !strings.Contains(prompt, "`javax.servlet`") {
		t.Fatalf("the prompt lacks the target, the map or the forbidden imports:\n%s", prompt)
	}
	// Running it again continues the same draft.
	h.expect(0, "spec", "from-legacy", "Pay an employee")
	h.expect(0, "spec", "list")
	if strings.Contains(h.out.String(), "0002") {
		t.Fatalf("a second specification was created:\n%s", h.out)
	}
	// The decision goes into the scenarios it affects.
	h.agent.rules = append([]rule{{when: "# Task: CHANGE", reply: "```json\n{\"status\":\"done\",\"files_written\":[],\"unknowns\":[]}\n```"}}, h.agent.rules...)
	h.tty, h.stdin = true, "Yes, payroll rounds down\n"
	h.expect(0, "spec", "clarify")
	h.expect(0, "spec", "approve", "--by", "Ana")
	h.expect(1, "spec", "from-legacy", "Pay an employee")
}
