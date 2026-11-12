# Grill the plan

The grill turns each open question of `plan.md` into a decision of the user. It runs after `harness run plan` and before the approval, except the one early round for the evidence baseline.

This step adapts the grilling skill of Matt Pocock (MIT license): <https://github.com/mattpocock/skills/blob/main/skills/productivity/grilling/SKILL.md>. The model of the design tree, the rounds and the recommended answers comes from that skill. The text here is new.

## Model

- **Design tree**: each decision of the plan is a node. A decision can open more decisions below it.
- **Frontier**: the open decisions whose parents are decided. Only the frontier goes to the user.
- **Round**: one call of the question tool with all the frontier questions. The answers of a round can open new questions, which make the next frontier.

## Facts and decisions

- A fact is in the code, the docs or the history. The explorer agent finds the facts. Never ask the user for a fact.
- A decision is a choice between options that the code cannot make. Each decision goes to the user. Never take a decision for the user.
- When the explorer cannot find a fact, write it as a risk, not as a question.
- The ambiguous items of the evidence baseline get one round before `plan.md` is written, with the same question tool, and their answers go to `## Decisions`. Record each one as `- <date>: evidence baseline: <class>: <behaviour>: <outcome>`, not as `- <date>: <question>: <answer>`.

## Questions

- Give each question a recommended answer. Put the recommended option first, with the suffix ` (Recommended)`.
- Write the question so that "yes" accepts the recommendation. Example: "Does the retry stop after 3 attempts?"
- Give each option a short label and one line that tells what follows from it.
- Ask one decision in each question. Split a question that has two decisions.

## Points of view

Grill the plan from these 3 points of view, in this order. Go to the next view only when the frontier of the current view is empty.

### Product

- Find each empty state: no data, no run, no plan, a first use.
- Find each edge case of the state machine and each impossible transition.
- For each one, ask what the user sees and what the system does.

### QA

- Find the tester edge cases: empty input, maximum size, concurrent calls, a lost connection, a retry, a partial failure and a timeout.
- For each one, ask how a test proves the behaviour.

### Tech specs

- Use this view only when the plan has a spec that can fail, such as a schema, a wire format, a migration or a limit.
- For each spec, ask what happens when it fails and how a test proves it.

## Question tool of the host

| Host | Tool | Questions in one call | Options in one question |
|---|---|---|---|
| Claude Code | `AskUserQuestion` | 4 | 2 to 4 |
| Codex | `request_user_input` | 3 | 2 to 3 |

- When the frontier has more questions than the limit, ask them in more than one call. Each call is part of the same round.
- Codex shows the tool only in Plan mode, or in Default mode with the feature `default_mode_request_user_input`. In another mode, tell the user to switch to Plan mode.
- Codex adds the option "Other" by itself. Claude Code also lets the user type a free answer.
- Never put answers in the input of the tool. The guard rule `approval-no-forge` denies it.
- A host with no question tool gets a plain-text round: a numbered list of questions, each with its options and the recommended option first. Then stop and wait for the reply.

## Record each answer

1. Move each answered question from `## Open questions` to `## Decisions`, as `- <date>: <question>: <answer>`.
2. Write a new question that an answer opens into `## Open questions`.
3. When `## Open questions` is empty, write `- none`.
4. Run `harness plan check` and `harness plan view` after each change of `plan.md`, and ask the user to reload the page.

## Done

The grill is done only when both conditions are true:

1. `## Open questions` holds only `- none`.
2. The user confirms that no decision is left.

Then run `harness plan view`, so the page shows the current plan, and ask the user to reload it. Then ask the approval: the question with the header `Plan` and the options `Approve` and `Change the plan`.
