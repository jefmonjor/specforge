# Task: REVIEW · {{.Lens}} lens — read only

You review a change to the {{.Stack}} project for the specification **{{.SpecTitle}}**{{if .Scenario}}, scenario {{.Marker}}{{end}}. **Do not change any file**: SpecForge checks that nothing changed and refuses the review otherwise.

## Your lens: {{.Lens}}
{{- if eq .Lens "risk"}}
Security and safety: injection, authentication and authorization, secrets in code or logs, unsafe deserialization, path traversal, process execution, data exposure, money and rounding, permissions.
{{- else if eq .Lens "reliability"}}
Correctness against the specification and its invariants: wrong results, missed edge cases (empty, zero, negative, limits, time zones), error handling that hides failures, behaviour the specification does not ask for.
{{- else if eq .Lens "readability"}}
Clarity for the next developer: names that do not follow the ubiquitous language, functions doing several things, duplication, dead code, comments that lie, layers mixed (domain doing I/O).
{{- else}}
Resilience under failure: timeouts, retries without limits, partial writes, concurrency and shared state, resource leaks (files, connections, goroutines), behaviour when a dependency is down.
{{- end}}
Report only what this lens is about; the other lenses cover the rest.
{{if .Scenario}}
## The scenario
```gherkin
{{.Scenario}}
```
{{end}}
{{- if .Invariants}}
## Domain invariants
{{.Invariants}}
{{end}}
{{- if .Plan}}
## Approved plan
{{.Plan}}
{{end}}
## The change (unified diff)
```diff
{{.Diff}}
```

## Rules for every finding
1. Point at the change. `proof_refs` must name a line the diff adds or modifies (`"kind": "changed-hunk"`, with `path` and the line number on the new side) or a file it creates (`"kind": "new-file"`). SpecForge checks every reference against the diff and **discards** a finding whose proof is outside it.
2. Say how you know: `evidence_class` is `deterministic` when it follows from the code alone, `inferential` when it depends on how the code is used (it will be refuted before it can block), `insufficient` when you cannot tell.
3. Say whether the change caused it: `causal_disposition` is `introduced`, `behavior-activated` (old code, newly reachable) or `worsened`; `pre-existing` or `base-only` when it was already there (a follow-up, never blocking); `unknown` when you cannot tell (the developer decides).
4. `severity`: `BLOCKER` or `CRITICAL` only for what must not be merged; `WARNING` and `SUGGESTION` are information.
5. IDs: `{{.Prefix}}-001`, `{{.Prefix}}-002`… No finding is better than an invented one.
{{if .Feedback}}
## Your previous answer was not accepted
{{.Feedback}}
{{end}}
## Answer
End with exactly one JSON object in a ```json block:
```json
{"lens": "{{.Lens}}", "findings": [{"id": "{{.Prefix}}-001", "severity": "CRITICAL", "location": {"path": "src/pay/net.go", "line": 42}, "claim": "what is wrong and why it matters, in one or two sentences", "evidence_class": "deterministic", "causal_disposition": "introduced", "proof_refs": [{"kind": "changed-hunk", "path": "src/pay/net.go", "line": 42}]}], "evidence": ["what you read or ran to reach this"]}
```
With no findings, `"findings": []`; `evidence` is never empty.
