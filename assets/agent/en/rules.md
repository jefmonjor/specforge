## Working under SpecForge

This repository is built from specifications in `specs/` through a Red → Green → Refactor loop that SpecForge verifies itself: it runs the tests, fingerprints the test files and checks every file you claim to have written.

### Ask, don't invent
If anything you need is not in the specification, its decisions log (`specs/<spec>/decisions.md`), the existing code or the prompt, **do not assume it**. Answer with `status: needs_clarification` and the exact question. A wrong guess costs more than a question.

### Tests are the contract
- Name every test with the scenario marker SpecForge gives you (for example `SDD_0001_003`).
- In RED, the test must compile and fail on an assertion. Add only the stubs it needs to compile.
- In GREEN and REFACTOR, never create, edit, rename or delete a test file.
- Never list a file you did not write.

### Your shell
- A guard reads every command you run, and the scripts, files and package scripts it runs. Write a script in one step and run it in another, so it can be read; never run in the same command a file you download, decode or copy.
- Never delete directories recursively, discard uncommitted work or rewrite git history. If a task seems to need it, ask.

### Craft
- Write the minimum code that makes the current test pass (YAGNI, KISS); refactor only with the tests green.
- Keep the domain free of frameworks and I/O; depend on small interfaces defined where they are used.
- One reason to change per type and per function; no God objects.
- Handle every error explicitly; never swallow one.
- Use the ubiquitous language of the specification for names; no abbreviations nobody asked for.
- No duplication, no dead code, no secrets or real personal data in code, tests or fixtures.
