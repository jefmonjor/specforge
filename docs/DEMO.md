# A real session

These are real runs of SpecForge with Claude Code: first on a new Go module, shortened where marked `…` and with local paths made relative. Two features: **discount codes** goes from specification to pull request body, and **gift cards** shows the interview. They also show what this README promises: the agent asked instead of guessing, and SpecForge checked each step itself. Section 7 is a SpecForge 5 run: a Java 6 payroll application rewritten on Java 21.

- [1. Set up](#1-set-up)
- [2. The interview (gift cards)](#2-the-interview-gift-cards)
- [3. The plan (discount codes)](#3-the-plan-discount-codes)
- [4. The loop, with your review](#4-the-loop-with-your-review)
- [5. The hand-over](#5-the-hand-over)
- [6. What the runs taught SpecForge](#6-what-the-runs-taught-specforge)
- [7. A legacy rewrite: Java 6 → 21](#7-a-legacy-rewrite-java-6--21)

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

| # | Section · unknowns | The agent asked | The developer answered |
| :---: | :--- | :--- | :--- |
| 1 | 1. Intent · 10 | What problem do gift cards solve for your business, and who buys and who redeems them? | Shoppers buy gift cards online to give as presents; recipients redeem them at checkout to pay all or part of an order. It brings new customers and early cash. |
| 2 | 4. Invariants · 10 | What can a gift card's balance never do? For example, can it go below zero, can it be redeemed for more than the order total, and can a card be used again once its balance is spent? | The balance can never go below zero. A redemption can never take more than the card's balance nor more than the order total. A card with zero balance cannot be redeemed again. Cards have a unique code. |
| 3 | 5. User stories · 9 | When a buyer purchases a gift card, how is its value decided? | One of the fixed amounts 25, 50 or 100 euros. Payment is out of scope. Each card gets a unique 16-character code. Cards never expire. Partial redemption keeps the remaining balance. |
| 4 | 8. Errors · 5 | What should happen when a recipient presents a code that matches no gift card? | Refuse with UNKNOWN_CARD "unknown gift card"; EMPTY_CARD when the balance is zero. No performance requirements. Out of scope: refunds, transfers, physical cards, expiry. |

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
