# Task: RED — write the failing test for one scenario

You are working on the specification **{{.SpecTitle}}** (`{{.SpecPath}}`) in a {{.Stack}} project.

## Scenario {{.Index}} of {{.Total}}
```gherkin
{{.Scenario}}```

## What to do
1. Write the automated test that verifies this scenario's `Then` steps through observable behaviour, not implementation details. One test per scenario.
2. The test name **must contain the marker `{{.Marker}}`** so SpecForge can run it alone. {{if .MarkerHint}}Example: `{{.MarkerHint}}`{{end}}
3. The test must **compile and fail on an assertion**. If the production types or functions it needs do not exist yet, add the smallest stubs that let it compile (signatures that return zero values or raise "not implemented"). Do not implement any behaviour.
4. Put the test where this project keeps its tests; extend an existing test file of this feature when there is one.
5. Do not modify any other test.

SpecForge will run `{{.TestCommand}}` and expects it to fail on your assertion.
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
{{- if .LastFailure}}
### Output of the previous run
```
{{.LastFailure}}
```
{{end}}
{{- template "context" .}}
{{- template "contract" .}}
