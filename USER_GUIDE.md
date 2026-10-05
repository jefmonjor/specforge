# SpecForge user guide

SpecForge drives a coding agent ([Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Gemini CLI](https://github.com/google-gemini/gemini-cli)) through a test-first loop against a specification you approved, and checks every step itself instead of trusting the agent's word.

This guide covers every command, every file SpecForge writes, what you edit by hand, and every exit code. New here? Read sections 1 to 9 once, then keep [Your files](#17-your-files-what-to-edit) and [Recipes](#18-recipes) at hand.

**Start**
- [1. How it works](#1-how-it-works)
- [2. Install](#2-install)
- [3. Configure once: `init`](#3-configure-once-init)
- [4. Prepare a repository: `setup`](#4-prepare-a-repository-setup)
- [5. Check the machine: `doctor`](#5-check-the-machine-doctor)

**Daily work**
- [6. Specifications: `spec`](#6-specifications-spec)
  - [Three ways in](#three-ways-in-functional-technical-from-code) · [Changing a specification: `spec change`](#changing-a-specification-spec-change)
- [7. The plan: `plan`](#7-the-plan-plan)
- [8. The loop: `loop`](#8-the-loop-loop)
- [9. Risk, review and verification](#9-risk-review-and-verification)
- [10. Questions instead of guesses](#10-questions-instead-of-guesses)
- [11. Quality gates](#11-quality-gates)
- [12. Security audit: `audit`](#12-security-audit-audit)
- [13. Browser verification: `e2e`](#13-browser-verification-e2e)
- [14. Hand-over: `deliver`](#14-hand-over-deliver)
- [15. Legacy rewrites: `legacy`, `spec from-legacy`](#15-legacy-rewrites-legacy-spec-from-legacy)
- [16. The destructive-command guard: `guard`](#16-the-destructive-command-guard-guard)
  - [What it reads](#what-it-reads) · [Checkpoints: `restore`](#checkpoints-restore)

**Reference**
- [17. Your files: what to edit](#17-your-files-what-to-edit)
- [18. Recipes](#18-recipes)
- [19. Configuration reference](#19-configuration-reference)
- [20. Exit codes](#20-exit-codes)
- [21. Troubleshooting](#21-troubleshooting)
- [22. Architecture](#22-architecture)

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
             → lenses by risk, verifier
             → your review     (R2)
             → one commit
           └ audit, e2e
             └ deliver         (R4)
```

- A **specification** (`specs/0001-slug.md`) says *what* to build and *why*, with the acceptance criteria as Gherkin scenarios.
- **Approval** is a human gate. It refuses while a `TODO` or an open question is left, records who approved and when, and seals the content with a SHA-256 hash. The loop only runs an approved, unchanged specification.
- The **plan** says where the code goes, with one planned test per scenario. You review and approve it before any code exists; the loop follows it.
- The **loop** takes one scenario at a time. The agent writes code; SpecForge runs the tests, compares the files on disk before and after each turn, fingerprints the test files after RED and runs the quality gates.
- After REFACTOR, each scenario is **reviewed in proportion to its risk**: review lenses whose findings must point at changed lines, and, for risky work, an independent verifier that probes the specification in a copy of the project.
- When the agent lacks information it **asks** instead of guessing. You answer once; the answer is recorded and reused.
- A **guard** installed as your agent's hook blocks destructive shell commands before they run.

## 2. Install

Download your binary from the [latest release](https://github.com/jefmonjor/specforge/releases/latest). Each release has `specforge-<os>-<arch>` files and `checksums.txt`.

```bash
R=https://github.com/jefmonjor/specforge
R=$R/releases/latest/download
# or darwin-amd64, linux-amd64,
# linux-arm64
F=specforge-darwin-arm64
curl -LO $R/$F
curl -LO $R/checksums.txt
sha256sum -c checksums.txt \
  --ignore-missing
chmod +x $F
sudo mv $F /usr/local/bin/specforge
specforge version
```

`sha256sum` must print `OK` for your file; on macOS use `shasum -a 256 -c` instead. Verify before renaming the file: the check matches by name.

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

The quality gates use more tools when they are installed: see [section 11](#11-quality-gates).

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

`setup` writes these files and never touches your code:

- **`specforge.yaml`**: project settings shared by the team, every default commented. Kept as it is on a second run.
- **`CLAUDE.md` / `GEMINI.md`**: a block between `<!-- specforge:begin … -->` and `<!-- specforge:end -->` with the working rules (ask, don't invent; tests are the contract; craft) and your stack's coding standard. Both agents read these files at every session. A second run refreshes the block; the rest of the file is yours.
- **`.claude/settings.json`** or **`.gemini/settings.json`**: the [destructive-command guard](#16-the-destructive-command-guard-guard) as the agent's pre-tool hook, merged into what is there. `--no-guard` or `guard.mode: off` skips it.
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

## 5. Check the machine: `doctor`

```bash
specforge doctor
specforge doctor --json
```

`doctor` changes nothing. It checks, one line each:

- **configuration**: `specforge.yaml` and your user settings are valid (an unknown key or value is reported here instead of stopping);
- **agent**: the configured CLI is on your `PATH`, with its version;
- **git**: installed, and `user.name` set when the loop commits;
- **tests**: your stack's runner, preferring the project's wrapper (`mvnw`, `gradlew`) or `.venv`;
- **gates**: each quality gate's tool (golangci-lint, Ruff, PMD in the build, jscpd, Knip, Stryker);
- **legacy**: the legacy repository of a rewrite exists;
- **browser**: Chrome, Chromium or Edge for `e2e`;
- **guard hook**: installed in your agent's settings.

✓ is ready, ⚠ works with a caveat, ✗ is missing; every ⚠ and ✗ comes with the command that fixes it. The exit code is **4** when something required is missing, **0** otherwise.

```text
  ✓ agent · claude 2.1.289
  ✓ git · git version 2.43.0
  ✓ tests · /usr/local/go/bin/go
  ✓ gate lint · golangci-lint
  ⚠ gate duplication · jscpd not
    found
      → npm install -g jscpd
  ✓ guard hook ·
    .claude/settings.json runs
    `specforge guard`
```

## 6. Specifications: `spec`

```bash
specforge spec new "Password reset"
specforge spec interview 0001
specforge spec clarify 0001
specforge spec lint 0001
specforge spec approve 0001
specforge spec change 0001 "Links expire after 15 minutes"
specforge spec list
```

- **`new`** creates `specs/0001-password-reset.md` from the template.
- **`interview`** completes it in a conversation with your agent.
- **`clarify`** asks the open questions, one at a time, then has your agent write the decisions everywhere they apply.
- **`change`** applies a change request, approved specification or not.
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

A specification drafted from legacy code adds `13. Legacy sources` ([section 15](#15-legacy-rewrites-legacy-spec-from-legacy)).

### Three ways in: functional, technical, from code

Every specification follows the same template and the same lifecycle; only how the first draft comes changes.

- **A feature** (*password reset*, *discount codes*): `spec new`, then `spec interview`. The interview starts from the intent, the actors and the rules that are never broken.
- **A technical requirement** (*cache exchange rates for ten minutes*, *rate-limit the login*, *structured logs*): the same `spec new` and `spec interview`. The actors are the systems involved (*the checkout service*, *the rate provider*); the invariants are the guarantees (*never more than 100 provider calls per minute*, *never an expired rate unmarked*); section 9 holds the numbers; the scenarios describe what another system observes, not the code. A real run turned *cache exchange rates for ten minutes* into twelve scenarios, one per guarantee, in nine questions.
- **Existing code, in any language**: `setup --legacy <repo>`, `legacy scan`, `legacy map`, then `spec from-legacy "<capability>"` drafts the specification from what the code really does, citing each source ([section 15](#15-legacy-rewrites-legacy-spec-from-legacy)). What the code does that nobody can explain becomes an open question; `spec clarify` asks them and writes the decisions into the scenarios.

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

You can always skip the interview and edit the file by hand. On an approved specification, `interview` does nothing and points to `spec change`.

### Clarify

`spec clarify` asks every `[NEEDS CLARIFICATION]` and writes each answer in place of its question:

```markdown
- **Decided:** Does a newer link
  invalidate the old one? → Yes.
  (2026-10-04, Ana QA)
```

So the specification itself says what was decided. Answers also go to `specs/0001-slug/decisions.md`. `--by` sets the name recorded (default: `git config user.name`). Without a terminal the questions go to `questions.md` (exit 5); answer them there and run `clarify` again.

A decision usually changes more than its own line: a message, a limit, a rounding rule. Once every question is answered, your agent writes the decisions into every section they affect (scenarios, invariants, data contracts, errors, out of scope), the way `spec change` does, asking if one is unclear. In a real run on legacy Python code, *"Half-up to two decimals, on exact decimal amounts"* became a changed invariant, a changed data contract and a new scenario pinning the case where the legacy code's floating point rounded the wrong way. `--no-apply` only records the decisions.

### Approve

`spec approve` lints, then records the approver (`--by`, else `git config user.name`, else a question) and the UTC time in the front matter, and appends the seal:

```markdown
<!-- seal: sha256-v1:9394d6452b3a… -->
```

The hash ignores line endings and trailing spaces, so a Windows checkout verifies the same. Specifications sealed by SpecForge 3 (`sha256:`) still verify.

### Changing a specification: `spec change`

```bash
specforge spec change 0001 "A tax rate below 0 or above 100 is refused with INVALID_RATE"
```

Your agent applies the request to every section it touches, asks what the request leaves open or contradicts, and changes no other file; the conversation is kept in `specs/0001-slug/change.jsonl`. It ends by showing what the next approval will record. You can also edit the file by hand: either way, until you approve it again the specification is `changed` and the loop refuses it, so a stray edit never reaches the agent.

**A scenario keeps its marker for good.** The marker (`SDD_0001_003`) names its tests, its review and verification records, and its row in the plan, so it never moves:

| You… | The scenario | The loop |
| :--- | :--- | :--- |
| edit its steps, same title | keeps its marker: `MODIFIED` | redoes it from RED, showing the agent what it said before, so its test is updated rather than taken as done |
| change its title, same steps | keeps its marker: `RENAMED` | nothing to redo |
| change an invariant it names | `MODIFIED` | redoes it |
| insert, move or reorder scenarios | the others keep theirs | nothing to redo for them |
| change its title and its steps together | a new marker (`ADDED`); the old one is `REMOVED` | does it as new; the plan must follow |
| add one | gets the next number, never a used one | does it; the plan must place it first |
| remove one | its number is retired: `REMOVED` | warns that its tests are still in the project |

`spec approve` keeps the markers in `specs/0001-slug/scenarios.json` and appends each approval to `specs/0001-slug/approvals.md` (date, approver, seal, every scenario with its marker and how it changed). When scenarios were added or removed and a plan exists, it says to revise the plan: `specforge plan 0001` revises the existing plan rather than starting over, and a plan that names a scenario that is gone is refused. A specification approved before SpecForge 6.1 takes its markers from its loop state in `.specforge/` at its next approval, so its tests keep their names. Make that approval in the clone where the loop ran, then commit `scenarios.json`; elsewhere the markers follow the scenarios' order, and `spec approve` and `spec change` say so. Rename scenarios after that first approval: until then a new title reads as a new scenario.

## 7. The plan: `plan`

```bash
specforge plan 0001
specforge plan approve 0001
```

`plan` asks your agent where the code goes before any code exists. It writes `specs/0001-slug/plan.md` with:

- the approach, in a few sentences;
- one line per file to create or change, the path in backticks, ending with the markers of the scenarios it serves when it does not serve them all;
- a table with **one planned test per scenario** (by marker);
- the interfaces the domain needs and what implements them;
- the risks.

The agent sees the approved specification and the list of project files. It may only write `plan.md`: SpecForge rejects a draft that changes any other file, and sends it back (up to `max_attempts`) while a scenario has no planned test. When an architectural choice is not settled, the agent asks.

**Read the plan and edit it as you like**, then approve it (gate R1): approval checks that every scenario marker is there, none is a removed one and no `TODO` is left, then records you and seals the file. Approving a plan that is already approved checks it again. Running `plan` again revises the current draft instead of starting over.

The plan is optional. When `plan.md` exists, the loop requires it approved, unchanged and placing every scenario, and every prompt carries it. Its files become the agent's [edit surfaces](#edit-surfaces-from-the-plan), and its scenario markers decide which scenarios may run [side by side](#scenarios-side-by-side). Approval warns about a component line without a path in backticks. A specification approved again with a scenario added or removed needs the plan updated and approved again; until then the loop stops with exit 3.

## 8. The loop: `loop`

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
- **🔵 REFACTOR.** The whole suite runs and so do the quality gates. Tests that [already failed before the loop](#known-failures) do not block. The agent is only called when something blocks, and fixes it without touching tests.
- **🔎 REVIEW and VERIFY.** The scenario's [risk](#risk-tiers) is measured; the [review lenses](#review-lenses) and, for high risk, the [independent verifier](#the-independent-verifier) check it. What they prove broken gets one correction, judged again by REFACTOR.
- **👀 Your review** (gate R2). You see the files the scenario changed and the gates. **Accept**, type **what should change** (back to GREEN with your note; the tests stay) or send it **back to RED** (the test does not express the scenario). Without a terminal the review is a question in `questions.md` (exit 5). `review: risk` asks only for medium and high-risk scenarios; `review: off` skips it.
- **COMMIT.** The scenario's files and the decisions log become one commit, `feat(SDD_0001_003): <title>`. Nothing else you staged is touched; hooks and signing run as usual. Off with `commit: false` or `--no-commit`; skipped outside git.

When a test with the scenario's marker already exists (an earlier run stopped before RED was accepted, or you wrote it yourself), RED runs it before calling the agent. A test that compiles and fails is accepted as it is; a passing test goes to you as above; one that does not compile goes to the agent with the output. A scenario you mark as already satisfied is committed as `test(SDD_…)` with its test.

`--scenario N --from red|green|refactor` reopens one scenario and keeps the others. Starting after RED takes the current test files as the reference for the tampering check.

**Lessons.** When a phase needed more than one attempt, the agent may add a one-sentence `lesson`: the rule that would have avoided the mistake. SpecForge keeps it in `specs/LESSONS.md`, tagged with the stack, without duplicates and at most 30, and every later prompt for that stack shows them. The file is yours to edit.

**State.** The loop state is saved after every step in `.specforge/state/<spec>.json`, written atomically. `--resume` re-reads the specification and checks the seal first. If the specification was approved again with changes, progress is matched by marker: `UNCHANGED` and `RENAMED` scenarios keep everything (commit, risk, review, verification), `MODIFIED` ones go back to RED with their previous text in the prompt, `ADDED` ones are done, and for `REMOVED` ones the loop warns about tests that still carry their marker. A finished loop is reported, not redone; `--restart` runs it again on purpose.

**Test runners.** The marker filter uses `go test -json -run`, Maven `-Dtest`, Gradle `--tests`, `vitest run -t`, `jest -t` or `pytest -k`. Results are read from the runner's machine-readable report (test2json, Surefire/JUnit XML, Vitest/Jest JSON, pytest JUnit XML), so "did not compile", "nothing ran" and "an assertion failed" are told apart. With plain `npm test` the filter is not exact and SpecForge asks you to confirm the RED.

## 9. Risk, review and verification

After REFACTOR, every scenario gets the scrutiny its change deserves, measured, not guessed.

### Known failures

A new loop runs the whole suite once before its first RED. The tests that already fail there are the **baseline**: named from the runner's report, shown to you, listed for the agent as *not yours to fix*, and kept out of REFACTOR's verdict. A test that did not fail before and fails now blocks as usual; a known one that starts passing leaves the baseline, so it cannot break again unnoticed. A runner without names (plain `npm test`) keeps every failure blocking. `DELIVERY.md` lists the known failures apart.

### Risk tiers

Each scenario's change is classified from what git says it touched (its files' added and deleted lines; lock files and vendored code do not count):

- **high**: a sensitive path (`auth`, `security`, `secret`, `token`, `password`, `payment`, `billing`, `permission`, `process`, `exec`, `migration`, `infra`, `deploy`, `ci`, CI workflows, `go.mod`, `package.json`, `pom.xml` and the other build files), or more than 400 changed lines;
- **passive**: documentation only;
- **medium**: everything else.

The reasons are printed and kept. The agent can **raise** the tier with `"risk": "high"` and a `"risk_reason"` in its answer (recorded as a decision), never lower it. `--strict` makes every change high. Tune it in `specforge.yaml`:

```yaml
risk:
  max_lines: 400
  floor: passive
  high_paths: ["(^|/)ledger/"]
```

The tier decides the rest: how many review lenses run, whether the verifier runs, whether the slow mutation gate runs (`quality.mutation_from`, default `medium`) and, with `review: risk`, whether you are asked to review the scenario at all.

### Review lenses

Lenses are read-only agent turns over the scenario's diff, each with one angle:

- **risk**: security, data, money, permissions, process execution;
- **reliability**: correctness against the specification and its invariants;
- **readability**: names from the ubiquitous language, structure, duplication;
- **resilience**: timeouts, retries, partial writes, concurrency, leaks.

A passive change gets none, a medium one gets one (risk when a path is sensitive, else reliability), a high one gets all four. `lenses: off` or a list in `specforge.yaml` overrides it.

Every finding has a severity, how the lens knows (`deterministic`, `inferential`, `insufficient`), whether the change caused it (`introduced`, `behavior-activated`, `worsened`, `pre-existing`, `base-only`, `unknown`) and **proof references** to lines of the change. SpecForge checks each answer and each proof itself:

- an answer that does not validate against the schema gets one retry, then the review fails closed (exit 2);
- a lens that changes a file is refused;
- a finding whose proof is not a line the change added or modified, or a file it created, is **discarded**, with the reason, in the record;
- only a BLOCKER or CRITICAL the change caused, with evidence, blocks; pre-existing ones become follow-ups; one whose cause is unknown goes to you;
- an inferential blocker goes to an independent **refuter** first: only a confirmed one blocks.

What blocks gets **one correction**: a GREEN-model turn with the findings, the tests untouched, the plan's surfaces checked, and a budget of half the scenario's changed lines (200 at most), counted line by line. A larger correction is a redesign and goes to you. REFACTOR then judges the corrected code, and a **validation** turn checks only the corrected findings. A regression goes to you; there is never a second automatic correction.

With `blind_review: true`, a high-risk change runs every lens twice, independently (`models.review2` for the second pass). What both passes prove on the same hunk skips the refuter; what only one found has to survive it.

Everything is kept in `specs/0001-slug/review/SDD_….json`, and `--resume` never runs the lenses twice.

**A whole branch.** `specforge review [spec] [--base ref] [--lens risk]` runs the same lenses and checks over your branch, for code that did not come out of the loop. It never corrects anything: exit 2 when a finding blocks or needs your judgement. With a specification, the report goes to `specs/0001-slug/review/branch.json` and `deliver` shows it. A documentation-only branch needs no lens, and `review` says so.

### The independent verifier

The writer's test can pin a bug: a test written from the code agrees with the code. The verifier checks the **specification** instead. In a disposable copy of your project (dependency directories linked, plus a copy of the last commit to compare old behaviour; files are cloned copy-on-write on APFS, Btrfs and XFS, so the copy costs almost nothing there), an agent derives its own probes from every invariant and scenario and runs them. It is the only agent allowed to run shell commands without asking, because nothing it does there is kept; the guard still applies.

SpecForge requires:

- a verdict, `met`, `unmet` or `unverified`, for **every** invariant and scenario asked;
- for every broken one, the **exact command** and the output it **observed**; SpecForge runs the command again and refuses a failure it cannot reproduce;
- your real project unchanged, fingerprinted before and after;
- an answer valid for its schema (one retry, then fail closed).

`verify: high` (the default) runs it for high-risk scenarios, `always` for every one, `feature` once over the whole specification at the end, `off` never. What it shows broken gets the one correction, REFACTOR judges it, and only those requirements are verified again; what stays broken goes to you. The regression tests it proposes are offered: added only as new test files inside the project, with the whole suite still passing. A project larger than `verify_max_mb` (500) is skipped, with the reason; the same limit applies to the sandboxes of [scenarios side by side](#scenarios-side-by-side).

`specforge verify 0001` runs it over a whole specification on demand: exit 2 with the command that reproduces each broken requirement, the report in `specs/0001-slug/verify/feature.json`.

### Edit surfaces from the plan

With an approved plan, the files the agent may change are its components (paths in backticks), its planned test files, and anything under the directory of a new component; every RED, GREEN and REFACTOR prompt lists them. After each turn, a file outside them goes to you (`plan.surfaces: ask`): accept it for this specification, or refuse it and the agent has to put it back, which the next turns check by fingerprint. A scenario still carrying a refused file is never committed. `strict` refuses without asking; `off` does not check. SpecForge's own records and lock files are never outside the plan.

### Scenarios side by side

With `loop.parallel: 2` (or more), consecutive scenarios whose plan surfaces do not overlap run at the same time, each in its own **sandbox**: a copy of the project made into a fresh git repository that borrows the project's git objects, so it stores only what is new. A component line that names scenario markers (`· SDD_0001_002`) belongs to those; one that names none is shared by all, so an unmarked plan never runs anything in parallel.

In its sandbox each scenario goes through RED, GREEN, REFACTOR and the review. Questions reach you one at a time. Then the work comes back in order: a file another scenario of the batch also wrote sends that scenario back to run on its own; the whole suite runs once over the combination (the **seam check**), and if it fails every file goes back as it was and the scenarios run again one by one. Only then does each scenario get your review and its own commit, in turn.

The scenarios share the specification's logs. What each one adds to `decisions.md`, `questions.md` and `LESSONS.md` in its sandbox is appended to the project's, never copied over it, so no scenario loses another's answers. A scenario that stops in its sandbox (a question nobody can answer there, a collision, a failed seam check) runs again on its own right after the batch: your product answers from the first attempt still count, but its process decisions (risk, review, verification) stay behind with the attempt.

### A model per phase

```yaml
model: claude-sonnet-5-5
models:
  plan: claude-opus-5-5
  green: claude-haiku-4-5
  review: claude-opus-5-5
```

Phases: `interview`, `plan`, `legacy`, `red`, `green`, `refactor`, `review`, `review2`, `refute`, `verify`, `audit`, `e2e`. The review's correction uses `green`'s model; `spec change` and `spec clarify` use `interview`'s. `--model` overrides every phase.

## 10. Questions instead of guesses

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

## 11. Quality gates

REFACTOR runs the gates that apply to the stack. Each reads its tool's machine-readable report and compares a number with a threshold. A tool that is missing or crashes is **skipped**, shown with ⚠, never as a pass. With `quality.strict: true` or `--strict`, a skipped gate blocks.

- **Lint**:
  - Go: golangci-lint, falling back to `go vet`.
  - Node: `npm run lint`.
  - Java: PMD, when `maven-pmd-plugin` is in the build (Maven).
  - Python: `ruff check`, the `.venv`'s first.
- **Duplication**, all stacks: jscpd over source code only (lock files, JSON, Markdown, tests and reports are not counted). Threshold `max_duplication_percent`, default 0.
- **Dead code**, Node: Knip.
- **Mutation**, Node: Stryker, when configured. Threshold `min_mutation_score`, default 80. It is slow, so it runs only from `quality.mutation_from` risk up (default `medium`).
- **Migration**, Java: when `migration.java_release` or `forbidden_imports` is set, the release the build declares and every `import` in the Java sources. It needs no tool, so it never skips, and names file and line:

```text
src/main/java/…/Payroll.java:3:
  imports javax.servlet.http.HttpServlet
  (forbidden: javax.servlet)
```

Node tools run with `npx --no-install`: nothing is downloaded during the loop.

## 12. Security audit: `audit`

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

## 13. Browser verification: `e2e`

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

## 14. Hand-over: `deliver`

```bash
specforge deliver 0001
gh pr create --body-file \
  specs/0001-password-reset/PR_BODY.md
```

`deliver` writes three files next to the specification, built only from what SpecForge recorded, never from the agent's word:

- **`DELIVERY.md`**, for people: who approved the specification and the plan, and when. One row per scenario with its tests (file and test names carrying the marker), its commit, its gates (✓ passed · ⚠ skipped · ✗ failed) and notes: its risk and why, its size in lines, its lens review and verification, changes requested in review, rejected attempts. Then the known failures, the review follow-ups, the decisions taken, the questions still open, the lessons, the branch review, the verification, the audit and E2E results, and what is out of scope.
- **`trace.json`**, for tools: the same data, machine-readable.
- **`PR_BODY.md`**, for the pull request: a summary, the scenario table, the decisions, the checks and what is not done. When the repository has a pull request template (`.github/pull_request_template.md` and the other places GitHub looks), its headings are kept and the summary goes under the first one.

**Size and slices.** Each scenario's authored lines are counted from its commit (lock, vendored and golden files excluded). Above `delivery.budget_lines` (400), `DELIVERY.md` and `PR_BODY.md` propose **stacked slices**: consecutive scenarios that fit the budget, each with the command to create its branch. `deliver --slices` also writes `PR_BODY-1.md` … `PR_BODY-n.md`. A scenario larger than the budget is flagged, never cut.

A delivery is honest about gaps: unfinished scenarios, open questions and checks that did not run are listed, and an incomplete delivery says so in its first line. Reviews and SpecForge's own verification questions are shown per scenario, apart from your product decisions.

The loop state lives in `.specforge/`, so run `deliver` where the loop ran. Commit `trace.json` to keep the trace.

## 15. Legacy rewrites: `legacy`, `spec from-legacy`

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

## 16. The destructive-command guard: `guard`

Agents run shell commands. The guard reads each one before it runs and blocks what destroys work:

- recursive deletes (`rm -r`, `find -delete`, `find -exec rm`, `rimraf`, `del-cli`, `trash`), `mkfs`, `shred`, `dd` onto a device;
- git commands that discard work or rewrite history: `reset --hard`, `clean -f`, `push --force` (and `+branch`, `:branch`, `--delete`), `branch -D`, `stash drop`/`clear`, `checkout .`, `restore`, `filter-branch`, `reflog expire`;
- SQL that drops or empties data: `DROP`, `TRUNCATE`, `DELETE` or `UPDATE` without `WHERE` (comments ignored), MongoDB `drop()` and `deleteMany({})`, Redis `FLUSHALL`;
- commands that touch secrets: `.env`, `.ssh/`, `*.pem`, `id_rsa`, `credentials`, `.netrc`.

It sees through `sudo`, `env`, `npx`, variable assignments, `xargs`, `timeout`, `nohup`, `sh -c`, `$( )` and backticks. Deleting `/`, `~`, the project or a system directory is **never** allowed, whatever the settings.

`setup` installs it as your agent's pre-tool hook, in `.claude/settings.json` (Claude Code) or `.gemini/settings.json` (Gemini CLI), keeping the rest of the file; `--no-guard` skips it. When it blocks, the agent sees the reason and has to find another way:

```text
The command didn't run. A SpecForge
guard hook blocked `git reset --hard
HEAD` because it discards uncommitted
work, and it said to find another way
or ask you to run it yourself.
```

```yaml
guard:
  mode: block
  allow:
    - "git push --force-with-lease *"
```

`block` (default) refuses; `confirm` lets Claude Code ask you, and still blocks inside the loop, where nobody can answer; `off` lets everything through but the hard denies. `allow` takes command patterns, `*` matching anything. An unreadable hook request, or an invalid `specforge.yaml`, blocks: the guard never fails open. `specforge guard --selftest` proves it blocks; `doctor` checks it is installed.

### What it reads

Not only the command line. Before a command runs, the guard reads what it runs, and blocks it if that destroys work:

| The agent runs | The guard reads |
| :--- | :--- |
| `sh x.sh`, `./x.sh`, `source x`, `scripts/nuke` | the script, and the scripts it runs in turn; a shebang (`#!/usr/bin/env python3`) names the language |
| `python x.py`, `node x.js`, `ruby`, `perl`, `php`, `go run x.go`, `deno run`, `bun x.ts` | the file, for calls that delete a tree (`shutil.rmtree`, a recursive `fs.rm`, `FileUtils.rm_rf`, `os.RemoveAll`…) and for shell or SQL in its strings (`os.system("rm -rf …")`, `["git", "reset", "--hard"]`, `"DELETE FROM users"`) |
| `python -c "…"`, `node -e "…"`, `ruby -e`, `perl -e`, `php -r`, `bun -e`, `deno eval` | the code given inline, the same way |
| `npm run clean`, `pnpm clean`, `yarn clean`, `bun run clean` | the script in `package.json`, and its `pre` and `post` scripts |
| `make clean` | the target's recipe in the `Makefile`, and its prerequisites' |
| `echo '…' > x.sh && sh x.sh`, a heredoc | the text the command writes into the file it then runs |

A file the same command writes without showing what (`curl -o x.sh … && sh x.sh`, `base64 -d > x.sh`, `cp`) is refused: the agent writes it first and runs it in a second command, where the guard can read it. Scripts are read up to 1 MB; binary files are not read. `guard.allow` matches what the agent typed (`sh ./scripts/clean.sh`), so a script you trust can be let through.

What stays out of reach: what a program imports, a compiled binary, a variable's value at run time. That is what checkpoints are for.

### Checkpoints: `restore`

Before every agent turn, the loop saves a **checkpoint** of your working tree: uncommitted and untracked files included, ignored ones (`node_modules`, `.env`) not. A checkpoint lives under `refs/specforge/checkpoints/`: no branch, commit, index, stash or push carries it, `git status` does not show it, and the newest 50 are kept. Only what changed is hashed, so one takes tens of milliseconds. Scenarios run [side by side](#scenarios-side-by-side) are not checkpointed inside their sandboxes, and when a checkpoint cannot be saved the loop warns once, notes it in the loop state and goes on.

If something gets past the guard, nothing is lost:

```text
$ specforge restore
  20261005T142655.383500121Z · 2026-10-05 14:26:55 · 0001 · scenario 1 (SDD_0001_001) · before GREEN
  restore one with `specforge restore <checkpoint>` (or `latest`); nothing written since is deleted
$ specforge restore latest
  ✓ restored checkpoint 20261005T142655.383500121Z (0001 · scenario 1 (SDD_0001_001) · before GREEN)
  your files just before are checkpoint 20261005T142734.969173443Z: `specforge restore 20261005T142734.969173443Z` undoes this
```

Restoring brings back every file of the checkpoint as it was, deleted or changed since, and **never deletes** a file written after it. Your working tree is checkpointed first, so a restore is undone the same way. Outside a git repository there are no checkpoints.

The guard and the checkpoints protect your repository from mistakes. They are not a sandbox against an agent determined to do harm outside it (another directory, a remote, a database): for that, run the agent in a container. The loop verifies its work with or without them.

## 17. Your files: what to edit

Everything SpecForge uses lives in your repository, next to your code. `0001-slug` stands for each specification's number and name.

### Files you work in

- 📝 **`specs/0001-slug.md`**: the specification. Edit it freely before approving. To change an approved one, run `spec change 0001 "<what to change>"` (or edit it), then `spec approve` again: every scenario keeps its marker.
- 🗺️ **`specs/0001-slug/plan.md`**: the plan. After `plan`, edit what you like, then `plan approve`.
- ❓ **`specs/0001-slug/questions.md`**: questions waiting for you when nobody was at a terminal. Replace `_awaiting an answer_` and run the same command again.
- ⚙️ **`specforge.yaml`**: project settings ([section 19](#19-configuration-reference)).
- 🤖 **`CLAUDE.md`, `GEMINI.md`**: your agent's instructions. Write anything outside the SpecForge block; `setup` refreshes only the block.
- 📚 **`specs/LESSONS.md`**: lessons the agent wrote after a failed attempt. Prune or reword them.
- 🏛 **`docs/legacy/CAPABILITIES.md`**: the legacy capability map.

### Files SpecForge writes

Read them; don't edit them by hand.

- The **seal**, the `<!-- seal: … -->` line at the end of a specification or a plan. Approve again instead.
- The front matter fields **`approved_by`** and **`approved_at`**, written by approval.
- **`specs/0001-slug/approvals.md`**: every approval and what changed in it.
- **`specs/0001-slug/decisions.md`**: every question and answer, reused in later prompts.
- **`specs/0001-slug/interview.jsonl`** and **`change.jsonl`**: the interview and change-request transcripts.
- **`specs/0001-slug/scenarios.json`**: each scenario's marker, kept across changes.
- **`specs/0001-slug/review/SDD_….json`** and **`verify/SDD_….json`**: each scenario's lens review and verification, discarded findings and their reasons included. `review/branch.json` and `verify/feature.json` come from `specforge review` and `specforge verify`.
- **`specs/0001-slug/DELIVERY.md`, `trace.json`, `PR_BODY.md`**: the hand-over, rewritten by each `deliver`.
- **`.claude/settings.json`**, **`.gemini/settings.json`**: the guard hook entry; the rest of each file is yours.
- **`docs/legacy/INVENTORY.md`**: the legacy inventory, rewritten by each scan.
- **`docs/legacy/decisions.md`, `questions.md`**: questions asked while mapping the legacy code.
- **`docs/security/`**: audit reports, owner-only permissions.
- **`docs/e2e/<spec>/`**: E2E reports and screenshots.

### Not committed

- **`.specforge/state/<spec>.json`**: the loop state for `--resume`. `setup` adds `.specforge/` to `.gitignore`.
- **`refs/specforge/checkpoints/`**: the loop's [checkpoints](#checkpoints-restore), local git refs that no push carries.
- The **log file**, in your user cache directory (`~/.cache/specforge/logs/specforge.log` on Linux), rotated, owner-only.

Commit everything else: the specifications, plans, logs and the delivery are the project's history.

## 18. Recipes

**Get back work an agent destroyed.** The loop saved a checkpoint before the turn:

```bash
specforge restore          # list them
specforge restore latest   # bring the newest back
```

**Change an approved specification.** Say what to change, or edit `specs/0001-slug.md` by hand, then approve:

```bash
specforge spec change 0001 "Links expire after 15 minutes"
specforge spec approve 0001
specforge loop 0001
```

Approval prints every scenario that changed, with its marker (`approvals.md` records them all); the loop redoes only those.

**Add or remove a scenario.** The same, and the plan must follow:

```bash
specforge spec change 0001 "Admins can revoke every link of a user"
specforge spec approve 0001
specforge plan 0001
specforge plan approve 0001
specforge loop 0001
```

**Change the approved plan.** Edit `plan.md` and run `specforge plan approve 0001`.

**Answer a question without a terminal.** Open `questions.md`, replace `_awaiting an answer_` with the answer or an option number, and run the same command again (`loop --resume` for the loop).

**Redo one scenario.** `specforge loop 0001 --scenario 3 --from green` (or `red`, `refactor`). The other scenarios keep their state.

**Start the loop over.** `specforge loop 0001 --restart`.

**Continue after Ctrl-C or a stop.** `specforge loop --resume`. With several saved loops, name the specification.

**The loop says a test was tampered with.** Something changed a test file after RED. Revert it (`git checkout -- <file>`), then `loop --resume`.

**A test passed before any code.** SpecForge asks: mark the scenario as already satisfied, ask the agent for a stricter test, or stop.

**Run without reviews or commits.** `--review off --no-commit`, or `review: off` and `commit: false` in `specforge.yaml`.

**Use another agent or model once.** `--agent gemini` or `--model <name>` on `spec interview`, `spec change`, `spec from-legacy`, `plan`, `loop`, `review`, `verify`, `audit`, `e2e` and `legacy map`. `spec clarify` uses the configured agent to apply the answers; without one it only records them.

**Run in CI.** Add `--non-interactive`. Questions go to `questions.md` with exit code 5; every other outcome has its own [exit code](#20-exit-codes). `--json` prints data and errors as JSON.

**See exactly what the agent received.** Add `--trace-io` and read the log file.

**Make a missing tool block.** `--strict`, or `quality.strict: true`. Strict also makes every change high risk.

**Check a new machine.** `specforge doctor`; every ⚠ and ✗ says what to install.

**Accept a file outside the plan.** Answer *Accept them* when asked; it joins the specification's surfaces. To stop being asked, add it to the plan's components and approve the plan again.

**Review only risky scenarios yourself.** `review: risk` in `specforge.yaml`.

**Force the verifier, or turn it off.** `verify: always` or `verify: off`; `specforge verify 0001` runs it once over the whole specification.

**Let one command through the guard.** Add its pattern to `guard.allow`, for example `"git push --force-with-lease origin claude/*"`.

**Run scenarios side by side.** End each component line of the plan with the markers it serves, approve the plan, and set `loop.parallel: 2`.

**Split a big delivery.** `specforge deliver 0001 --slices`, then one pull request per `PR_BODY-n.md`, in order.

**Open a pull request.** `specforge deliver 0001`, then `gh pr create --body-file specs/0001-slug/PR_BODY.md`.

## 19. Configuration reference

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
# scenario: review each one (R2)
# | risk: only medium and high | off
review: scenario
# one commit per finished scenario
commit: true
timeouts:
  agent: 20m
  tests: 10m
quality:
  # true: a skipped gate blocks, and
  # every change is high risk
  strict: false
  max_duplication_percent: 0
  min_mutation_score: 80
  # lowest risk that runs mutation
  mutation_from: medium
# a model per phase, over `model`
models:
  review: claude-opus-5-5
risk:
  # larger changes are high risk
  max_lines: 400
  # lowest tier any change gets
  floor: passive
  # regular expressions; when set
  # they replace the defaults
  high_paths: ["(^|/)ledger/"]
  passive_paths: ["\\.md$"]
# auto (by risk) | off | a list
lenses: auto
# high risk: every lens twice
blind_review: false
# high | always | feature | off
verify: high
# largest project the verifier or a sandbox copies
verify_max_mb: 500
plan:
  # outside the plan: ask | strict | off
  surfaces: ask
delivery:
  # slices are proposed above it
  budget_lines: 400
loop:
  # disjoint scenarios side by side
  parallel: 1
guard:
  # block | confirm | off
  mode: block
  allow: []
# only for a rewrite (section 15)
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
- `--json`: data from `version`, `spec list`, `deliver`, `doctor`, `legacy scan`, `review`, `verify` and `restore` as JSON on stdout, and errors as `{"exit", "title", "cause", "action"}` on stderr.
- `--quiet`: only warnings, errors and data.
- `--verbose`: progress details and the agent's live output.
- `--debug`: debug records in the log.
- `--trace-io`: every prompt and answer in the log.

## 20. Exit codes

- **0**: done.
- **1**: unexpected error or bad usage.
- **2**: a gate said no: tests, quality, a review finding or a correction you stopped, the verifier, a refused file not put back, security findings, E2E pass rate, a legacy document that was not accepted, or the agent is blocked. The guard also blocks a command with exit 2, which your agent sees.
- **3**: the specification or the loop state needs attention: not approved, changed after approval, a plan not approved or naming a removed scenario, lint issues, ambiguous or missing, test tampering, a loop in progress.
- **4**: a tool, browser or setting is missing: agent CLI, test runner, configuration, legacy repository; `doctor` found something required missing.
- **5**: a question awaits your answer (no terminal).
- **130**: interrupted with Ctrl-C; the state saved so far is valid.

Every failure prints what happened, why and what to do next.

## 21. Troubleshooting

- **What did the agent receive and answer?** Re-run with `--trace-io` and read the log file.
- **The agent claims files it did not write.** The attempt is rejected and the agent is told which ones; that is the loop working. Repeated rejections exhaust `max_attempts` (exit 2).
- **"Several stacks detected".** Set `stack:` in `specforge.yaml` or pass `--stack`.
- **The loop says the specification changed.** Revert the edit, or review it and run `specforge spec approve` again; every scenario keeps its marker and unchanged ones keep their progress.
- **A gate shows ⚠ skipped.** Install the tool, or accept the warning; `--strict` makes it block.
- **"The agent changed the legacy code".** Restore the legacy repository (`git checkout .` there) and run the command again; the agent may only read it.
- **A legacy document keeps coming back.** The diagnosis lists each citation that did not resolve, and the agent gets the same list. Citations are relative to the legacy repository.
- **`deliver` says 0 scenarios finished.** The loop state is local: run `deliver` in the clone where the loop ran.
- **A review finding was discarded.** Its proof did not point at a line the change added or modified: the record in `review/SDD_….json` says which reference failed.
- **"A review step failed closed".** The lens, refuter or verifier never returned valid JSON. Run again; if it repeats, try another model with `models.review` or `models.verify`.
- **"A refused file was not put back".** Restore it (`git checkout -- <file>`) and `loop --resume`.
- **The guard blocks a command you need.** Run it yourself, or add its pattern to `guard.allow` (for a script, the command that runs it: `"sh ./scripts/clean.sh"`).
- **"The plan needs attention": it does not match the specification's scenarios.** A scenario was added, or the plan names one that was removed: `specforge plan 0001` revises the plan, then `specforge plan approve 0001`.
- **Tests of removed scenarios are still here.** The loop warns about tests whose name carries a retired marker. Delete them, or keep them on purpose: no scenario runs them any more.
- **`spec interview` says the specification is approved.** Use `specforge spec change 0001 "<what to change>"`.
- **An agent destroyed work anyway.** `specforge restore` lists the checkpoints the loop saved before each agent turn; `specforge restore latest` brings the newest back without deleting anything written since.

## 22. Architecture

SpecForge follows the architecture it asks of your code: the domain is pure, use cases depend on small ports, adapters do the I/O and `cmd/` only wires them.

```text
cmd/            CLI, composition root,
                exit codes
internal/
  domain/       pure, no I/O:
    spec/       Gherkin, seal, lint,
                scenario markers,
                plan surfaces
    risk/       tiers from paths, lines
    review/     diff hunks, findings,
                proof, blind merge
    verification/ the verifier's report
    guard/      destructive commands,
                the scripts they run
    change/     lines per file
    legacy/     inventory, citations,
                migration conformance
    tdd/ stack/ quality/ lessons/
    delivery/ security/ e2e/
  app/          use cases:
    tddloop/    the TDD loop, review,
                verify, parallel batches
    reviewer/   review lenses, refuter
    verifier/   the verifier in a copy
    doctor/     environment checks
    guardhook/  the guard as a hook
    restore/    checkpoints back
    answer/     schema-checked answers
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
                runners, gates, git
                (diffs, checkpoints),
                scratch copies, browser,
                files, logs
  config/ ui/   settings, terminal UI
assets/         prompts, rules,
                standards, templates,
                scaffolds
```

Tests run the real CLI against scripted agents and real `go test`, `git` and Chromium where available: `make test`. Releases are built by GoReleaser when a `v*` tag is pushed.
