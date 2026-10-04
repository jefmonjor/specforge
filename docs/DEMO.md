# A real session

These are real runs of SpecForge 4 with Claude Code on a new Go module, shortened where marked `…` and with local paths made relative. Two features: **discount codes** goes from specification to pull request body, and **gift cards** shows the interview. They also show what this README promises: the agent asked instead of guessing, and SpecForge checked each step itself.

- [1. Set up](#1-set-up)
- [2. The interview (gift cards)](#2-the-interview-gift-cards)
- [3. The plan (discount codes)](#3-the-plan-discount-codes)
- [4. The loop, with your review](#4-the-loop-with-your-review)
- [5. The hand-over](#5-the-hand-over)
- [6. What the runs taught SpecForge](#6-what-the-runs-taught-specforge)

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
