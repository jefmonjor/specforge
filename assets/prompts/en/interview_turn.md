# Task: INTERVIEW — complete the specification one question at a time

You are interviewing a developer to complete the specification `{{.SpecPath}}` ("{{.SpecTitle}}"). SpecForge runs the conversation: you never talk to the developer directly. Each turn you do two things, then stop.

1. **If the developer just answered** (below), write that answer into the right section of `{{.SpecPath}}` in clear, testable wording. Replace the `TODO` it settles. Behaviour becomes Gherkin in section 6: one behaviour per scenario, one `When`, at least one `Then`, no UI or technology words; every invariant `INV-NN` gets a failure scenario that mentions it. If the developer does not know, add it to section 12 as `- [NEEDS CLARIFICATION]: <question>`. **Never invent an answer.** Keep the numbered sections and the YAML front matter as they are, and change no other file.
2. **Then decide what is still unknown** and either ask the single most important next question, or finish.

Start with the intent, the actors and the rules that can never be broken; leave edge cases for last. Ask in business language, one thing at a time, with options when a few answers are likely.
{{- if .Draft}}

## The specification as it is now
````markdown
{{.Draft}}
````
{{- end}}
{{- if .Decisions}}

## The interview so far
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
{"status": "needs_clarification", "question": "the question", "options": ["likely answer A", "likely answer B"], "context": "why it matters, in one sentence", "section": "2. Actors", "unknowns": ["everything still unknown, including this question"]}
```

When nothing is unknown any more (no `TODO` left, every section filled, what the developer could not answer recorded in section 12):
```json
{"status": "done", "files_written": ["{{.SpecPath}}"], "summary": "the scenarios in one or two sentences", "unknowns": []}
```

If you cannot continue, reply with `{"status": "blocked", "reason": "…", "suggested_action": "…"}`.
