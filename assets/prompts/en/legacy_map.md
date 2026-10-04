# Task: LEGACY MAP — find the business capabilities of a legacy system

A legacy system is being rewritten. Its code is at `{{.Legacy}}`: **read it, never change it** (SpecForge checks that no file there changed). Its inventory, measured by SpecForge, is below.

## Inventory
{{.Inventory}}

## What to do
Write `{{.DocPath}}` and change no other file. It lists what the system does for the business, one capability per section, so each one can become a specification:

```markdown
# Capability map

## <capability name, in business words>
- **What it does**: one or two sentences, in the business's language.
- **Entry points**: `relative/path/Servlet.java:12-40`, `relative/path/page.jsp`
- **Rules found in the code**: each rule with the source that implements it, `path:line`.
- **Data**: the tables, files or messages it reads and writes.
- **Depends on**: the other capabilities it needs.
- **Unclear**: what the code does that you cannot explain (dead branches, magic numbers, commented-out code). Never guess a reason.
```

Rules:
- Every source is cited in backticks as `path:line` or `path:from-to`, relative to `{{.Legacy}}`. SpecForge opens every citation; one that does not exist, or a line past the end of its file, sends the map back.
- Describe behaviour, not technology: "an employee's net pay is gross minus tax" rather than "PayrollServlet calls EmployeeDao".
- Order the capabilities so that the ones the others depend on come first: that is the order to migrate them.
- Do not propose the new design here; the plan of each specification does that.
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
