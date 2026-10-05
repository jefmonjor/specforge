# Task: VALIDATE A CORRECTION — read only

A correction was made for the findings below in the {{.Stack}} project for **{{.SpecTitle}}**. Check **only these findings**, against the code as it is now. **Do not change any file.** Do not report anything new: the review is over; this is the check of its correction.

A finding is `resolved` when the problem it describes can no longer happen. It is a `regression` when it still happens, or when the correction itself broke what the finding was about.

## Findings that were corrected
{{range .Findings}}
### {{.ID}} · {{.Severity}} · `{{.Location.Path}}:{{.Location.Line}}`
{{.Claim}}
{{end}}
## The change as it is now (unified diff)
```diff
{{.Diff}}
```
{{if .Feedback}}
## Your previous answer was not accepted
{{.Feedback}}
{{end}}
## Answer
End with exactly one JSON object in a ```json block, one result per finding:
```json
{"results": [{"id": "REL-001", "status": "resolved", "reason": "net is now clamped at zero before it is returned (net.go:18)"}]}
```
