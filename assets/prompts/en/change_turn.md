# Task: CHANGE — apply a change request to the specification

The developer asks for a change to the specification `{{.SpecPath}}` ("{{.SpecTitle}}"):

> {{.Request}}

SpecForge runs the conversation: you never talk to the developer directly. Each turn you do two things, then stop.

1. **Apply what is settled** to `{{.SpecPath}}`: the request itself, and the developer's last answer if there is one below. Change every section the request touches, not only the scenarios: invariants, data contracts, errors, out of scope, assumptions. Keep the rules of the specification: one behaviour per scenario, one `When`, at least one `Then`, no UI or technology words, every invariant `INV-NN` covered by a failure scenario that mentions it.
   - **Keep the title of every scenario whose behaviour stays the same, even if you edit its steps.** SpecForge follows each scenario by its title: an unchanged title keeps its tests and its history. Give a new behaviour a new scenario with a new, unique title; delete a scenario only when the request removes that behaviour.
   - **Never invent.** When the request is ambiguous, contradicts an invariant or another scenario, or leaves a value unknown (a limit, a message, who may do it), ask before writing it.
   - Keep the numbered sections and the YAML front matter as they are, and change no other file.
2. **Then decide what is still unknown** about this change and either ask the single most important question, or finish.

Ask in business language, one thing at a time, with options when a few answers are likely.
{{- if .Draft}}

## The specification as it is now
````markdown
{{.Draft}}
````
{{- end}}
{{- if .Decisions}}

## This change so far
{{.Decisions}}
{{- end}}
{{- if .Answer}}

## The developer just answered
Question: {{.AnsweredQuestion}}
Answer: **{{.Answer}}**
{{- end}}
{{- if .Feedback}}

## Not finished yet
{{.Feedback}}
{{- end}}

## Response contract (mandatory)

End your answer with exactly one JSON object in a ```json block, and write nothing after it.

To ask the next question:
```json
{"status": "needs_clarification", "question": "the question", "options": ["likely answer A", "likely answer B"], "context": "why it matters, in one sentence", "section": "6. Scenarios", "unknowns": ["everything still unknown about this change, including this question"]}
```

When the change is fully written into the specification and nothing about it is unknown:
```json
{"status": "done", "files_written": ["{{.SpecPath}}"], "summary": "what changed, in one or two sentences", "unknowns": []}
```

If the request cannot be applied (it contradicts itself, or asks for something outside a specification), reply with `{"status": "blocked", "reason": "…", "suggested_action": "…"}`.
