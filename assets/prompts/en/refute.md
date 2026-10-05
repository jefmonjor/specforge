# Task: REFUTE — read only

Review lenses reported the findings below about a change to the {{.Stack}} project for **{{.SpecTitle}}**. Each one is *inferential*: it depends on how the code is used. Your job is to try to **refute** each one, independently of whoever found it. **Do not change any file.**

For each finding, read the code around it and the code that calls it. It is `refuted` when something in the code makes it impossible (a type that cannot hold the bad value, a check upstream, a caller that never passes it). It is `confirmed` when you can follow a real path to the problem. When you cannot settle it, it is `confirmed`: only a refutation you can show removes a finding.

## Findings
{{range .Findings}}
### {{.ID}} · {{.Severity}} · `{{.Location.Path}}:{{.Location.Line}}`
{{.Claim}}
{{end}}
## The change (unified diff)
```diff
{{.Diff}}
```
{{if .Feedback}}
## Your previous answer was not accepted
{{.Feedback}}
{{end}}
## Answer
End with exactly one JSON object in a ```json block, one verdict per finding:
```json
{"verdicts": [{"id": "REL-001", "verdict": "refuted", "reason": "Money rejects negative amounts in its constructor (money.go:14), so bonus is never negative"}]}
```
