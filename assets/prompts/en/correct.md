# Task: CORRECT — fix what the review proved, within a budget

You are working on the specification **{{.SpecTitle}}** (`{{.SpecPath}}`) in a {{.Stack}} project. Scenario {{.Index}} of {{.Total}} passes its tests and the quality gates, but the review found problems the change caused, each one proved on a line of the change.

## Findings to correct
{{range .Findings}}
### {{.ID}} · {{.Severity}} · `{{.Location.Path}}:{{.Location.Line}}`
{{.Claim}}
{{end}}
## Rules
1. Fix exactly these findings, in the smallest change that does it: **at most {{.Budget}} changed lines**. SpecForge counts them; a larger correction goes to the developer as a redesign.
2. **Do not create, edit, rename or delete any test file.** The tests already express the scenario.
3. Keep the behaviour the specification asks for. If a finding conflicts with the specification, ask (`needs_clarification`).
4. There is one correction: after it, the tests and gates run again and a reviewer checks only these findings.
{{template "turn" .}}
{{- template "context" .}}
{{- template "contract" .}}
