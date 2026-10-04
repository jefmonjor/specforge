# Task: REFACTOR — fix the findings without changing behaviour

You are working on the specification **{{.SpecTitle}}** (`{{.SpecPath}}`) in a {{.Stack}} project. Scenario {{.Index}} of {{.Total}} passes its test, but the project is not clean yet.
{{if .SuiteFailure}}
## The full test suite fails
```
{{.SuiteFailure}}
```
{{end}}
{{- if .GateReport}}
## Quality gates that failed
```
{{.GateReport}}
```
{{end}}
## Rules
1. Fix exactly what is reported above; keep the behaviour unchanged.
2. **Do not create, edit, rename or delete any test file.** If a finding can only be fixed by changing a test, reply `blocked` and explain why.
3. Prefer removing duplication and dead code over adding abstractions.
4. If a finding conflicts with the specification, ask (`needs_clarification`).
{{if .Answer}}
## Your question was answered
You asked: {{.AnsweredQuestion}}
The developer answered: **{{.Answer}}**
It is recorded in the decisions log. Continue the task with it.
{{end}}
{{if .Feedback}}
## Your previous attempt ({{.Attempt}} of {{.MaxAttempts}}) was rejected
{{.Feedback}}
{{end}}
{{- template "context" .}}
{{- template "contract" .}}
