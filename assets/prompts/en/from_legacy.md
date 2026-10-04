# Task: SPECIFICATION FROM LEGACY — write down what the old system really does

A legacy system is being rewritten{{if .JavaRelease}} on Java {{.JavaRelease}}{{end}}. Its code is at `{{.Legacy}}`: **read it, never change it** (SpecForge checks that no file there changed). Fill in the specification `{{.SpecPath}}` for this capability:

> **{{.Capability}}**

The specification records the behaviour **as it is today**, so the new code can be tested against it. It is not a wish list.

## What to do
1. Find the code that implements the capability (the capability map and the inventory below help) and read it, including the SQL, the JSP pages and the configuration it depends on.
2. Replace every `TODO` of `{{.SpecPath}}`, in business words and in its language, keeping the numbered sections and the YAML front matter:
   - each rule the code enforces becomes an invariant `INV-NN` with a failure scenario that mentions it;
   - each behaviour becomes a Gherkin scenario in section 6: one `When`, at least one `Then`, the real values the code uses (limits, rounding, messages), no technology words;
   - section 8 lists the errors with the messages the legacy code really shows.
3. Add a last section, `## 13. Legacy sources`, with one line per rule or scenario and the code it comes from: `- INV-01: \`path/To.java:40-52\``. Paths are relative to `{{.Legacy}}`. **SpecForge opens every citation**: a file that does not exist or a line past its end sends the specification back.
4. Whatever the code does that you cannot explain, or that looks like a bug (a rule applied in one place but not another, a magic number, dead code), goes to section 12 as `- [NEEDS CLARIFICATION]: <question>` with its source. **Never decide it yourself**: the developer answers those before approval, and may decide to keep or fix the behaviour.
5. Leave out what belongs to the platform rather than to the business (servlet plumbing, connection pools, logging): the plan of the rewrite handles it.

Change no file other than `{{.SpecPath}}`.

## Inventory of the legacy code
{{.Inventory}}
{{- if .Capabilities}}

## Capability map
{{.Capabilities}}
{{- end}}
{{- if .Draft}}

## The specification as it is now
````markdown
{{.Draft}}
````
{{- end}}
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
