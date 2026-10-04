{{define "contract"}}
## Response contract (mandatory)

End your answer with exactly one JSON object in a ```json block, and write nothing after it.

When you finished the task:
```json
{"status": "done", "files_written": ["relative/path/of/each/file/you/changed"], "summary": "one sentence"}
```

If anything you need is not in the specification, the decisions log, the existing code or this prompt, **do not assume it**. A wrong guess costs more than a question. Reply instead with:
```json
{"status": "needs_clarification", "question": "the exact question for the developer", "options": ["option A", "option B"], "context": "why you need it"}
```

If you cannot continue for a reason the developer must fix (a missing tool, a broken build unrelated to this task), reply with:
```json
{"status": "blocked", "reason": "what is wrong", "suggested_action": "what the developer should do"}
```

SpecForge checks `files_written` against the files that really changed. Never list a file you did not write.
{{end}}
