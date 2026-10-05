{{define "contract"}}
## Response contract (mandatory)

End your answer with exactly one JSON object in a ```json block, and write nothing after it.

When you finished the task:
```json
{"status": "done", "files_written": ["relative/path/of/each/file/you/changed"], "summary": "one sentence"}
```
If this task needed more than one attempt, add `"lesson"`: one sentence with the rule that would have avoided the mistake. It is kept in `specs/LESSONS.md` and shown in later prompts, so make it general, not about this scenario.

{{- if .Marker}}
SpecForge sizes the review of this change from the paths and lines it touches. If it is more delicate than that suggests (it handles credentials, money, permissions, user data, or changes a public contract), add `"risk": "high"` (or `"medium"`) and `"risk_reason"`: one sentence saying why. You can raise the scrutiny, never lower it.
{{end}}

If anything you need is not in the specification, the decisions log, the existing code or this prompt, **do not assume it**. A wrong guess costs more than a question. Reply instead with:
```json
{"status": "needs_clarification", "question": "the exact question for the developer", "options": ["option A", "option B"], "context": "why you need it"}
```

When your doubt is about files, paths, commands, names or technical alternatives, derive the candidate answers yourself and put at least two in `options`: the developer picks or trims them. A question without options is refused here; free text is for the developer, not for you to ask for.

If you cannot continue for a reason the developer must fix (a missing tool, a broken build unrelated to this task), reply with:
```json
{"status": "blocked", "reason": "what is wrong", "suggested_action": "what the developer should do"}
```

SpecForge checks `files_written` against the files that really changed. Never list a file you did not write.
{{end}}

{{define "turn"}}
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
{{end}}
