# A real session

These are real runs of SpecForge with Claude Code: first on a new Go module, shortened where marked `…` and with local paths made relative. Two features: **discount codes** goes from specification to pull request body, and **gift cards** shows the interview. They also show what this README promises: the agent asked instead of guessing, and SpecForge checked each step itself. Section 7 is a SpecForge 5 run: a Java 6 payroll application rewritten on Java 21. Section 8 is SpecForge 6 on a Go payroll module: a branch that already had a failing test, risk tiers, review lenses, the independent verifier, two scenarios side by side and the command guard. Section 9 is SpecForge 6.1, which closes the two gaps section 8 left open: the guard could not see inside a script, and copies were heavy.

- [1. Set up](#1-set-up)
- [2. The interview (gift cards)](#2-the-interview-gift-cards)
- [3. The plan (discount codes)](#3-the-plan-discount-codes)
- [4. The loop, with your review](#4-the-loop-with-your-review)
- [5. The hand-over](#5-the-hand-over)
- [6. What the runs taught SpecForge](#6-what-the-runs-taught-specforge)
- [7. A legacy rewrite: Java 6 → 21](#7-a-legacy-rewrite-java-6--21)
- [8. SpecForge 6: risk, lenses, verifier, parallel, guard](#8-specforge-6-risk-lenses-verifier-parallel-guard)
- [9. SpecForge 6.1: closing the gaps](#9-specforge-61-closing-the-gaps)

## 1. Set up

```text
$ specforge init --agent claude --language en
  ✓ configuration saved in ~/.config/specforge/config.yaml
$ specforge setup

SpecForge · setup
─────────────────
  ✓ created · specforge.yaml
  ✓ created · CLAUDE.md
  ✓ created · .gitignore
  stack: go
```

## 2. The interview (gift cards)

The interview ran without a terminal, as it would in CI: each question went to `specs/0002-gift-cards/questions.md`, the answer was written there, and the command was run again. Each turn the agent first wrote the previous answer into the specification, then asked the single most important next question.

```text
$ specforge spec new "Gift cards"
  ✓ created specs/0002-gift-cards.md
$ specforge spec interview 2

SpecForge · interview · Gift cards
  section 1. Intent · 10 unknown(s) left

✗ A question needs your answer
    What problem do gift cards solve for your business, and who buys and who redeems them?
  → write the answer in place of the placeholder in specs/0002-gift-cards/questions.md and run the same command again …
  (exit 5)
```

1. Section **1. Intent** · 10 unknown(s) left<br>
   *Asked:* What problem do gift cards solve for your business, and who buys and who redeems them?<br>
   *Answered:* Shoppers buy gift cards online to give as presents; recipients redeem them at checkout to pay all or part of an order. It brings new customers and early cash.
2. Section **4. Invariants** · 10 unknown(s) left<br>
   *Asked:* What can a gift card's balance never do? For example, can it go below zero, can it be redeemed for more than the order total, and can a card be used again once its balance is spent?<br>
   *Answered:* The balance can never go below zero. A redemption can never take more than the card's balance nor more than the order total. A card with zero balance cannot be redeemed again. Cards have a unique code.
3. Section **5. User stories** · 9 unknown(s) left<br>
   *Asked:* When a buyer purchases a gift card, how is its value decided?<br>
   *Answered:* One of the fixed amounts 25, 50 or 100 euros. Payment is out of scope. Each card gets a unique 16-character code. Cards never expire. Partial redemption keeps the remaining balance.
4. Section **8. Errors** · 5 unknown(s) left<br>
   *Asked:* What should happen when a recipient presents a code that matches no gift card?<br>
   *Answered:* Refuse with UNKNOWN_CARD "unknown gift card"; EMPTY_CARD when the balance is zero. No performance requirements. Out of scope: refunds, transfers, physical cards, expiry.

```text
$ specforge spec interview 2
  ✓ interview closed: review specs/0002-gift-cards.md, then `specforge spec approve 0002`
$ specforge spec approve 2
  ✓ specs/0002-gift-cards.md approved by Ana QA · seal e8685a7facc4
```

From four answers the specification gained eight invariants (`INV-01` … `INV-08`), each covered by a failure scenario, twelve Gherkin scenarios, a data contract, an error catalogue, what is out of scope and the assumptions with who confirmed them. It passed the lint with no advice. An excerpt:

```gherkin
  Scenario: INV-04 a gift card with a zero balance cannot be redeemed
    Given a gift card with a balance of 0
    When the recipient redeems the gift card for an order
    Then the redemption is refused with error EMPTY_CARD
    And the gift card balance is still 0
```

## 3. The plan (discount codes)

```text
$ specforge plan 1

SpecForge · plan · Discount codes
  ✓ plan written: specs/0001-discount-codes/plan.md
  next: review it (edit it freely), then `specforge plan approve 0001`
```

The plan placed one test per scenario. It also flagged a gap instead of filling it:

```markdown
## Tests per scenario
| Marker | Scenario | Test file | Test name |
|---|---|---|---|
| `SDD_0001_001` | A valid code reduces the total | `internal/discount/discount_test.go` | `TestSDD_0001_001_ValidCodeReducesTotal` |
| `SDD_0001_002` | An unknown code is refused (INV-01) | `internal/discount/discount_test.go` | `TestSDD_0001_002_UnknownCodeIsRefused` |

## Risks
- Rounding: 10% of a total not divisible by 10 (e.g. 10005 cents) is not specified; the scenarios only use 10000. Left out of scope until the spec says how to round.
```

## 4. The loop, with your review

```text
$ specforge plan approve 1
$ specforge loop 1

SpecForge · Red → Green → Refactor · 0001 · Discount codes
  stack go · agent claude · 2 scenario(s)

Scenario 1/2 · RED · A valid code reduces the total
  ✓ RED accepted
Scenario 1/2 · GREEN · A valid code reduces the total
  ✓ GREEN accepted
Scenario 1/2 · REFACTOR · A valid code reduces the total
  ✓ lint · passed · golangci-lint: no issues
  ⚠ duplication · skipped · tool not installed: tool not found: jscpd (npm install -g jscpd)
  ✓ REFACTOR accepted
  ? Review scenario 1 (A valid code reduces the total): do you accept it? Type what should change to send it back to GREEN.
    SDD_0001_001: internal/discount/discount.go, internal/discount/discount_test.go · gates: lint=passed duplication=skipped
    1) Accept
    2) Back to RED: the test does not express the scenario
  > 1
  ✓ committed 1673c15

Scenario 2/2 · RED · An unknown code is refused (INV-01)
  ? The test for scenario 2 passed before any implementation. Is this behaviour already implemented?
    1) Yes: mark the scenario as already satisfied
    2) No: ask the agent for a stricter test
    3) Stop the loop
  > 1
  ✓ scenario 2 marked as already satisfied by the developer
  ✓ committed 684724b

2 scenario(s) done: 1 through RED → GREEN → REFACTOR, 1 already satisfied
```

In the real session this loop ran twice: it stopped once at scenario 2 and was resumed with `specforge loop --resume`, which is why the delivery below counts rejected attempts for that scenario (see section 6). The second scenario's test passed before any new code, because GREEN for the first one had followed the plan's whole approach. SpecForge did not count that as a RED. It asked, and the answer was recorded.

```text
$ git log --oneline
684724b test(SDD_0001_002): An unknown code is refused (INV-01)
1673c15 feat(SDD_0001_001): A valid code reduces the total
8afb0b2 plan
bc37ae7 spec
```

## 5. The hand-over

```text
$ specforge deliver 1
  ✓ delivery written: 2/2 scenario(s) finished
specs/0001-discount-codes/DELIVERY.md
specs/0001-discount-codes/trace.json
specs/0001-discount-codes/PR_BODY.md
```

```markdown
# Delivery · 0001 Discount codes

**Specification** `specs/0001-discount-codes.md` · approved by Ana QA on 2026-10-04 · `sha256-v1:9394d6452b3a…`

**Plan** `specs/0001-discount-codes/plan.md` · approved by Ana QA on 2026-10-04 · `sha256-v1:b57f7f8eda49…`

**Scenarios** 2/2 finished · 1 through RED → GREEN → REFACTOR · 1 already satisfied

| # | Scenario | Tests | Commit | Gates | Notes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | A valid code reduces the total | `internal/discount/discount_test.go` · `TestSDD_0001_001_ValidCodeReducesTotal` | `1673c15` | lint ✓ · duplication ⚠ | — |
| 2 | An unknown code is refused (INV-01) | `internal/discount/discount_test.go` · `TestSDD_0001_002_UnknownCodeIsRefused` | `684724b` | — | already satisfied; 3 rejected attempt(s) |
```

## 6. What the runs taught SpecForge

Every live run found something the unit tests had not, and each became a fix with its own regression test:

| Run | What happened | Fix |
| :--- | :--- | :--- |
| First loop, no terminal | The agent wrote the test and then asked a question; on resume, that test no longer counted as written in the step. | A pending question saves the step with its baseline; `--resume` continues the same step and answers from `questions.md` or the terminal. |
| Same | The resumed prompt did not carry the answer. | Answers go into the prompt explicitly, with the decisions log as it is after answering. |
| Loop with review | "Accept" typed at a question with fixed options stopped the loop. | Fixed-choice questions accept only their options and ask again. |
| Same | A scenario marked as already satisfied left its new test uncommitted. | It is committed as `test(SDD_…)`. |
| Resume after a stop | RED kept rejecting "no test file changed" because the test already existed. | RED runs an existing test with the marker before calling the agent. |
| `deliver` | A message key was missing in both languages. | A test now checks that every key used in the code exists. |
| Interview | The pending-question hint pointed to `loop --resume` for every command. | The hint says to run the same command again. |

## 7. A legacy rewrite: Java 6 → 21

The legacy system is a small Java 6 payroll application, the way many still run: an Ant build, a servlet, a JDBC DAO returning a raw `Vector`, a `Hashtable` passed to a JSP, Log4j 1, `java.util.Calendar`, money in `double`, and one JUnit 3 test. The rewrite is a new project next to it.

```text
$ specforge setup --new java --legacy ../legacy-payroll
  ✓ created · pom.xml
  ✓ created · src/test/java/com/example/payroll/ArchitectureTest.java
  …
  stack: maven
$ specforge legacy scan
  ✓ inventory written: docs/legacy/INVENTORY.md
  build ant · Java 1.6 · 4 source file(s), 1 test file(s), 166 line(s)
  found: Servlet API (javax.servlet), JSP views, Plain JDBC, Log4j 1.x, JUnit 3, Vector / Hashtable / Enumeration, java.util.Date / Calendar / SimpleDateFormat
$ specforge legacy map
  … claude is working (LEGACY)
  ✓ capability map written, every cited source checked: docs/legacy/CAPABILITIES.md
```

The map named five capabilities in migration order (roster, withholding, seniority bonus, net pay, payroll run), each rule with its source, and listed what it could not explain instead of explaining it. One of them is a real bug in the legacy code that nobody had asked about:

```markdown
- **Unclear**:
  - The servlet's data access object is never assigned in this code (`src/com/acme/payroll/web/PayrollServlet.java:18`, used at line 23); how it is supplied is unknown, and without it the run fails with error 500.
```

```text
$ specforge spec from-legacy "Net pay calculation"
  ✓ created specs/0001-net-pay-calculation.md
  ✓ specification drafted from the legacy code, every cited source checked: specs/0001-net-pay-calculation.md
  6 open question(s) about the legacy behaviour: answer them with `specforge spec clarify 0001`, then approve
```

The first draft took in the payroll run too (16 scenarios, 16 questions). That is another capability, so the prompt now requires one capability per specification, and the second draft kept 6 scenarios and sent the rest to *Out of scope*. Every rule cites its line, and SpecForge opened each one:

```markdown
## 13. Legacy sources

- INV-02: `src/com/acme/payroll/PayrollCalculator.java:58`
- INV-03: `src/com/acme/payroll/PayrollCalculator.java:54-56`
- INV-04: `src/com/acme/payroll/PayrollCalculator.java:61`
- Scenario "The net pay is truncated to whole cents": `src/com/acme/payroll/PayrollCalculator.java:61`
```

The six questions were the oddities a rewrite must not settle by itself. The developer answered them, and each answer went into the specification in place of its question:

| The agent found | The developer decided |
| :--- | :--- |
| Withholding excludes the seniority bonus; the comment gives no reason. | Keep it: HR pays the bonus gross. |
| Money is `double`, so an exact cent can fall one cent short before truncation. | Exact decimals, truncated down to cents; do not reproduce floating-point errors. |
| The log shows the net pay before truncation, the result is truncated. | Log the truncated amount, the one actually paid. |
| Only one legacy test exists, and it does not cover the net pay. | Validated against the March 2026 payroll. |

The plan chose `BigDecimal` with `RoundingMode.DOWN`, a `Money` record and a log port, and flagged building decimals from strings in the tests. In the loop, SpecForge's gates ran on every REFACTOR: PMD rejected the first version and the agent fixed it, the migration gate checked the declared release and the imports, and ArchUnit ran with the suite. Two scenarios were already satisfied by the first implementation; SpecForge asked instead of counting a test that passed before any code as a RED.

```text
Scenario 1/6 · REFACTOR · Net pay adds the seniority bonus and subtracts the withholding
  ✗ lint · failed · PMD found 1 violation(s)
  ✓ migration · passed · Java 21 declared, no forbidden import
  ✓ lint · passed · PMD: no violations
  ✓ migration · passed · Java 21 declared, no forbidden import
  ✓ REFACTOR accepted
  ✓ committed fa8a360
```

```java
public record Money(BigDecimal amount) {
    public Money truncated() {
        return new Money(amount.setScale(2, RoundingMode.DOWN));
    }
    …
}
```

```text
6 scenario(s) done: 4 through RED → GREEN → REFACTOR, 2 already satisfied
$ mvn test
Tests run: 4, Failures: 0 … in com.example.payroll.ArchitectureTest
Tests run: 6, Failures: 0 … in com.example.payroll.application.CalculateNetPayTest
$ specforge deliver 0001
  ✓ delivery written: 6/6 scenario(s) finished
```

The legacy repository had no change at the end (`git status` clean): SpecForge hashed it around every agent turn.

The same release was run on the other scaffolds. Python (`setup --new python`): two scenarios, `Decimal` with `ROUND_HALF_UP`, Ruff from the project's `.venv`. React (`setup --new react`): two scenarios with ESLint and `tsc`, Knip, jscpd and Stryker (mutation score 100 %) on every REFACTOR. The first run found a bug in SpecForge itself: the duplication gate counted `package-lock.json` and stopped the loop with exit 2. It now counts source code only, and `loop --resume` finished the feature.

## 8. SpecForge 6: risk, lenses, verifier, parallel, guard

A Go payroll module with one test that already failed (`TestLegacyRounding`), and a specification with three scenarios: net is gross minus tax, deductions never make the net negative (INV-01), and a signed payslip link verifies only for its employee (INV-02). The project asked for the verifier on high-risk scenarios and two scenarios at a time:

```yaml
review: off
verify: high
loop:
  parallel: 2
```

`doctor` checked the machine first; what only some commands need is a warning, not a stop:

```text
$ specforge doctor
  ✓ agent · claude · 2.1.289 (Claude Code)
  ✓ tests · /usr/local/go/bin/go
  ✓ gate lint · golangci-lint
  ⚠ gate duplication · jscpd not found
      → npm install -g jscpd (or add it to devDependencies)
  ⚠ browser (e2e) · no Chrome, Chromium or Edge found
  ✓ guard hook · .claude/settings.json runs `specforge guard`
ready: everything required is installed
```

The plan put each component on its scenarios' lines; that is what lets SpecForge know which scenarios touch disjoint files:

```markdown
- `internal/pay/net.go`: computes the net from gross, tax rate
  and deductions … (new) · SDD_0001_001 · SDD_0001_002
- `internal/auth/payslip_link.go`: signs an employee id with the
  company secret … (new) · SDD_0001_003
```

### The loop

The failing test was found before any scenario and never blocked one. Scenario 1 was ordinary code: one lens. Scenarios 2 and 3 share no file, so they ran side by side, each in its own git sandbox; scenario 3 lives under `internal/auth/`, a sensitive path, so it got the four lenses and the verifier:

```text
$ specforge loop 0001 --non-interactive
  … running go test ./...
  ⚠ 1 test(s) already fail on this branch and will not block (…):
      example.com/payroll/pay › TestLegacyRounding

Scenario 1/3 · REFACTOR · Net is the gross minus the tax
  risk medium · code changed: 35 line(s) in 2 file(s)
  ✓ lint · passed · golangci-lint: no issues
  ✓ REFACTOR accepted
Scenario 1/3 · REVIEW · Net is the gross minus the tax
  … review: reliability lens (read only)
  ✓ review: 1 lens(es) · 1 reported · 0 corrected · 0 follow-up(s) · 0 discarded
  ✓ committed …

side by side, each in its own sandbox: SDD_0001_002 · SDD_0001_003
…
Scenario 3/3 · REFACTOR · A payslip link verifies only for its employee (INV-02)
  ⚠ risk high · `internal/auth/payslip_link.go` is a sensitive path · …
Scenario 3/3 · REVIEW · A payslip link verifies only for its employee (INV-02)
  … review: risk lens (read only)
  … review: reliability lens (read only)
  … review: readability lens (read only)
  … review: resilience lens (read only)
  ✓ review: 4 lens(es) · 2 reported · 0 corrected · 0 follow-up(s) · 0 discarded
  … verify: 3 requirement(s), in a copy of the project
```

The agent also asked for more scrutiny itself, and the request is a decision on record:

```markdown
### 2026-10-05 11:33 · RISK · scenario 3 (`SDD_0001_003`)

- **Question:** The agent asked for more scrutiny of scenario 3
- **Answer:** medium · The code signs and verifies links with a
  company secret, so it handles credentials and access to employee
  payslips.
```

The lenses only report what they can tie to a changed line. On scenario 1 the reliability lens saw that `Net` did not clamp to zero yet (scenario 2's job) and that `ErrInvalidRate` was declared but never returned; on scenario 3 the risk lens noted that an empty company secret would still sign links:

> **RSK-001** · risk lens · SUGGESTION · `internal/auth/payslip_link.go:10` · inferential, introduced by this change
>
> SignPayslipLink and VerifyPayslipLink accept an empty company secret and still produce and accept valid HMACs, so a misconfigured secret would make payslip links forgeable. …

### The verifier, and what it taught SpecForge

The first verifier run had blamed scenario 3 for INV-01, an invariant of another scenario, with a "command" that was a code reading. SpecForge now asks the verifier only about the scenario's own invariants and marker, and re-runs every blocker's command, refusing one it cannot reproduce. The next run said "unverified" for everything: in headless mode its `go run` waited for an approval that never comes. The verifier is the one agent that works in a disposable copy, so it is now the one allowed to run commands, and on the finished feature it checked every requirement with a probe of its own:

```text
$ specforge verify 0001
  … verify: 5 requirement(s), in a copy of the project
  ✓ verify: 5 met · 0 unmet · 0 unverified
  report: specs/0001-net-pay/verify/feature.json
```

```json
{
  "id": "INV-02",
  "status": "met",
  "command": "go run ./cmd/probe",
  "observed": "2e238a90…c2fc728 true false false false"
}
```

Its advisories found what no scenario covered, the same gap the reliability lens had seen: the specification names an `INVALID_RATE` error that no code returns. No scenario requires it, so it does not block; it is in the report for the developer:

> - INVALID_RATE (spec section 8) is never returned: `pay.Net(100000, 101, 0)` gives 0 with a nil error and `pay.Net(100000, -5, 0)` gives 105000 with a nil error; ErrInvalidRate is declared but unused.
> - gross\*taxRate can overflow int64 for very large gross values: `pay.Net(math.MaxInt64, 20, 0)` returns 9223372036854775807 instead of about 80 percent of it.

The verifier also proposed a regression test for INV-02. SpecForge asked, the developer answered *Add them*, the whole suite still passed and the test went into the scenario's commit.

### The hand-over

```text
$ specforge deliver 0001
  ✓ delivery written: 3/3 scenario(s) finished
```

```markdown
**Scenarios** 3/3 finished · 3 through RED → GREEN → REFACTOR

329 authored line(s) · budget 400

## Known failures (already failing before the loop; …)

- `example.com/payroll/pay › TestLegacyRounding`

## Checks

- Independent verification of the specification: 5 met · 0 unmet ·
  0 unverified (probes derived from the specification, run in a copy).
```

A documentation-only branch reviewed outside the loop needs no lens:

```text
$ specforge review --base main
  risk passive · documentation only
  ✓ no lens needed (risk passive)
```

### The guard

With the hook installed by `setup`, Claude Code was asked to run `git reset --hard HEAD` in a repository with uncommitted work. The hook stopped it before it ran, and the work was still there:

```text
The command didn't run. A SpecForge
guard hook blocked `git reset --hard
HEAD` because it discards uncommitted
work, and it said to find another way
or ask you to run it yourself.
$ git status --short
 M keep.go
```

### What this run fixed

| What happened | Fix |
| :--- | :--- |
| The verifier blamed a scenario for another scenario's invariant. | It checks the scenario's own invariants and marker; the whole feature with `specforge verify`. |
| A blocker's "command" was a code reading. | SpecForge runs every blocker's command and requires its observed output. |
| The verifier could not run its probes in headless mode. | Only the verifier, in its disposable copy, may run commands. |
| A scenario that asked a question in its sandbox pointed at the sandbox's `questions.md`. | It runs again in the project, where the question and its answer belong. |
| Its escalation was then recorded twice. | The escalations of a discarded attempt stay behind. |
| The scenario integrated from a sandbox showed no gates in `DELIVERY.md`. | Its RED, GREEN and REFACTOR join the project's record. |
| The review and verification records were left out of the commit. | They are committed with the scenario. |
| A stop on the agent's usage limit showed an empty reason. | The reason is taken from the agent's output when its error stream is empty. |
| `review --base` with a branch that does not exist printed an empty review before the error. | Only a review that ran is reported. |

A quality review of the branch afterwards (reuse, simplification, efficiency, and whether each fix sits at the right depth) found three more problems in the same path the run used, where scenarios come back from their sandboxes:

| What it found | Fix |
| :--- | :--- |
| When two scenarios of a batch both answered a question, the second to come back lost its answer. | Each sandbox's logs are appended from where the batch started. |
| SpecForge's own logs counted as a scenario's files, so two scenarios that logged a decision "collided". | Only the scenario's work files are compared and copied; the logs come back by appending. |
| The developer's review of an integrated scenario showed the previous scenario's gates. | The scenario is adopted whole from its sandbox, history included. |

Each has a regression test that fails without the fix. The same pass shared what had been written twice (questions with fixed options, the verifier's request, a schema loader, record paths), and made a small edit to a large file cheap to measure.

## 9. SpecForge 6.1: closing the gaps

SpecForge 6 left two known gaps: the guard read only the command line, so a script the agent wrote went unseen, and every copy wrote the whole project, sometimes several times over.

### The guard reads what a command runs

A repository with uncommitted work in `keep.go`, an untracked `build/`, a `package.json` whose `clean` script is `rimraf build`, and the hook installed by `setup`. Claude Code was asked to wipe it three ways the 6.0 guard could not see: a script it writes and runs, `python -c`, and an npm script.

The first run got one through. Claude wrote `reset.sh` and ran it **in the same command**, so the file did not exist yet when the hook read it:

```text
1. reset.sh: ran.
   Output: HEAD is now at 618c223 init
2. python3 -c "import shutil; shutil.rmtree('build')": blocked.
3. npm run clean: blocked.
```

The uncommitted work in `keep.go` was gone. The guard now also reads what a command line writes before it runs it: the text of `echo`, `printf` or a heredoc is inspected as the file, and a file the line writes without showing what (`curl -o`, `cp`, `base64 -d >`) is refused until the agent writes it in one step and runs it in another. The same request, again:

```text
All three commands were blocked by the SpecForge guard hook before they ran.

1. SpecForge guard: git reset --hard discards uncommitted work
   (run by `sh reset.sh`) (git: `sh reset.sh`).
2. SpecForge guard: shutil.rmtree deletes a directory tree
   (fs: `python3 -c "import shutil; shutil.rmtree('build')"`).
3. SpecForge guard: rimraf deletes directory trees
   (run by `npm run clean`) (fs: `npm run clean`).
```

`keep.go` and `build/` were untouched. A fuzz test then sent 260,000 generated command lines through the guard: a line made only of a redirection (`> out.txt`) made it panic. Fixed before release.

### Checkpoints for what still gets past

A compiled program is out of any guard's reach. On the payroll module of section 8, with a note and a change not yet committed, the loop ran one step and saved a checkpoint before the agent's turn. Then a compiled binary, which the guard lets through because it cannot read it, ran `git reset --hard` and `git clean -fd`:

```text
$ ./wipe
HEAD is now at 68c3c45 deliver 0001
Removing NOTES.md
Removing docs/
$ specforge restore
  20261005T142655.383500121Z · 2026-10-05 14:26:55 · 0001 · scenario 1 (SDD_0001_001) · before GREEN
$ specforge restore latest
  ✓ restored checkpoint 20261005T142655.383500121Z (0001 · scenario 1 (SDD_0001_001) · before GREEN)
  your files just before are checkpoint 20261005T142734.969173443Z: `specforge restore 20261005T142734.969173443Z` undoes this
$ git status --short
 M internal/pay/net.go
?? NOTES.md
?? docs/
```

Everything came back: the uncommitted change, the untracked note and directory. No branch, stash or commit was created. On a 15,000-file repository a checkpoint takes about 65 ms.

### Copies that write each byte once

Measured on two repositories, as bytes written (the disk of the test machine was too noisy for timings):

| | 15,000 files, 177 MB | 500 incompressible files, 49 MB |
| :--- | ---: | ---: |
| Verifier copy and base, 6.0 | 477 MB | 143 MB |
| Verifier copy and base, 6.1 | 310 MB | 95 MB |
| Sandbox, 6.0 | 157 MB | 96 MB |
| Sandbox, 6.1 | 157 MB | 48 MB |

The base is checked out straight from git instead of through an archive written and extracted, and a sandbox borrows the project's git objects instead of storing them again. Cloning the repository was tried first and dropped: it wrote as much and cost more CPU. On APFS (macOS), Btrfs and XFS, files are cloned copy-on-write, so a copy writes almost nothing; CI checks that on macOS. The size limit is now checked before anything is written. Its test also found a bug: on a file system without copy-on-write, a failed clone attempt left the file with mode 0600, and an executable script lost its `x` bit.

