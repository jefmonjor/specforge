# Task: GREEN — make the failing test pass with the minimum code

You are working on the specification **{{.SpecTitle}}** (`{{.SpecPath}}`) in a {{.Stack}} project.

## Scenario {{.Index}} of {{.Total}}
```gherkin
{{.Scenario}}```

## The test `{{.Marker}}` fails
```
{{.LastFailure}}
```

## Rules
1. Change production code only. **Do not create, edit, rename or delete any test file**: SpecForge fingerprints them and stops the loop if one changes.
2. Write the minimum code that makes this test pass (YAGNI). Keep the rest of the suite green.
3. Name things with the ubiquitous language below.
4. If the scenario leaves a business rule undefined, ask (`needs_clarification`) instead of choosing for the developer.

SpecForge will run `{{.TestCommand}}` and expects it to pass.
{{template "turn" .}}
{{- template "context" .}}
{{- template "contract" .}}
