---
id: ""
title: ""
status: draft
created: "{{.Date}}"
approved_by: ""
approved_at: ""
---

# {{.ID}} · {{.Title}}

<!--
This specification says WHAT the software must do and WHY, in the business's
own words. HOW belongs in the plan. Replace every TODO: `specforge spec approve`
refuses while one is left, and so does any open question in section 12.
-->

## 1. Intent

TODO: the problem this solves and the value it brings, in two or three sentences.

## 2. Actors

- **TODO role**: what this actor can and cannot do.

## 3. Ubiquitous language

- **TODO term**: its exact meaning. Every term used in the scenarios is defined here.

## 4. Invariants

- **INV-01**: TODO a rule the system must never break, in any state.

## 5. User stories

- **US-1**: As a TODO, I want TODO so that TODO.

## 6. Scenarios

<!-- One behaviour per scenario: one When, at least one Then, a unique title,
     no UI or technology words. Cover every invariant with a failure scenario. -->

```gherkin
Feature: {{.Title}}

  Scenario: TODO the main success path
    Given TODO
    When TODO
    Then TODO

  Scenario: TODO a handled failure
    Given TODO
    When TODO
    Then TODO
```

## 7. Data contracts

| Field | Type | Required | Rule |
| :--- | :--- | :---: | :--- |
| TODO | | | |

## 8. Errors

| Code | When | Message |
| :--- | :--- | :--- |
| TODO | | |

## 9. Non-functional requirements

- TODO: a number (latency, volume, retention…) or "none".

## 10. Out of scope

- TODO: what this specification deliberately does not cover.

## 11. Assumptions

- TODO: what is taken for granted, and who confirmed it.

## 12. Open questions

<!-- One per line, as "- [NEEDS CLARIFICATION]: the question". It must be
     empty before approval: the agent never builds on a guess. -->
