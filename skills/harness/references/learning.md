# Learn the conventions

The learn step turns the repeated lessons of a run into durable project rules. It runs in `shipping`, before the `pr-open` skill, so the rules land in the same pull request.

## Candidates

1. Run `harness run learn`. It lists a `candidate:` line for each real Standards finding and each real finding that a `gate-checkable` rework names.
2. Add each correction of the user from the session and from `handoff.md`. Give it the source `user`.
3. Drop the rest. A Spec finding, a bug of another class and a one-off bug of the diff are not conventions.

- A `findings.md` with no `Lens` column gives no Standards candidate. Read such findings by hand, and keep the ones that name a repository rule.
- With no candidate and no user correction, say so in one line and start the `pr-open` skill. Write nothing.

## Homes

Pick the first home that fits the rule.

1. A lint rule in the project lint pack, when a tool can check the rule. Do not write it in this run. Print a follow-up `harness run start` prompt for the rule and its canary. The rule gets its own run and review.
2. An ADR under `docs/adr/`, plus a row in `docs/adr/index.md`, for a decision.
3. A `CONTEXT.md` entry, for a domain term or a convention.

## No duplicate

Before a write, search `CONTEXT.md`, `docs/adr/` and the lint pack for the rule. When an entry already holds it, extend that entry. Write no duplicate.

## Fields

Give each convention these fields:

- State the rule in one sentence.
- Say why it matters, in one or two sentences.
- Give a bad example: the code before the fix.
- Give a good example: the code after the fix.
- Name the real files that follow it.

## Question

Ask one round, with the header `Conventions` and the options `Write` (first) and `Skip`. List the candidates in the question text, each with its home.

| Host | Tool | Options |
|---|---|---|
| Claude Code | `AskUserQuestion` | 2 to 4 |
| Codex | `request_user_input` | 2 to 3 |

- A free-text answer drops or changes candidates. Apply it, and ask again only when a candidate stays unclear.
- A host with no question tool gets a plain-text round: the list, the options and the `Write` option first. Then stop and wait for the reply.
- On `Skip`, write nothing and start the `pr-open` skill.

## Write

1. Write each confirmed convention to its home, with the fields above.
2. Make one `docs:` commit for all the writes.
3. Run `harness verify --commits <default branch>`.
4. Start the `pr-open` skill.
