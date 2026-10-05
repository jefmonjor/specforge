# Task: PLAN — decide where the code goes, before any code is written

You are working on the approved specification **{{.SpecTitle}}** (`{{.SpecPath}}`) in a {{.Stack}} project.

## Specification
{{.Spec}}

## Files in the project
```
{{.Tree}}
```

## What to do
Write the technical plan to `{{.PlanPath}}` and change no other file. Use these sections:

1. `## Approach`: at most five sentences on how the scenarios will be implemented, in the project's existing style.
2. `## Components`: one line per file to create or change, with the path in backticks, like this line: - `internal/pay/net.go`: computes the net salary (new). SpecForge lets the agent edit only these files, the planned test files and new files under the directory of a new component.
3. `## Tests per scenario`: a table with one row per scenario: marker, scenario title, test file (in backticks), test name. Every marker must appear: {{range $i, $m := .Markers}}{{if $i}}, {{end}}`{{$m}}`{{end}}.
4. `## Interfaces and dependencies`: the interfaces (ports) the domain needs and which existing code or library implements them. Prefer what the project already uses; add no dependency the specification does not require.
5. `## Risks`: what could make a scenario harder than it looks.

Keep it short: the plan is reviewed by a person before any code exists. Do not write code or tests. If an architectural choice is not settled by the specification, the existing code or the decisions log, ask instead of choosing.
{{- if .Draft}}
## Current draft (revise it; keep what is still right)
````markdown
{{.Draft}}
````
{{end}}
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
