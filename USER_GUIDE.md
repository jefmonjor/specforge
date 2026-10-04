# SpecForge user guide

SpecForge drives a coding agent ([Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Gemini CLI](https://github.com/google-gemini/gemini-cli)) through a test-first loop against a specification you approved, and checks every step itself instead of trusting the agent's word. This guide covers every command, every file SpecForge writes and every exit code.

- [1. How it works](#1-how-it-works)
- [2. Install](#2-install)
- [3. Configure once: `init`](#3-configure-once-init)
- [4. Prepare a repository: `setup`](#4-prepare-a-repository-setup)
- [5. Specifications: `spec`](#5-specifications-spec)
- [6. The loop: `loop`](#6-the-loop-loop)
- [7. Questions instead of guesses](#7-questions-instead-of-guesses)
- [8. Quality gates](#8-quality-gates)
- [9. Security audit: `audit`](#9-security-audit-audit)
- [10. Browser verification: `e2e`](#10-browser-verification-e2e)
- [11. Configuration reference](#11-configuration-reference)
- [12. Files SpecForge writes](#12-files-specforge-writes)
- [13. Exit codes](#13-exit-codes)
- [14. Troubleshooting](#14-troubleshooting)
- [15. Architecture](#15-architecture)

## 1. How it works

```text
spec new ─► interview / edit ─► lint ─► approve (R0: sealed) ─► loop ─► audit ─► e2e
                                                                │
                              per scenario:  RED ─► GREEN ─► REFACTOR
```

- A **specification** (`specs/NNNN-slug.md`) says *what* to build and *why*, with the acceptance criteria as Gherkin scenarios.
- **Approval** is a human gate: it refuses while a `TODO` or an open question is left, records who approved and when, and seals the content with a SHA-256 hash. The loop only runs an approved, unchanged specification.
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

## 5. Specifications: `spec`

```bash
specforge spec new "Password reset"   # specs/0001-password-reset.md from the template
specforge spec interview 0001         # complete it in a conversation with your agent
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

`spec interview` opens your agent in your terminal with instructions to complete the file one question at a time, write each answer into the right section and record what you cannot answer yet as an open question. It never approves or seals. It needs an interactive terminal.

### Approve

`spec approve` lints, then records the approver (`--by`, else `git config user.name`, else a question) and the UTC time in the front matter, and appends the seal:

```markdown
<!-- seal: sha256-v1:9394d6452b3a… -->
```

The hash ignores line endings and trailing spaces, so a Windows checkout verifies the same. Editing an approved specification makes it `changed`: the loop refuses it until you revert the edit or approve the new version on purpose. Specifications sealed by SpecForge 3 (`sha256:`) still verify.

## 6. The loop: `loop`

```bash
specforge loop 0001             # start
specforge loop --resume         # continue after a stop, a question or Ctrl-C
specforge loop 0001 --restart   # discard the saved state and start over
```

For each scenario, in order:

| Phase | The agent | SpecForge accepts it only when |
| :--- | :--- | :--- |
| **RED** | Writes the test for this scenario, named with its marker (`SDD_0001_003`), plus the stubs it needs to compile. | A test file carrying the marker changed; the files the agent lists really changed; the filtered test run compiles, runs at least one test and fails. A test that passes before any implementation is reported to you: either the behaviour exists already (mark the scenario satisfied) or the test is wrong. |
| **GREEN** | Writes the minimum code. | The test files are byte-for-byte as RED left them (else the loop stops: *test tampering*); the marker's tests pass. Failures are fed back, up to `max_attempts` (default 3). |
| **REFACTOR** | Only called when something blocks: fixes the full suite or the gate findings without touching tests. | The whole suite passes and no gate blocks. |

The loop state is saved after every step in `.specforge/state/<spec>.json` (written atomically). `--resume` re-reads the specification and checks the seal first. If the specification was approved again with changes, the scenarios whose text did not change keep their progress and the rest are redone. A finished loop is reported, not redone; `--restart` runs it again on purpose.

Test commands used for the marker filter: `go test -json -run`, Maven `-Dtest`, Gradle `--tests`, `vitest run -t`, `jest -t`, `pytest -k`. Results are read from the runner's machine-readable report (test2json, Surefire/JUnit XML, Vitest/Jest JSON, pytest JUnit XML), so "did not compile", "nothing ran" and "an assertion failed" are told apart. With plain `npm test` the filter is not exact and SpecForge asks you to confirm the RED.

## 7. Questions instead of guesses

Every prompt ends with a response contract. The agent answers `done` (with the files it wrote), `needs_clarification` (a question, optional choices, context) or `blocked` (a reason and a suggested action). Prose without the contract gets one retry, then the step stops: prose never counts as success.

- **At a terminal**, the question is shown with numbered options; your answer is appended to `specs/NNNN-slug/decisions.md` (date, phase, scenario) and sent back to the agent. The decisions log is part of every later prompt, so nothing is asked twice.
- **Without a terminal** (CI, `--non-interactive`), the question is written to `specs/NNNN-slug/questions.md` and the command exits with code **5**. Answer it in either of two ways:
  - write the answer (or an option number) in place of `_awaiting an answer_` in `questions.md`, then run `specforge loop --resume` anywhere, CI included;
  - or run `specforge loop --resume` at a terminal and answer there.

  The resumed step starts with the answer and does not start over: the files the agent had already written before asking still count as written in that turn. While the question has no answer, `--resume` exits with code 5 again without calling the agent.
- **Blocked** stops with exit code 2 and the agent's suggested action.

The managed block in `CLAUDE.md`/`GEMINI.md` carries the same rule: if anything needed is not in the specification, the decisions log, the code or the prompt, do not assume it.

## 8. Quality gates

REFACTOR runs the gates that apply to the stack. Each reads its tool's machine-readable report and compares a number with a threshold. A tool that is missing or crashes is **skipped**, shown with ⚠, never as a pass; with `quality.strict: true` (or `--strict`) a skipped gate blocks. Node tools run with `npx --no-install`: nothing is downloaded during the loop.

| Gate | Go | Node | Java | Python |
| :--- | :--- | :--- | :--- | :--- |
| Lint | `golangci-lint` (falls back to `go vet` when absent) | `npm run lint` | — | `ruff check` |
| Duplication (`max_duplication_percent`, default 0) | jscpd | jscpd | jscpd | jscpd |
| Dead code | — | Knip | — | — |
| Mutation (`min_mutation_score`, default 80) | — | Stryker, when configured | — | — |

## 9. Security audit: `audit`

```bash
specforge audit                    # your changes since origin/main, origin/master, main or master
specforge audit --base v1.2.0
specforge audit --full --fail-on medium
```

Three passes run with your agent over each chunk of code: **reconnaissance**, a red-team **hunter** that proposes findings with file, line and evidence, and a blue-team **validator** that confirms or rejects each one. Every answer must match the embedded JSON schema; an invalid answer gets one retry and then the audit stops with an error, never with an empty passing report (*fail-closed*). Findings the validator cannot settle at or above the threshold are questions for you. A confirmed finding at or above `--fail-on` (default `high`) exits with code 2.

The base ref is validated as a commit and passed after `--end-of-options`, so it can never be read as a git option. Reports go to `docs/security/` (`REPORT.md`, `report.json`) with owner-only permissions.

## 10. Browser verification: `e2e`

```bash
specforge e2e 0001 --url http://localhost:3000
specforge e2e 0001 --url http://localhost:3000 --scenario 2 --headed
```

`e2e` drives Chrome, Chromium or Edge (found on your system, or `CHROME_PATH`) through [chromedp](https://github.com/chromedp/chromedp), scenario by scenario. The agent sees a compact view of the page (URL, title, interactive elements with unique selectors, visible text) and answers with one typed action at a time: `navigate`, `click`, `type`, `select`, `press`, `scroll`, `wait`, `assert` or `fail`. Before running an action SpecForge checks that the selector exists on the page and that navigation stays on the same origin. An `assert` names a `Then` and the evidence (text, selector or URL); SpecForge verifies the evidence itself. The page is untrusted input: its text never becomes an instruction.

Each step saves a screenshot. Results go to `docs/e2e/<spec>/` (`report.json`, `REPORT.md`, `scenario-NN/step-MM.png`). The command exits with code 2 when the pass rate is below `--min-pass-rate` (default 100). `--insecure` accepts invalid TLS certificates for local test servers; it is off by default. Only approved specifications run.

## 11. Configuration reference

Values resolve in this order: command-line flag, then `specforge.yaml`, then your user configuration, then the default.

```yaml
# specforge.yaml
language: en            # es | en
stack: go               # go | maven | gradle | node | python (detected when unset)
agent: claude           # usually each developer's choice
model: ""               # passed to the agent; empty uses its default
max_attempts: 3         # GREEN attempts per scenario
timeouts:
  agent: 20m
  tests: 10m
quality:
  strict: false
  max_duplication_percent: 0
  min_mutation_score: 80
```

Unknown keys are an error, so a typo never silently leaves a default in place.

Global flags: `--verbose` (progress details and the agent's live output), `--debug` (debug records in the log), `--trace-io` (every prompt and answer in the log), `--quiet` (only warnings, errors and data), `--non-interactive` (never ask: write questions to a file and exit 5).

## 12. Files SpecForge writes

| Path | Committed | Content |
| :--- | :---: | :--- |
| `specforge.yaml` | yes | Project settings. |
| `CLAUDE.md`, `GEMINI.md` | yes | Your content plus the managed block. |
| `specs/NNNN-slug.md` | yes | The specification, its front matter and its seal. |
| `specs/NNNN-slug/decisions.md` | yes | Every question the agent asked and your answer. |
| `specs/NNNN-slug/questions.md` | yes | Questions asked when nobody was at the terminal. |
| `docs/security/` | your choice | Audit reports (owner-only permissions). |
| `docs/e2e/<spec>/` | your choice | E2E reports and screenshots. |
| `.specforge/state/<spec>.json` | no | Loop state for `--resume`. |

The log file lives in your user cache directory (`~/.cache/specforge/logs/specforge.log` on Linux), rotated, with owner-only permissions.

## 13. Exit codes

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

## 14. Troubleshooting

- **What did the agent receive and answer?** Re-run with `--trace-io` and read the log file.
- **The agent claims files it did not write.** The attempt is rejected and the agent is told which ones; that is the loop working. Repeated rejections exhaust `max_attempts` (exit 2).
- **"Several stacks detected".** Set `stack:` in `specforge.yaml` or pass `--stack`.
- **The loop says the specification changed.** Revert the edit, or review it and run `specforge spec approve` again; unchanged scenarios keep their progress.
- **A gate shows ⚠ skipped.** Install the tool, or accept the warning; `--strict` makes it block.

## 15. Architecture

SpecForge follows the architecture it asks of your code: the domain is pure, use cases depend on small ports, adapters do the I/O and `cmd/` only wires them.

```text
cmd/                      CLI and composition root (cobra); signals → context; exit codes
internal/
  domain/                 pure: no I/O
    spec/                 Gherkin parsing (official parser), seal, lint, front matter, open questions
    tdd/                  loop state, test outcomes, typed errors
    stack/                stack detection, test commands, test-file rules
    quality/ security/ e2e/
  app/                    use cases
    tddloop/              Red → Green → Refactor with verification
    specs/ setup/         specification lifecycle, repository setup
    audit/ e2erun/        security audit, browser verification
    clarify/ protocol/    questions to the developer, the agent response contract
    prompts/ layout/      prompt templates, project paths
  ports/                  interfaces the use cases depend on
  adapters/               agent CLI, process runner, test runners, gates, git, browser, files, logging
  config/ ui/             settings resolution; terminal output and diagnoses
assets/                   embedded prompts, rules, standards, audit method, templates (es/en)
```

Tests run the real CLI against scripted agents and real `go test`, `git` and Chromium where available: `make test`.
