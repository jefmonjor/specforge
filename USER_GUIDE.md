# SpecForge user guide

SpecForge drives a coding agent ([Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Gemini CLI](https://github.com/google-gemini/gemini-cli)) through a test-first loop against a specification you approved, and checks every step itself instead of trusting the agent's word.

This guide covers every command, every file SpecForge writes, what you edit by hand, and every exit code. New here? Read sections 1 to 7 once, then keep [Your files](#14-your-files-what-to-edit) and [Recipes](#15-recipes) at hand.

**Start**
- [1. How it works](#1-how-it-works)
- [2. Install](#2-install)
- [3. Configure once: `init`](#3-configure-once-init)
- [4. Prepare a repository: `setup`](#4-prepare-a-repository-setup)

**Daily work**
- [5. Specifications: `spec`](#5-specifications-spec)
- [6. The plan: `plan`](#6-the-plan-plan)
- [7. The loop: `loop`](#7-the-loop-loop)
- [8. Questions instead of guesses](#8-questions-instead-of-guesses)
- [9. Quality gates](#9-quality-gates)
- [10. Security audit: `audit`](#10-security-audit-audit)
- [11. Browser verification: `e2e`](#11-browser-verification-e2e)
- [12. Hand-over: `deliver`](#12-hand-over-deliver)
- [13. Legacy rewrites: `legacy`, `spec from-legacy`](#13-legacy-rewrites-legacy-spec-from-legacy)

**Reference**
- [14. Your files: what to edit](#14-your-files-what-to-edit)
- [15. Recipes](#15-recipes)
- [16. Configuration reference](#16-configuration-reference)
- [17. Exit codes](#17-exit-codes)
- [18. Troubleshooting](#18-troubleshooting)
- [19. Architecture](#19-architecture)

---

## 1. How it works

```text
spec new
 └ interview, or edit by hand
   └ clarify the open questions
     └ spec approve       (gate R0)
       └ plan, plan approve    (R1)
         └ loop, per scenario:
             RED → GREEN → REFACTOR
             → your review     (R2)
             → one commit
           └ audit, e2e
             └ deliver         (R4)
```

- A **specification** (`specs/0001-slug.md`) says *what* to build and *why*, with the acceptance criteria as Gherkin scenarios.
- **Approval** is a human gate. It refuses while a `TODO` or an open question is left, records who approved and when, and seals the content with a SHA-256 hash. The loop only runs an approved, unchanged specification.
- The **plan** says where the code goes, with one planned test per scenario. You review and approve it before any code exists; the loop follows it.
- The **loop** takes one scenario at a time. The agent writes code; SpecForge runs the tests, compares the files on disk before and after each turn, fingerprints the test files after RED and runs the quality gates.
- When the agent lacks information it **asks** instead of guessing. You answer once; the answer is recorded and reused.

## 2. Install

Download your binary from the [latest release](https://github.com/jefmonjor/specforge/releases/latest). Each release has `specforge-<os>-<arch>` files and `checksums.txt`.

```bash
R=https://github.com/jefmonjor/specforge
R=$R/releases/latest/download
# or darwin-amd64, linux-amd64,
# linux-arm64
F=specforge-darwin-arm64
curl -Lo specforge $R/$F
chmod +x specforge
sudo mv specforge /usr/local/bin/
specforge version
```

**Windows:** download `specforge-windows-amd64.exe`, rename it to `specforge.exe` and put it in a folder on your `PATH`. SpecForge never edits your `PATH` or registry.

**From source** (the Go version in [`go.mod`](go.mod)):

```bash
git clone \
  https://github.com/jefmonjor/specforge
cd specforge
make build      # ./specforge
```

You also need:

- the agent CLI you choose (`claude` or `gemini`), installed and signed in. SpecForge drives the agent you already use and never asks for an API key;
- `git`;
- your stack's test runner (`go`, `mvn`/`gradle`, `npm`, `pytest`).

The quality gates use more tools when they are installed: see [section 9](#9-quality-gates).

## 3. Configure once: `init`

```bash
specforge init
specforge init --agent claude \
  --language es
```

The first form asks; the second asks nothing. `init` saves your agent (`claude` or `gemini`), the language of prompts, templates and messages (`en` or `es`) and, optionally, `--model`. Nothing else.

The file lives in your user configuration directory, with owner-only permissions:

- Linux: `~/.config/specforge/config.yaml`
- macOS: `~/Library/Application Support/specforge/`
- Windows: `%AppData%\specforge\`

Set `SPECFORGE_HOME` to keep it somewhere else.

## 4. Prepare a repository: `setup`

```bash
cd my-project
specforge setup
```

Options: `--agents claude,gemini` writes the rules for both agents; `--stack node` picks the stack when several are detected.

`setup` writes three things and never touches your code:

- **`specforge.yaml`**: project settings shared by the team, every default commented. Kept as it is on a second run.
- **`CLAUDE.md` / `GEMINI.md`**: a block between `<!-- specforge:begin … -->` and `<!-- specforge:end -->` with the working rules (ask, don't invent; tests are the contract; craft) and your stack's coding standard. Both agents read these files at every session. A second run refreshes the block; the rest of the file is yours.
- **`.gitignore`**: adds `.specforge/`, the local loop state.

The stack is detected from `go.mod`, `pom.xml`, `build.gradle(.kts)`, `package.json` (Vitest, Jest or `npm test`) or `pyproject.toml`/`requirements.txt`/`setup.py`. With several, SpecForge asks which one to drive, or fails in CI until you set `stack:`.

### Starting a new project: `--new`

```bash
mkdir shop && cd shop && git init
specforge setup --new react
```

`--new` takes `java`, `react`, `python` or `go`, and `--name` sets the project name (default: the directory name). It writes a skeleton with the tooling the loop drives, then does the usual setup. It refuses a directory that already holds a build file, and keeps any other file that exists.

Every scaffold was installed and run end to end with the versions it pins:

- **`java`**: Java 21 Maven (`maven.compiler.release`), JUnit 6, AssertJ, PMD, and an `ArchitectureTest` with ArchUnit rules: the domain imports no framework, the application does not know the adapters, and no `javax.*` Java EE, Log4j 1, JUnit 3, `Vector` or `Hashtable`. Packages `domain`, `application` and `adapters` are ready.
- **`react`**: Vite 7, React 19, strict TypeScript, Vitest 4.0 with jsdom and Testing Library, ESLint with typescript-eslint and react-hooks, Knip, jscpd and Stryker. A first test of `App` passes.
- **`python`**: `pyproject.toml` with a `src/` layout, pytest and Ruff with a broad rule set. Tests may carry the scenario marker in their names.
- **`go`**: `go.mod`, `main.go` and a golangci-lint v2 configuration.

Then install the tools:

```bash
mvn test                       # java
npm install && npm test        # react
python3 -m venv .venv          # python
.venv/bin/pip install -e '.[dev]'
```

Python projects use the `.venv`'s pytest and Ruff; Maven and Gradle projects use `mvnw`/`gradlew` when present.

## 5. Specifications: `spec`

```bash
specforge spec new "Password reset"
specforge spec interview 0001
specforge spec clarify 0001
specforge spec lint 0001
specforge spec approve 0001
specforge spec list
```

- **`new`** creates `specs/0001-password-reset.md` from the template.
- **`interview`** completes it in a conversation with your agent.
- **`clarify`** asks the open questions, one at a time.
- **`lint`** shows what blocks approval, and advice.
- **`approve`** is review gate R0: lint, approver, seal.
- **`list`** shows number, state and title.

A specification is named by its number (`1`, `0001`), a file-name prefix (`0001-pass`) or its path. With one specification the argument is optional; with several, SpecForge asks (or fails in CI) instead of picking one.

### The template

`spec new` numbers the file after the highest one and fills a YAML front matter (`id`, `title`, `status`, `created`, `approved_by`, `approved_at`). The body has twelve numbered sections, each with a `TODO` to replace:

1. **Intent**: the problem and the value, in two or three sentences.
2. **Actors**: who acts, and what each one can and cannot do.
3. **Ubiquitous language**: every term the scenarios use, defined.
4. **Invariants**: rules that are never broken, as `INV-01`, `INV-02`…
5. **User stories**: *As a…, I want… so that…*
6. **Scenarios**: the acceptance criteria, in Gherkin.
7. **Data contracts**: fields, types and rules.
8. **Errors**: codes, when they happen, messages.
9. **Non-functional requirements**: numbers, or "none".
10. **Out of scope**: what this specification does not cover.
11. **Assumptions**: what is taken for granted, and who confirmed it.
12. **Open questions**: `- [NEEDS CLARIFICATION]: …`, empty before approval.

A specification drafted from legacy code adds `13. Legacy sources` ([section 13](#13-legacy-rewrites-legacy-spec-from-legacy)).

### Writing good scenarios

Scenarios go in a fenced ```` ```gherkin ```` block and are parsed by the official [Cucumber Gherkin parser](https://github.com/cucumber/gherkin): every Gherkin language (`# language: es`), `And`/`But`, `Background`, `Rule` and `Scenario Outline` with `Examples` (one scenario per row).

````markdown
## 4. Invariants

- **INV-01**: A reset link works once.

## 6. Scenarios

```gherkin
Feature: Password reset

  Scenario: A user gets a reset link
    Given a registered user "Ana"
    When Ana asks for a reset
    Then a reset link is sent to Ana
    And the link expires in 30 minutes

  Scenario: INV-01 a used link fails
    Given a reset link that was used
    When the user opens it again
    Then it is refused: LINK_USED
```
````

Each scenario becomes one test (named with its marker, `SDD_0001_002` for the second), one RED → GREEN → REFACTOR and one commit. So:

- **One behaviour per scenario**: one `When`, at least one `Then`, a unique title.
- **Business words**, no UI or technology: "the user asks to reset", not "clicks the button" or "POST /reset".
- **Real values**: "expires in 30 minutes", not "expires soon".
- **Every invariant gets a scenario that tries to break it**, with the `INV-NN` in its title.
- **What you do not know** goes to section 12 as `- [NEEDS CLARIFICATION]: the question`. Never guess: the agent builds on what the specification says.

### Lint

Blocks approval:

- a `TODO` outside an HTML comment;
- an open question, `- [NEEDS CLARIFICATION]: …`;
- no Gherkin scenario, or Gherkin that does not parse;
- a scenario without `When` or without `Then`.

Advice only:

- a numbered template section is missing;
- an invariant `INV-NN` that nothing else mentions;
- a scenario with more than one `When`.

### Interview

`spec interview` completes the specification in a conversation SpecForge runs, one question at a time. Each turn is one call to your agent, which:

1. writes your last answer into the right section, in testable wording (behaviour becomes Gherkin; every invariant gets a failure scenario);
2. returns the single most important next question, why it matters, the section it completes and what is still unknown; or reports that nothing is.

SpecForge shows the progress (`section 4. Invariants · 6 unknown(s) left`), asks you, and records each question and answer in `specs/0001-slug/interview.jsonl` and in the decisions log. The agent may only change the specification: any other file is an error.

The interview ends only when the lint finds no `TODO` and no missing structure; an agent that says it is done too early is sent back with the issues. What you cannot answer becomes an open question for `spec clarify`, never a guess. The interview never approves or seals.

Without a terminal the question goes to `questions.md` (exit 5): write the answer there and run `spec interview` again; it continues where it stopped. `--chat` instead hands your terminal to the agent for a free conversation.

You can always skip the interview and edit the file by hand.

### Clarify

`spec clarify` asks every `[NEEDS CLARIFICATION]` and writes each answer in place of its question:

```markdown
- **Decided:** Does a newer link
  invalidate the old one? → Yes.
  (2026-10-04, Ana QA)
```

So the specification itself says what was decided. Answers also go to `specs/0001-slug/decisions.md`. `--by` sets the name recorded (default: `git config user.name`). Without a terminal the questions go to `questions.md` (exit 5); answer them there and run `clarify` again.

### Approve

`spec approve` lints, then records the approver (`--by`, else `git config user.name`, else a question) and the UTC time in the front matter, and appends the seal:

```markdown
<!-- seal: sha256-v1:9394d6452b3a… -->
```

The hash ignores line endings and trailing spaces, so a Windows checkout verifies the same. Specifications sealed by SpecForge 3 (`sha256:`) still verify.

### Changing an approved specification

Edit the file, then approve it again. Until you do, it is `changed` and the loop refuses it, so a stray edit never reaches the agent.

Every approval is appended to `specs/0001-slug/approvals.md` with the date, the approver, the seal and each scenario marked `ADDED`, `MODIFIED`, `UNCHANGED` or `REMOVED` compared with the previous approval. `approve` prints the changes, and the loop redoes only the scenarios whose text changed.

## 6. The plan: `plan`

```bash
specforge plan 0001
specforge plan approve 0001
```

`plan` asks your agent where the code goes before any code exists. It writes `specs/0001-slug/plan.md` with:

- the approach, in a few sentences;
- one line per file to create or change;
- a table with **one planned test per scenario** (by marker);
- the interfaces the domain needs and what implements them;
- the risks.

The agent sees the approved specification and the list of project files. It may only write `plan.md`: SpecForge rejects a draft that changes any other file, and sends it back (up to `max_attempts`) while a scenario has no planned test. When an architectural choice is not settled, the agent asks.

**Read the plan and edit it as you like**, then approve it (gate R1): approval checks that every scenario marker is there and no `TODO` is left, records you and seals the file. Running `plan` again revises the current draft instead of starting over.

The plan is optional. When `plan.md` exists, the loop requires it approved, unchanged and placing every scenario, and every prompt carries it. A specification amended with a new scenario needs the plan updated and approved again.

## 7. The loop: `loop`

```bash
specforge loop 0001
specforge loop --resume
specforge loop 0001 --restart
specforge loop 0001 --scenario 3 \
  --from green
specforge loop 0001 --review off \
  --no-commit
```

In order: start; continue after a stop, a question or Ctrl-C; discard the saved state and start over; redo one scenario from a phase; run without per-scenario review or commits.

For each scenario, in order:

- **🔴 RED.** The agent writes the test for this scenario, named with its marker (`SDD_0001_003`), plus the stubs it needs to compile. SpecForge accepts it only when a test file carrying the marker changed, the files the agent lists really changed, and the filtered test run compiles, runs at least one test and fails. A test that passes before any implementation is reported to you: either the behaviour exists already (mark the scenario satisfied) or the test is wrong.
- **🟢 GREEN.** The agent writes the minimum code. Accepted only when the test files are byte-for-byte as RED left them (otherwise the loop stops: *test tampering*) and the marker's tests pass. Failures are fed back, up to `max_attempts` (default 3).
- **🔵 REFACTOR.** The whole suite runs and so do the quality gates. The agent is only called when something blocks, and fixes it without touching tests.
- **👀 REVIEW** (gate R2). You see the files the scenario changed and the gates. **Accept**, type **what should change** (back to GREEN with your note; the tests stay) or send it **back to RED** (the test does not express the scenario). Without a terminal the review is a question in `questions.md` (exit 5). `review: off` skips it.
- **COMMIT.** The scenario's files and the decisions log become one commit, `feat(SDD_0001_003): <title>`. Nothing else you staged is touched; hooks and signing run as usual. Off with `commit: false` or `--no-commit`; skipped outside git.

When a test with the scenario's marker already exists (an earlier run stopped before RED was accepted, or you wrote it yourself), RED runs it before calling the agent. A test that compiles and fails is accepted as it is; a passing test goes to you as above; one that does not compile goes to the agent with the output. A scenario you mark as already satisfied is committed as `test(SDD_…)` with its test.

`--scenario N --from red|green|refactor` reopens one scenario and keeps the others. Starting after RED takes the current test files as the reference for the tampering check.

**Lessons.** When a phase needed more than one attempt, the agent may add a one-sentence `lesson`: the rule that would have avoided the mistake. SpecForge keeps it in `specs/LESSONS.md`, tagged with the stack, without duplicates and at most 30, and every later prompt for that stack shows them. The file is yours to edit.

**State.** The loop state is saved after every step in `.specforge/state/<spec>.json`, written atomically. `--resume` re-reads the specification and checks the seal first. If the specification was approved again with changes, the scenarios whose text did not change keep their progress and the rest are redone. A finished loop is reported, not redone; `--restart` runs it again on purpose.

**Test runners.** The marker filter uses `go test -json -run`, Maven `-Dtest`, Gradle `--tests`, `vitest run -t`, `jest -t` or `pytest -k`. Results are read from the runner's machine-readable report (test2json, Surefire/JUnit XML, Vitest/Jest JSON, pytest JUnit XML), so "did not compile", "nothing ran" and "an assertion failed" are told apart. With plain `npm test` the filter is not exact and SpecForge asks you to confirm the RED.

## 8. Questions instead of guesses

Every prompt ends with a response contract. The agent answers:

- `done`, with the files it wrote;
- `needs_clarification`: a question, optional choices and context;
- `blocked`: a reason and a suggested action.

Prose without the contract gets one retry, then the step stops: prose never counts as success.

**At a terminal**, the question is shown with numbered options. Your answer is appended to `specs/0001-slug/decisions.md` (date, phase, scenario) and sent back to the agent. The decisions log is part of every later prompt, so nothing is asked twice.

**Without a terminal** (CI, `--non-interactive`), the question is written to `specs/0001-slug/questions.md` and the command exits with code **5**:

```markdown
### 2026-10-04 22:03 · GREEN ·
    scenario 2 (`SDD_0001_002`)

- **Question:** Should an expired link
  say so, or look unknown?
  - Options: Say expired · Look unknown
- **Answer:** _awaiting an answer_
```

Replace `_awaiting an answer_` with your answer (or an option number) and run the same command again; for the loop, `specforge loop --resume`, in CI too. Or run it at a terminal and answer there. The resumed step starts with the answer and does not start over: the files the agent had already written still count. While a question has no answer, running again exits with code 5 without calling the agent.

**Blocked** stops with exit code 2 and the agent's suggested action.

The managed block in `CLAUDE.md`/`GEMINI.md` carries the same rule: if anything needed is not in the specification, the decisions log, the code or the prompt, do not assume it.

## 9. Quality gates

REFACTOR runs the gates that apply to the stack. Each reads its tool's machine-readable report and compares a number with a threshold. A tool that is missing or crashes is **skipped**, shown with ⚠, never as a pass. With `quality.strict: true` or `--strict`, a skipped gate blocks.

- **Lint**:
  - Go: golangci-lint, falling back to `go vet`.
  - Node: `npm run lint`.
  - Java: PMD, when `maven-pmd-plugin` is in the build (Maven).
  - Python: `ruff check`, the `.venv`'s first.
- **Duplication**, all stacks: jscpd over source code only (lock files, JSON, Markdown, tests and reports are not counted). Threshold `max_duplication_percent`, default 0.
- **Dead code**, Node: Knip.
- **Mutation**, Node: Stryker, when configured. Threshold `min_mutation_score`, default 80.
- **Migration**, Java: when `migration.java_release` or `forbidden_imports` is set, the release the build declares and every `import` in the Java sources. It needs no tool, so it never skips, and names file and line:

```text
src/main/java/…/Payroll.java:3:
  imports javax.servlet.http.HttpServlet
  (forbidden: javax.servlet)
```

Node tools run with `npx --no-install`: nothing is downloaded during the loop.

## 10. Security audit: `audit`

```bash
specforge audit
specforge audit --base v1.2.0
specforge audit --full --fail-on medium
```

Without `--base`, it audits your changes since `origin/main`, `origin/master`, `main` or `master`. `--full` audits the whole project.

Three passes run with your agent over each chunk of code:

1. **reconnaissance**;
2. a red-team **hunter** that proposes findings with file, line and evidence;
3. a blue-team **validator** that confirms or rejects each one.

Every answer must match the embedded JSON schema. An invalid answer gets one retry, then the audit stops with an error, never with an empty passing report (*fail-closed*). Findings the validator cannot settle at or above the threshold are questions for you. A confirmed finding at or above `--fail-on` (default `high`) exits with code 2.

The base ref is validated as a commit and passed after `--end-of-options`, so it can never be read as a git option. Reports go to `docs/security/` (`REPORT.md`, `findings.json`, `coverage-ledger.json`) with owner-only permissions.

## 11. Browser verification: `e2e`

```bash
specforge e2e 0001 \
  --url http://localhost:3000
specforge e2e 0001 \
  --url http://localhost:3000 \
  --scenario 2 --headed
```

`e2e` drives Chrome, Chromium or Edge (found on your system, or `CHROME_PATH`) through [chromedp](https://github.com/chromedp/chromedp), scenario by scenario. The agent sees a compact view of the page (URL, title, interactive elements with unique selectors, visible text) and answers with one typed action at a time: `navigate`, `click`, `type`, `select`, `press`, `scroll`, `wait`, `assert` or `fail`.

Before each action SpecForge checks that the selector exists on the page and that navigation stays on the same origin. An `assert` names a `Then` and the evidence (text, selector or URL); SpecForge verifies the evidence itself. The page is untrusted input: its text never becomes an instruction.

Each step saves a screenshot. Results go to `docs/e2e/<spec>/` (`report.json`, `REPORT.md`, `scenario-NN/step-MM.png`). The command exits with code 2 when the pass rate is below `--min-pass-rate` (default 100). `--max-steps` bounds the actions per scenario (default 15). `--insecure` accepts invalid TLS certificates for local test servers; it is off by default. Only approved specifications run.

## 12. Hand-over: `deliver`

```bash
specforge deliver 0001
gh pr create --body-file \
  specs/0001-password-reset/PR_BODY.md
```

`deliver` writes three files next to the specification, built only from what SpecForge recorded, never from the agent's word:

- **`DELIVERY.md`**, for people: who approved the specification and the plan, and when. One row per scenario with its tests (file and test names carrying the marker), its commit, its gates (✓ passed · ⚠ skipped · ✗ failed) and notes (already satisfied, changes requested in review, rejected attempts). Then the decisions taken, the questions still open, the lessons, the audit and E2E results, and what is out of scope.
- **`trace.json`**, for tools: the same data, machine-readable.
- **`PR_BODY.md`**, for the pull request: a summary, the scenario table, the decisions, the checks and what is not done. When the repository has a pull request template (`.github/pull_request_template.md` and the other places GitHub looks), its headings are kept and the summary goes under the first one.

A delivery is honest about gaps: unfinished scenarios, open questions and checks that did not run are listed, and an incomplete delivery says so in its first line. Reviews and SpecForge's own verification questions are shown per scenario, apart from your product decisions.

The loop state lives in `.specforge/`, so run `deliver` where the loop ran. Commit `trace.json` to keep the trace.

## 13. Legacy rewrites: `legacy`, `spec from-legacy`

Rewriting a legacy system (a Java 6 servlet application on Java 21, for example) happens in a **new project next to the old one**. The legacy repository is evidence, not a workspace: the agent may read it, and SpecForge hashes it before and after every agent turn (map, specification, plan and every loop phase) and refuses any change.

```bash
mkdir payroll && cd payroll && git init
specforge setup --new java \
  --legacy ../legacy-payroll
specforge legacy scan
specforge legacy map
specforge spec from-legacy "Net pay"
specforge spec clarify 0001
specforge spec approve 0001
specforge plan 0001
specforge plan approve 0001
specforge loop 0001
```

`setup --legacy` writes the migration settings (by hand works too):

```yaml
# specforge.yaml
migration:
  # relative to the project, or absolute
  legacy: ../legacy-payroll
  # the build must declare it
  java_release: 21
  # optional; default: see below
  forbidden_imports:
    - javax.servlet
    - org.apache.log4j
```

With `java_release` set and no list of its own, the forbidden imports are the Java EE packages Jakarta renamed (`javax.servlet`, `javax.persistence`, `javax.validation`, `javax.ejb`, `javax.jms`, `javax.ws.rs`, `javax.xml.bind`, `javax.xml.rpc`, `javax.annotation`, `javax.inject`, `javax.faces`, `javax.transaction`), Log4j 1, JUnit 3, `Vector` and `Hashtable`.

### `legacy scan [path]`

Writes `docs/legacy/INVENTORY.md`, measured, without an agent: the build tool, the declared Java release (the lowest of `pom.xml`, Gradle and Ant), the size, the frameworks found from imports and files, the largest packages, and what each finding means for Java 21.

### `legacy map [path]`

The agent reads the legacy code and writes `docs/legacy/CAPABILITIES.md`: one section per business capability with what it does, its entry points, its rules with their sources, its data, what it depends on and what is unclear. Capabilities come in migration order: what others depend on first.

SpecForge checks that only that file changed, that there is at least one capability, and **opens every citation** (`` `path:line` `` or `` `path:from-to` ``) in the legacy code. A file that does not exist or a line past its end sends the map back with the list. Review the map before writing specifications from it; it is yours to edit.

### `spec from-legacy "<capability>"`

Creates the next specification, and the agent fills it with the behaviour **as it is today**: rules as invariants, scenarios with the real values and messages, the real errors, and a last section citing the code of each rule and scenario:

```markdown
## 13. Legacy sources

- INV-02: `src/…/Payroll.java:58`
- Scenario "Net pay is truncated":
  `src/…/Payroll.java:61`
```

SpecForge checks that only the specification changed, the template lint (open questions aside), that the sources section exists, and that every citation in it, and every line citation elsewhere, resolves. One capability per specification: the others go to *Out of scope*. Run it again with the same capability to continue the draft instead of creating another. `--legacy` overrides the path in `specforge.yaml`.

What the code does that nobody can explain (a rule applied in one place and not another, floating-point money, dead code) is written as `[NEEDS CLARIFICATION]` with its source. Answer them with `spec clarify` (or edit them), then approve: the specification is now the contract the new code is tested against.

During `plan` and `loop` the agent receives the legacy path, the sources section and the target release, and reads the cited code to reproduce the behaviour. The migration gate checks the result.

## 14. Your files: what to edit

Everything SpecForge uses lives in your repository, next to your code. `0001-slug` stands for each specification's number and name.

### Files you work in

- 📝 **`specs/0001-slug.md`**: the specification. Edit it freely before approving. To change an approved one, edit it and run `spec approve` again.
- 🗺️ **`specs/0001-slug/plan.md`**: the plan. After `plan`, edit what you like, then `plan approve`.
- ❓ **`specs/0001-slug/questions.md`**: questions waiting for you when nobody was at a terminal. Replace `_awaiting an answer_` and run the same command again.
- ⚙️ **`specforge.yaml`**: project settings ([section 16](#16-configuration-reference)).
- 🤖 **`CLAUDE.md`, `GEMINI.md`**: your agent's instructions. Write anything outside the SpecForge block; `setup` refreshes only the block.
- 📚 **`specs/LESSONS.md`**: lessons the agent wrote after a failed attempt. Prune or reword them.
- 🏛 **`docs/legacy/CAPABILITIES.md`**: the legacy capability map.

### Files SpecForge writes

Read them; don't edit them by hand.

- The **seal**, the `<!-- seal: … -->` line at the end of a specification or a plan. Approve again instead.
- The front matter fields **`approved_by`** and **`approved_at`**, written by approval.
- **`specs/0001-slug/approvals.md`**: every approval and what changed in it.
- **`specs/0001-slug/decisions.md`**: every question and answer, reused in later prompts.
- **`specs/0001-slug/interview.jsonl`**: the interview transcript.
- **`specs/0001-slug/DELIVERY.md`, `trace.json`, `PR_BODY.md`**: the hand-over, rewritten by each `deliver`.
- **`docs/legacy/INVENTORY.md`**: the legacy inventory, rewritten by each scan.
- **`docs/legacy/decisions.md`, `questions.md`**: questions asked while mapping the legacy code.
- **`docs/security/`**: audit reports, owner-only permissions.
- **`docs/e2e/<spec>/`**: E2E reports and screenshots.

### Not committed

- **`.specforge/state/<spec>.json`**: the loop state for `--resume`. `setup` adds `.specforge/` to `.gitignore`.
- The **log file**, in your user cache directory (`~/.cache/specforge/logs/specforge.log` on Linux), rotated, owner-only.

Commit everything else: the specifications, plans, logs and the delivery are the project's history.

## 15. Recipes

**Change an approved specification.** Edit `specs/0001-slug.md`, then:

```bash
specforge spec approve 0001
specforge loop --resume
```

Approval prints which scenarios were added, modified or removed; the loop redoes only those.

**Add a scenario to an approved specification.** Add it to section 6 and approve. If there is a plan, it must place the new scenario:

```bash
specforge spec approve 0001
specforge plan 0001
specforge plan approve 0001
specforge loop --resume
```

**Change the approved plan.** Edit `plan.md` and run `specforge plan approve 0001`.

**Answer a question without a terminal.** Open `questions.md`, replace `_awaiting an answer_` with the answer or an option number, and run the same command again (`loop --resume` for the loop).

**Redo one scenario.** `specforge loop 0001 --scenario 3 --from green` (or `red`, `refactor`). The other scenarios keep their state.

**Start the loop over.** `specforge loop 0001 --restart`.

**Continue after Ctrl-C or a stop.** `specforge loop --resume`. With several saved loops, name the specification.

**The loop says a test was tampered with.** Something changed a test file after RED. Revert it (`git checkout -- <file>`), then `loop --resume`.

**A test passed before any code.** SpecForge asks: mark the scenario as already satisfied, ask the agent for a stricter test, or stop.

**Run without reviews or commits.** `--review off --no-commit`, or `review: off` and `commit: false` in `specforge.yaml`.

**Use another agent or model once.** `--agent gemini` or `--model <name>` on `spec interview`, `plan`, `loop`, `audit`, `e2e`, `legacy map` and `spec from-legacy`.

**Run in CI.** Add `--non-interactive`. Questions go to `questions.md` with exit code 5; every other outcome has its own [exit code](#17-exit-codes). `--json` prints data and errors as JSON.

**See exactly what the agent received.** Add `--trace-io` and read the log file.

**Make a missing tool block.** `--strict`, or `quality.strict: true`.

**Open a pull request.** `specforge deliver 0001`, then `gh pr create --body-file specs/0001-slug/PR_BODY.md`.

## 16. Configuration reference

Values resolve in this order: command-line flag, then `specforge.yaml`, then your user configuration, then the default. Every key is optional.

```yaml
# specforge.yaml
# es | en: prompts, templates, messages
language: en
# go | maven | gradle | node | python
# (detected when unset)
stack: go
# usually each developer's choice
agent: claude
# empty: the agent's default
model: ""
# attempts per phase
max_attempts: 3
# scenario: review each one (R2) | off
review: scenario
# one commit per finished scenario
commit: true
timeouts:
  agent: 20m
  tests: 10m
quality:
  # true: a skipped gate blocks
  strict: false
  max_duplication_percent: 0
  min_mutation_score: 80
# only for a rewrite (section 13)
migration:
  legacy: ../old-system
  java_release: 21
  forbidden_imports:
    - javax.servlet
    - org.apache.log4j
```

Unknown keys are an error, so a typo never silently leaves a default in place.

**Global flags**

- `--non-interactive`: never ask; write questions to a file and exit 5.
- `--json`: data from `version`, `spec list` and `deliver` as JSON on stdout, and errors as `{"exit", "title", "cause", "action"}` on stderr.
- `--quiet`: only warnings, errors and data.
- `--verbose`: progress details and the agent's live output.
- `--debug`: debug records in the log.
- `--trace-io`: every prompt and answer in the log.

## 17. Exit codes

- **0**: done.
- **1**: unexpected error or bad usage.
- **2**: a gate said no: tests, quality, security findings, E2E pass rate, a legacy document that was not accepted, or the agent is blocked.
- **3**: the specification or the loop state needs attention: not approved, changed after approval, lint issues, ambiguous or missing, test tampering, a loop in progress.
- **4**: a tool, browser or setting is missing: agent CLI, test runner, configuration, legacy repository.
- **5**: a question awaits your answer (no terminal).
- **130**: interrupted with Ctrl-C; the state saved so far is valid.

Every failure prints what happened, why and what to do next.

## 18. Troubleshooting

- **What did the agent receive and answer?** Re-run with `--trace-io` and read the log file.
- **The agent claims files it did not write.** The attempt is rejected and the agent is told which ones; that is the loop working. Repeated rejections exhaust `max_attempts` (exit 2).
- **"Several stacks detected".** Set `stack:` in `specforge.yaml` or pass `--stack`.
- **The loop says the specification changed.** Revert the edit, or review it and run `specforge spec approve` again; unchanged scenarios keep their progress.
- **A gate shows ⚠ skipped.** Install the tool, or accept the warning; `--strict` makes it block.
- **"The agent changed the legacy code".** Restore the legacy repository (`git checkout .` there) and run the command again; the agent may only read it.
- **A legacy document keeps coming back.** The diagnosis lists each citation that did not resolve, and the agent gets the same list. Citations are relative to the legacy repository.
- **`deliver` says 0 scenarios finished.** The loop state is local: run `deliver` in the clone where the loop ran.

## 19. Architecture

SpecForge follows the architecture it asks of your code: the domain is pure, use cases depend on small ports, adapters do the I/O and `cmd/` only wires them.

```text
cmd/            CLI, composition root,
                exit codes
internal/
  domain/       pure, no I/O:
    spec/       Gherkin, seal, lint
    legacy/     inventory, citations,
                migration conformance
    tdd/ stack/ quality/ lessons/
    delivery/ security/ e2e/
  app/          use cases:
    tddloop/    the TDD loop
    specs/      spec lifecycle
    planning/   the plan
    interview/  the interview
    docturn/    checked document turns
    migrate/    legacy scan, map, spec
    scaffold/   setup --new
    deliver/ audit/ e2erun/ setup/
    conversation/ clarify/ protocol/
    prompts/ layout/
  ports/        what use cases need
  adapters/     agent CLI, processes,
                runners, gates, git,
                browser, files, logs
  config/ ui/   settings, terminal UI
assets/         prompts, rules,
                standards, templates,
                scaffolds
```

Tests run the real CLI against scripted agents and real `go test`, `git` and Chromium where available: `make test`. Releases are built by GoReleaser when a `v*` tag is pushed.
