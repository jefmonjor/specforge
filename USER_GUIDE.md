# SpecForge user guide

SpecForge drives a coding agent ([Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Gemini CLI](https://github.com/google-gemini/gemini-cli)) through a test-first loop against a specification you approved, and checks every step itself instead of trusting the agent's word. This guide covers every command, every file SpecForge writes and every exit code.

- [1. How it works](#1-how-it-works)
- [2. Install](#2-install)
- [3. Configure once: `init`](#3-configure-once-init)
- [4. Prepare a repository: `setup`](#4-prepare-a-repository-setup)
- [5. Specifications: `spec`](#5-specifications-spec)
- [6. The plan: `plan`](#6-the-plan-plan)
- [7. The loop: `loop`](#7-the-loop-loop)
- [8. Questions instead of guesses](#8-questions-instead-of-guesses)
- [9. Quality gates](#9-quality-gates)
- [10. Security audit: `audit`](#10-security-audit-audit)
- [11. Browser verification: `e2e`](#11-browser-verification-e2e)
- [12. Hand-over: `deliver`](#12-hand-over-deliver)
- [13. Legacy rewrites: `legacy`, `spec from-legacy`](#13-legacy-rewrites-legacy-spec-from-legacy)
- [14. Configuration reference](#14-configuration-reference)
- [15. Files SpecForge writes](#15-files-specforge-writes)
- [16. Exit codes](#16-exit-codes)
- [17. Troubleshooting](#17-troubleshooting)
- [18. Architecture](#18-architecture)

## 1. How it works

```text
spec new ─► interview / edit ─► clarify ─► approve (R0) ─► plan ─► plan approve (R1) ─► loop ─► audit ─► e2e ─► deliver (R4)
                                                                                       │
                                  per scenario:  RED ─► GREEN ─► REFACTOR ─► review (R2) ─► commit
```

- A **specification** (`specs/NNNN-slug.md`) says *what* to build and *why*, with the acceptance criteria as Gherkin scenarios.
- **Approval** is a human gate: it refuses while a `TODO` or an open question is left, records who approved and when, and seals the content with a SHA-256 hash. The loop only runs an approved, unchanged specification.
- The **plan** says where the code goes, with one planned test per scenario. You review and approve it before any code exists; the loop follows it.
- The **loop** takes one scenario at a time. The agent writes code; SpecForge runs the tests, compares the files on disk before and after each turn, fingerprints the test files after RED and runs the quality gates. Whatever the agent claims, SpecForge checks.
- When the agent lacks information it **asks** (`needs_clarification`) instead of guessing. You answer once; the answer is recorded and reused.

## 2. Install

Download the binary for your platform from the [latest release](https://github.com/jefmonjor/specforge/releases/latest) (`specforge-<os>-<arch>`, plus `checksums.txt`), make it executable and put it on your `PATH`:

```bash
curl -L -o specforge https://github.com/jefmonjor/specforge/releases/latest/download/specforge-darwin-arm64
chmod +x specforge && sudo mv specforge /usr/local/bin/
```

On Windows, download `specforge-windows-amd64.exe`, rename it to `specforge.exe` and place it in a folder on your `PATH`. SpecForge never edits your `PATH` or registry.

From source (Go version from [`go.mod`](go.mod)):

```bash
git clone https://github.com/jefmonjor/specforge.git && cd specforge
make test build        # or: go build -o specforge .
```

You also need the agent CLI you choose, installed and signed in. SpecForge drives the agent you already use and never asks for an API key.

## 3. Configure once: `init`

```bash
specforge init                              # asks
specforge init --agent claude --language es # no questions
```

`init` saves your agent (`claude` or `gemini`), the language of prompts, templates and messages (`en` or `es`) and, optionally, `--model`. Nothing else. The file lives in your user configuration directory (`~/.config/specforge/config.yaml` on Linux, `~/Library/Application Support/specforge/` on macOS, `%AppData%\specforge\` on Windows), with owner-only permissions. Set `SPECFORGE_HOME` to keep it elsewhere.

## 4. Prepare a repository: `setup`

```bash
cd my-project
specforge setup                       # your configured agent, detected stack
specforge setup --agents claude,gemini --stack node
```

`setup` writes three things and never touches your code:

| File | What | On a second run |
| :--- | :--- | :--- |
| `specforge.yaml` | Project settings shared by the team, every default commented. | Kept as is. |
| `CLAUDE.md` / `GEMINI.md` | A block between `<!-- specforge:begin … -->` and `<!-- specforge:end -->` with the working rules (ask, don't invent; tests are the contract; craft) and the coding standard of your stack. Both agents load these files at every session. | The block is refreshed; the rest of the file is yours and stays untouched. |
| `.gitignore` | Adds `.specforge/`, the local loop state. | Unchanged. |

The stack is detected from `go.mod`, `pom.xml`, `build.gradle(.kts)`, `package.json` (Vitest, Jest or `npm test`) or `pyproject.toml`/`requirements.txt`/`setup.py`. With several, SpecForge asks which one to drive, or fails in CI until you set `stack:`.

### Starting a new project: `--new`

```bash
mkdir shop && cd shop && git init
specforge setup --new react                  # or java, python, go; --name sets the project name
specforge setup --new java --legacy ../old   # a Java 21 rewrite of ../old (see section 13)
```

`--new` writes a project skeleton with the tooling the loop drives, then does the usual setup. It refuses a directory that already holds a build file, and keeps any other file that exists. Every scaffold was installed and run end to end (tests, lint, and the other gates) with the versions it pins:

| Stack | Files | Tooling |
| :--- | :--- | :--- |
| `java` | `pom.xml`, `domain`/`application`/`adapters` packages, `ArchitectureTest` | Java 21 (`maven.compiler.release`), JUnit 6, AssertJ, ArchUnit (the domain imports no framework, the application does not know the adapters, no `javax.*` Java EE, Log4j 1, JUnit 3, `Vector` or `Hashtable`), PMD quickstart rules |
| `react` | Vite app, `App.test.tsx`, `src/test/setup.ts` | Vite 7, React 19, TypeScript strict, Vitest 4.0 with jsdom and Testing Library, ESLint with typescript-eslint and react-hooks, Knip, jscpd, Stryker |
| `python` | `pyproject.toml`, `src/<module>/`, `tests/` | pytest, Ruff with a broad rule set (tests may carry the marker in their names) |
| `go` | `go.mod`, `main.go`, `.golangci.yml` | golangci-lint v2 |

Then install the tools: `mvn test`, `npm install`, or `python3 -m venv .venv && .venv/bin/pip install -e '.[dev]'`. Python projects use the `.venv`'s pytest and Ruff; Java and Gradle projects use `mvnw`/`gradlew` when present.

## 5. Specifications: `spec`

```bash
specforge spec new "Password reset"   # specs/0001-password-reset.md from the template
specforge spec interview 0001         # complete it in a conversation with your agent
specforge spec clarify 0001           # answer the open questions, one at a time
specforge spec lint 0001              # what blocks approval, and advice
specforge spec approve 0001           # review gate R0: lint, approver, seal
specforge spec list                   # number, state, title
```

A specification is named by its number (`1`, `0001`), a file-name prefix (`0001-pass`) or its path. With one specification the argument is optional; with several, SpecForge asks (or fails in CI) instead of picking one.

### The template

`spec new` numbers the file after the highest existing one and fills a YAML front matter (`id`, `title`, `status`, `created`, `approved_by`, `approved_at`). The body has twelve sections: intent, actors, ubiquitous language, invariants (`INV-NN`), user stories, scenarios (Gherkin), data contracts, errors, non-functional requirements, out of scope, assumptions and open questions. Every placeholder is a `TODO`.

Scenarios go in a fenced ```` ```gherkin ```` block and are parsed by the official [Cucumber Gherkin parser](https://github.com/cucumber/gherkin): every Gherkin language (`# language: es`), `And`/`But`, `Background`, `Rule` and `Scenario Outline` with `Examples` (one scenario per row).

### Lint

| Blocks approval | Advice only |
| :--- | :--- |
| A `TODO` outside an HTML comment | A numbered template section is missing |
| An open question: `- [NEEDS CLARIFICATION]: …` | An invariant `INV-NN` that nothing else mentions |
| No Gherkin scenario, or Gherkin that does not parse | A scenario with more than one `When` |
| A scenario without `When` or without `Then` | |

### Interview

`spec interview` completes the specification in a conversation SpecForge runs, one question at a time. Each turn is one call to your agent, which:

1. writes your last answer into the right section, in testable wording (behaviour becomes Gherkin scenarios; every invariant gets a failure scenario);
2. returns the single most important next question, why it matters, the section it completes and the list of what is still unknown, or reports that nothing is.

SpecForge shows the progress (`section 4. Invariants · 6 unknown(s) left`), asks you, and records each question and answer in `specs/NNNN-slug/interview.jsonl` and in the decisions log. The agent may only change the specification: any other file is an error. The interview ends only when the lint finds no `TODO` and no missing structure; if the agent says it is done too early, it is sent back with the issues. What you cannot answer is written as an open question for `spec clarify`, never guessed. The interview never approves or seals.

Without a terminal the question goes to `questions.md` (exit 5): write the answer there and run `spec interview` again; it continues where it stopped. `--chat` instead hands your terminal to the agent for a free conversation.

### Clarify

`spec clarify` asks every `[NEEDS CLARIFICATION]` and writes each answer in place of its question, as `- **Decided:** question → answer (date, name)`, so the specification itself says what was decided. Answers also go to `specs/NNNN-slug/decisions.md`. Without a terminal the questions go to `questions.md` (exit 5); answer them there and run `clarify` again.

### Approve

`spec approve` lints, then records the approver (`--by`, else `git config user.name`, else a question) and the UTC time in the front matter, and appends the seal:

```markdown
<!-- seal: sha256-v1:9394d6452b3a… -->
```

The hash ignores line endings and trailing spaces, so a Windows checkout verifies the same. Specifications sealed by SpecForge 3 (`sha256:`) still verify.

**Changing an approved specification** is an edit plus a new approval. The edit makes it `changed` and the loop refuses it until you revert the edit or approve the new version on purpose. Every approval is appended to `specs/NNNN-slug/approvals.md` with the date, the approver, the seal and each scenario marked `ADDED`, `MODIFIED`, `UNCHANGED` or `REMOVED` compared with the previous approval. `approve` prints the changes, and the loop redoes only the scenarios whose text changed.

## 6. The plan: `plan`

```bash
specforge plan 0001            # the agent drafts specs/0001-password-reset/plan.md
specforge plan approve 0001    # review gate R1: lint, approver, seal
```

`plan` asks your agent where the code goes before any code exists: the approach, one line per file to create or change, a table with **one planned test per scenario** (by marker), the interfaces the domain needs and the risks. It shows the agent the approved specification and the list of project files. The agent may only write `plan.md`: SpecForge rejects a draft that changes any other file, and sends it back (up to `max_attempts`) while a scenario has no planned test. When an architectural choice is not settled, the agent asks.

Read the plan and edit it as you like, then approve it: approval checks that every scenario marker is there and no `TODO` is left, records you and seals the file. Running `plan` again revises the current draft instead of starting over.

The plan is optional. When `plan.md` exists, the loop requires it approved, unchanged and placing every scenario (an amended specification with a new scenario needs the plan updated and approved again), and every prompt carries it.

## 7. The loop: `loop`

```bash
specforge loop 0001                          # start
specforge loop --resume                      # continue after a stop, a question or Ctrl-C
specforge loop 0001 --restart                # discard the saved state and start over
specforge loop 0001 --scenario 3 --from green  # redo one scenario from a phase
specforge loop 0001 --review off --no-commit   # no per-scenario review, no commits
```

For each scenario, in order:

| Phase | The agent | SpecForge accepts it only when |
| :--- | :--- | :--- |
| **RED** | Writes the test for this scenario, named with its marker (`SDD_0001_003`), plus the stubs it needs to compile. | A test file carrying the marker changed; the files the agent lists really changed; the filtered test run compiles, runs at least one test and fails. A test that passes before any implementation is reported to you: either the behaviour exists already (mark the scenario satisfied) or the test is wrong. |
| **GREEN** | Writes the minimum code. | The test files are byte-for-byte as RED left them (else the loop stops: *test tampering*); the marker's tests pass. Failures are fed back, up to `max_attempts` (default 3). |
| **REFACTOR** | Only called when something blocks: fixes the full suite or the gate findings without touching tests. | The whole suite passes and no gate blocks. |
| **REVIEW** (R2) | — | You review the finished scenario: the files it changed and the gates. **Accept**, type **what should change** (back to GREEN with your note; the tests stay) or send it **back to RED** (the test does not express the scenario). Without a terminal the review is a question in `questions.md` (exit 5). `review: off` skips it. |
| **COMMIT** | — | The scenario's files, with the decisions log, are recorded as one commit, `feat(SDD_0001_003): <title>`, and nothing else you have staged is touched. Hooks and signing run as usual. Off with `commit: false` or `--no-commit`; skipped outside git. |

When a test with the scenario's marker already exists (an earlier run stopped before RED was accepted, or you wrote it yourself), RED runs it before calling the agent: a test that compiles and fails is accepted as it is, a passing test goes to you as above, and one that does not compile goes to the agent with the output. A scenario you mark as already satisfied is committed as `test(SDD_…)` with its test.

`--scenario N --from red|green|refactor` reopens one scenario and keeps the others as they are. Starting after RED takes the current test files as the reference for the tampering check.

When a phase needed more than one attempt, the agent may add a one-sentence `lesson` to its answer: the rule that would have avoided the mistake. SpecForge keeps it in `specs/LESSONS.md`, tagged with the stack, without duplicates and at most 30, and every later prompt for that stack shows them. The file is yours to edit.

The loop state is saved after every step in `.specforge/state/<spec>.json` (written atomically). `--resume` re-reads the specification and checks the seal first. If the specification was approved again with changes, the scenarios whose text did not change keep their progress and the rest are redone. A finished loop is reported, not redone; `--restart` runs it again on purpose.

Test commands used for the marker filter: `go test -json -run`, Maven `-Dtest`, Gradle `--tests`, `vitest run -t`, `jest -t`, `pytest -k`. Results are read from the runner's machine-readable report (test2json, Surefire/JUnit XML, Vitest/Jest JSON, pytest JUnit XML), so "did not compile", "nothing ran" and "an assertion failed" are told apart. With plain `npm test` the filter is not exact and SpecForge asks you to confirm the RED.

## 8. Questions instead of guesses

Every prompt ends with a response contract. The agent answers `done` (with the files it wrote), `needs_clarification` (a question, optional choices, context) or `blocked` (a reason and a suggested action). Prose without the contract gets one retry, then the step stops: prose never counts as success.

- **At a terminal**, the question is shown with numbered options; your answer is appended to `specs/NNNN-slug/decisions.md` (date, phase, scenario) and sent back to the agent. The decisions log is part of every later prompt, so nothing is asked twice.
- **Without a terminal** (CI, `--non-interactive`), the question is written to `specs/NNNN-slug/questions.md` and the command exits with code **5**. Answer it in either of two ways:
  - write the answer (or an option number) in place of `_awaiting an answer_` in `questions.md`, then run `specforge loop --resume` anywhere, CI included;
  - or run `specforge loop --resume` at a terminal and answer there.

  The resumed step starts with the answer and does not start over: the files the agent had already written before asking still count as written in that turn. While the question has no answer, `--resume` exits with code 5 again without calling the agent.
- **Blocked** stops with exit code 2 and the agent's suggested action.

The managed block in `CLAUDE.md`/`GEMINI.md` carries the same rule: if anything needed is not in the specification, the decisions log, the code or the prompt, do not assume it.

## 9. Quality gates

REFACTOR runs the gates that apply to the stack. Each reads its tool's machine-readable report and compares a number with a threshold. A tool that is missing or crashes is **skipped**, shown with ⚠, never as a pass; with `quality.strict: true` (or `--strict`) a skipped gate blocks. Node tools run with `npx --no-install`: nothing is downloaded during the loop.

| Gate | Go | Node | Java | Python |
| :--- | :--- | :--- | :--- | :--- |
| Lint | `golangci-lint` (falls back to `go vet` when absent) | `npm run lint` | PMD, when `maven-pmd-plugin` is in the build (Maven) | `ruff check` (the `.venv`'s first) |
| Duplication (`max_duplication_percent`, default 0) | jscpd | jscpd | jscpd | jscpd |
| Dead code | — | Knip | — | — |
| Mutation (`min_mutation_score`, default 80) | — | Stryker, when configured | — | — |
| Migration | — | — | when `migration.java_release` or `forbidden_imports` is set: the declared release and every `import` in the Java sources | — |

The migration gate needs no tool, so it never skips. Its findings name the file and line: `src/main/java/…/Payroll.java:3: imports javax.servlet.http.HttpServlet (forbidden: javax.servlet)`.

## 10. Security audit: `audit`

```bash
specforge audit                    # your changes since origin/main, origin/master, main or master
specforge audit --base v1.2.0
specforge audit --full --fail-on medium
```

Three passes run with your agent over each chunk of code: **reconnaissance**, a red-team **hunter** that proposes findings with file, line and evidence, and a blue-team **validator** that confirms or rejects each one. Every answer must match the embedded JSON schema; an invalid answer gets one retry and then the audit stops with an error, never with an empty passing report (*fail-closed*). Findings the validator cannot settle at or above the threshold are questions for you. A confirmed finding at or above `--fail-on` (default `high`) exits with code 2.

The base ref is validated as a commit and passed after `--end-of-options`, so it can never be read as a git option. Reports go to `docs/security/` (`REPORT.md`, `findings.json`, `coverage-ledger.json`) with owner-only permissions.

## 11. Browser verification: `e2e`

```bash
specforge e2e 0001 --url http://localhost:3000
specforge e2e 0001 --url http://localhost:3000 --scenario 2 --headed
```

`e2e` drives Chrome, Chromium or Edge (found on your system, or `CHROME_PATH`) through [chromedp](https://github.com/chromedp/chromedp), scenario by scenario. The agent sees a compact view of the page (URL, title, interactive elements with unique selectors, visible text) and answers with one typed action at a time: `navigate`, `click`, `type`, `select`, `press`, `scroll`, `wait`, `assert` or `fail`. Before running an action SpecForge checks that the selector exists on the page and that navigation stays on the same origin. An `assert` names a `Then` and the evidence (text, selector or URL); SpecForge verifies the evidence itself. The page is untrusted input: its text never becomes an instruction.

Each step saves a screenshot. Results go to `docs/e2e/<spec>/` (`report.json`, `REPORT.md`, `scenario-NN/step-MM.png`). The command exits with code 2 when the pass rate is below `--min-pass-rate` (default 100). `--insecure` accepts invalid TLS certificates for local test servers; it is off by default. Only approved specifications run.

## 12. Hand-over: `deliver`

```bash
specforge deliver 0001
gh pr create --body-file specs/0001-password-reset/PR_BODY.md
```

`deliver` writes three files next to the specification, built only from what SpecForge recorded, never from the agent's word:

| File | For | Content |
| :--- | :--- | :--- |
| `DELIVERY.md` | people | Who approved the specification and the plan, and when. One row per scenario: its tests (file and test names carrying the marker), its commit, its gates (✓ passed · ⚠ skipped · ✗ failed) and notes (already satisfied, changes requested in review, rejected attempts). The decisions taken, the questions still open, the lessons, the audit and E2E results, and what is out of scope. |
| `trace.json` | tools | The same data, machine-readable. |
| `PR_BODY.md` | the pull request | A summary, the scenario table, the decisions, the checks and what is not done. When the repository has a pull request template (`.github/pull_request_template.md` and the other places GitHub looks), its headings are kept and the summary goes under the first one. |

A delivery is honest about gaps: unfinished scenarios, open questions and checks that did not run are listed, and an incomplete delivery says so in its first line. Reviews and SpecForge's own verification questions are shown per scenario, not mixed with your product decisions. The loop state lives in `.specforge/`, so run `deliver` where the loop ran; `trace.json` keeps the trace once committed.

## 13. Legacy rewrites: `legacy`, `spec from-legacy`

Rewriting a legacy system (a Java 6 servlet application on Java 21, for example) happens in a **new project next to the old one**. The legacy repository is evidence, not a workspace: the agent may read it, and SpecForge hashes it before and after every agent turn (map, specification, plan and every loop phase) and refuses any change.

```yaml
# specforge.yaml (written by `setup --legacy`, or by hand)
migration:
  legacy: ../legacy-payroll   # relative to the project, or absolute
  java_release: 21            # the build must declare it
  # forbidden_imports: [javax.servlet, org.apache.log4j]   # default: see below
```

With `java_release` set and no list of its own, the forbidden imports are the Java EE packages Jakarta renamed (`javax.servlet`, `javax.persistence`, `javax.validation`, `javax.ejb`, `javax.jms`, `javax.ws.rs`, `javax.xml.bind`, `javax.xml.rpc`, `javax.annotation`, `javax.inject`, `javax.faces`, `javax.transaction`), Log4j 1, JUnit 3, `Vector` and `Hashtable`.

| Command | What it does | What SpecForge verifies |
| :--- | :--- | :--- |
| `legacy scan [path]` | Writes `docs/legacy/INVENTORY.md` without an agent: build tool, declared Java release (the lowest of `pom.xml`, Gradle and Ant), size, frameworks found from imports and files, the largest packages, and what each finding means for Java 21. | It is measured, not generated. |
| `legacy map [path]` | The agent reads the legacy code and writes `docs/legacy/CAPABILITIES.md`: one section per business capability with what it does, entry points, rules with their sources, data, dependencies and what is unclear, ordered so that what others depend on comes first. | Only that file may change; at least one capability; every `` `path:line` `` or `` `path:from-to` `` citation is opened in the legacy code. A file that does not exist or a line past its end sends the map back with the list. |
| `spec from-legacy "<capability>"` | Creates the next specification and the agent fills it with the behaviour **as it is today**: rules as invariants, scenarios with the real values and messages, and a `## 13. Legacy sources` section citing the code of each rule and scenario. | Only the specification may change; the template lint (open questions aside); a sources section with citations; every citation in it, and every line citation elsewhere, resolves. One capability per specification: the others go to *Out of scope*. Run it again with the same capability to continue the draft. |

What the code does that nobody can explain (a rule applied in one place and not another, floating-point money, dead code) is written as `[NEEDS CLARIFICATION]` with its source. Answer them with `spec clarify` (or edit them), then approve: the specification is now the contract the new code is tested against. During `plan` and `loop` the agent receives the legacy path, the sources section and the target release, reads the cited code to reproduce the behaviour, and the migration gate checks the result.

## 14. Configuration reference

Values resolve in this order: command-line flag, then `specforge.yaml`, then your user configuration, then the default.

```yaml
# specforge.yaml
language: en            # es | en
stack: go               # go | maven | gradle | node | python (detected when unset)
agent: claude           # usually each developer's choice
model: ""               # passed to the agent; empty uses its default
max_attempts: 3         # attempts per phase
review: scenario        # scenario: review every finished scenario (R2) | off
commit: true            # one commit per finished scenario
timeouts:
  agent: 20m
  tests: 10m
quality:
  strict: false
  max_duplication_percent: 0
  min_mutation_score: 80
migration:              # only for a rewrite (section 13)
  legacy: ../old-system
  java_release: 21
  forbidden_imports: [javax.servlet, org.apache.log4j]
```

Unknown keys are an error, so a typo never silently leaves a default in place.

Global flags: `--verbose` (progress details and the agent's live output), `--debug` (debug records in the log), `--trace-io` (every prompt and answer in the log), `--quiet` (only warnings, errors and data), `--non-interactive` (never ask: write questions to a file and exit 5), `--json` (data from `version`, `spec list` and `deliver` as JSON on stdout, and errors as `{"exit", "title", "cause", "action"}` on stderr).

## 15. Files SpecForge writes

| Path | Committed | Content |
| :--- | :---: | :--- |
| `specforge.yaml` | yes | Project settings. |
| `CLAUDE.md`, `GEMINI.md` | yes | Your content plus the managed block. |
| `specs/NNNN-slug.md` | yes | The specification, its front matter and its seal. |
| `specs/NNNN-slug/plan.md` | yes | The technical plan, its front matter and its seal. |
| `specs/NNNN-slug/approvals.md` | yes | Every approval with its scenario changes. |
| `specs/NNNN-slug/interview.jsonl` | yes | The interview transcript: each question with its section and unknowns, and your answer. |
| `specs/NNNN-slug/decisions.md` | yes | Every question the agent asked and your answer. |
| `specs/LESSONS.md` | yes | Lessons the agent wrote after a rejected attempt. |
| `specs/NNNN-slug/DELIVERY.md`, `trace.json`, `PR_BODY.md` | yes | The hand-over written by `deliver`. |
| `specs/NNNN-slug/questions.md` | yes | Questions asked when nobody was at the terminal. |
| `docs/legacy/INVENTORY.md`, `CAPABILITIES.md` | yes | The legacy inventory and capability map. |
| `docs/legacy/decisions.md`, `questions.md` | yes | Questions asked while mapping the legacy code, and your answers. |
| `docs/security/` | your choice | Audit reports (owner-only permissions). |
| `docs/e2e/<spec>/` | your choice | E2E reports and screenshots. |
| `.specforge/state/<spec>.json` | no | Loop state for `--resume`. |

The log file lives in your user cache directory (`~/.cache/specforge/logs/specforge.log` on Linux), rotated, with owner-only permissions.

## 16. Exit codes

| Code | Meaning |
| :---: | :--- |
| 0 | Done. |
| 1 | Unexpected error or bad usage. |
| 2 | A gate said no: tests, quality, security findings, E2E pass rate, or the agent is blocked. |
| 3 | The specification or the loop state needs attention: not approved, changed after approval, lint issues, ambiguous or missing, test tampering, a loop in progress. |
| 4 | A tool, browser or setting is missing: agent CLI, test runner, configuration. |
| 5 | A question awaits your answer (no terminal). |
| 130 | Interrupted with Ctrl-C; the state saved so far is valid. |

Every failure prints what happened, why and what to do next.

## 17. Troubleshooting

- **What did the agent receive and answer?** Re-run with `--trace-io` and read the log file.
- **The agent claims files it did not write.** The attempt is rejected and the agent is told which ones; that is the loop working. Repeated rejections exhaust `max_attempts` (exit 2).
- **"Several stacks detected".** Set `stack:` in `specforge.yaml` or pass `--stack`.
- **The loop says the specification changed.** Revert the edit, or review it and run `specforge spec approve` again; unchanged scenarios keep their progress.
- **A gate shows ⚠ skipped.** Install the tool, or accept the warning; `--strict` makes it block.
- **"The agent changed the legacy code".** Restore the legacy repository (`git checkout .` there) and run the command again; the agent may only read it.
- **A legacy document keeps coming back.** The diagnosis lists each citation that did not resolve; the agent gets the same list. Check the paths are relative to the legacy repository.

## 18. Architecture

SpecForge follows the architecture it asks of your code: the domain is pure, use cases depend on small ports, adapters do the I/O and `cmd/` only wires them.

```text
cmd/                      CLI and composition root (cobra); signals → context; exit codes
internal/
  domain/                 pure: no I/O
    spec/                 Gherkin parsing (official parser), seal, lint, front matter, open questions
    lessons/              the curated lessons list
    delivery/             the delivery trace and its rendering
    tdd/                  loop state, test outcomes, typed errors
    stack/                stack detection, test commands, test-file rules
    legacy/               legacy inventory, citations and their verification, migration conformance
    quality/ security/ e2e/
  app/                    use cases
    tddloop/              Red → Green → Refactor with verification, review and commit
    specs/ planning/      specification lifecycle and approvals; drafting the plan
    interview/            the turn-based interview
    docturn/              one agent turn whose product is a document, verified (plan, map, legacy spec)
    migrate/              legacy scan, capability map, specification from legacy
    scaffold/             setup --new
    deliver/              the hand-over: DELIVERY.md, trace.json, PR_BODY.md
    conversation/         one agent turn under the response contract
    setup/                repository setup
    audit/ e2erun/        security audit, browser verification
    clarify/ protocol/    questions to the developer, the agent response contract
    prompts/ layout/      prompt templates, project paths
  ports/                  interfaces the use cases depend on
  adapters/               agent CLI, process runner, test runners, gates, git, browser, files, logging
  config/ ui/             settings resolution; terminal output and diagnoses
assets/                   embedded prompts, rules, standards, audit method, templates, scaffolds (es/en)
```

Tests run the real CLI against scripted agents and real `go test`, `git` and Chromium where available: `make test`.
