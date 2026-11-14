# Learn the conventions

The learn step turns the repeated lessons of a run into durable project rules. It runs in `shipping`, before the `pr-open` skill, so the rules land in the same pull request. The step also turns what slowed the run into a change of the agent environment.

This step adapts the retro skill of Matt Pocock (MIT license): <https://github.com/mattpocock/skills/blob/5863806/skills/engineering/retro/SKILL.md>. The idea of sorting what slowed a run into named categories, each with a home, comes from that skill. The text here is new.

## Candidates

1. Run `harness run learn`. It lists a `candidate:` line for each real Standards finding and each real finding that a `gate-checkable` rework names. A plan line `Regression test: none: <reason>` gives the first line, with the source `no-seam`. It also lists an `evidence:` line for each item of the run evidence.
2. Add each correction of the user from the session and from `handoff.md`. Give it the source `user`.
3. Drop the rest. A Spec finding, a bug of another class and a one-off bug of the diff are not conventions.

- A `findings.md` with no `Lens` column gives no Standards candidate. Read such findings by hand, and keep the ones that name a repository rule.
- With no candidate, no user correction and no lesson, say so in one line and start the `pr-open` skill. Write nothing.
- With no candidate and no evidence, the command prints `learn: no convention candidates and no evidence`. The lead still reads `handoff.md`, because one of its lines can hold a user correction or support a lesson.

## Homes

Pick the first home that fits the rule.

1. A lint rule in the project lint pack, when a tool can check the rule. Do not write it in this run. Print a follow-up `harness run start` prompt for the rule and its canary. Give the prompt an entry in `docs/follow-ups.md`. The rule gets its own run and review.
2. An ADR under `docs/adr/`, plus a row in `docs/adr/index.md`, for a decision.
3. A `CONTEXT.md` entry, for a domain term or a convention.

A `no-seam` candidate has its own home. Print a follow-up `harness run start` prompt that builds the seam. Give the prompt an entry in `docs/follow-ups.md`. For a deliberate gap, write an ADR instead.

## No duplicate

Before a write, search `CONTEXT.md`, `docs/adr/` and the lint pack for the rule. The lead also reads the open causes of `docs/follow-ups.md`. When an entry already holds the rule, extend that entry. Write no duplicate.

## Fields

Give each convention these fields:

- State the rule in one sentence.
- Say why it matters, in one or two sentences.
- Give a bad example: the code before the fix.
- Give a good example: the code after the fix.
- Name the real files that follow it.

## Lessons

A lesson is a change of the agent environment that cites run evidence. The `evidence:` IDs are `rework-<n>`, `block-<n>`, `amends` and `stop-blocks`.

| Category | The lesson names | Evidence to read | Home |
|---|---|---|---|
| Navigation | What was hard to find, and the pointer that finds it | A `spec-gap` rework, a real finding about a missed file, the `amends` count | A pointer in `CONTEXT.md`, or in the line of its ADR in `docs/adr/index.md` |
| Automated checks | A check that no gate runs, or a mechanical mistake | A `gate-checkable` rework, a `candidate:` line, a real finding | A follow-up prompt for the gate or the lint rule |
| Tool economy | A command or a tool call that was repeated or wasted | A `minor` rework, rework lines that repeat one class, the `stop-blocks` count | A follow-up prompt for the script, the gate paths or the setting |
| No-ops | An instruction that changed nothing | A real finding that breaks a rule that a doc already states | The removal of the line, or a check in its place |
| Information access | Something the agent needed and could not reach | A `block` line, a line of `handoff.md` | A line in `CONTEXT.md` with the command or the path, or a follow-up prompt for the access |

- Cite the evidence: an `evidence:` line, a `candidate:` line, a row of `findings.md` or a line of `handoff.md`.
- Drop a lesson with no evidence. The memory of the lead is not evidence.
- A count line alone supports no lesson. The lesson also cites a line with text.
- A mechanical mistake gets a deterministic check, never a sentence of guidance.
- Write one lesson for each cause.
- A lesson does not repeat a convention. When a `candidate:` line already gets a lint rule prompt, write no lesson for the same check.
- A repeated rework class supports a lesson, but it is not a source of a convention.

Give each lesson these fields: the category, the evidence, the lesson in one sentence, and the home.

The homes of a lesson, in order:

1. A follow-up `harness run start` prompt, for a check, a gate, a script or a setting. Its entry goes in `docs/follow-ups.md`.
2. The `docs:` commit of this run, for a pointer or a removed instruction in a file of the repository.
3. A draft for the `report` skill, for a lesson about the harness plugin outside the repository.

The no-duplicate search covers a lesson too. Search before the write, and write nothing that a doc already says. One exception: a repeat of an open cause adds a `Cited by` line to its entry.

## Follow-ups

Each follow-up prompt gets an entry in `docs/follow-ups.md`, so a printed prompt is not lost. A lint rule prompt and a `no-seam` prompt get one too. The entry has this form:

```text
### <the cause in one line>

- Category: <the category of the lesson, or Convention>
- Cited by: `<run id>`, `<evidence ID or file:line>`
- Prompt: `harness run start "<the prompt>. Remove the entry of this cause from docs/follow-ups.md."`
```

- List the open causes once for the whole step: `grep -n '^### ' docs/follow-ups.md`.
- Compare by cause, not by wording.
- When no entry holds the cause, add one under `## Open`.
- A new file starts with the title `# Follow-ups`, one line that names `skills/harness/references/learning.md`, the heading `## Open`, and then its entries.
- A missing file means that no cause is open. The grep then fails, and that is not an error.
- When an entry holds the cause, add one `- Cited by:` line. Write no second entry, and print the prompt again.
- A `Cited by` line holds two code spans: the run id, then one evidence ID or one `file:line`. One line stands for one run.
- The count of `Cited by` lines shows a repeat. The grep prints headings only, so the lead reads the entry of the cause to count its lines. Name the count in the `Conventions` question.
- The `Prompt` line holds one code span with no backtick inside. The prompt asks the follow-up run to remove the entry.
- The category is one of the five lesson categories, or `Convention` for the prompt of a candidate.

## Plugin lessons

A lesson about the harness plugin goes to the `report` skill. The host command is `/90p:report`, and on Codex it is `$90p:report`.

- The lead drafts the report with the skill, up to the dry run. The learn step files nothing and posts nothing.
- The lesson and its evidence are the argument of the skill.
- When the search of the skill finds an issue of the cause, the draft is a comment for that issue. A repeat of a plugin lesson then shows as a comment on its issue.
- The duplicate search of the skill is the one search of the lesson. Skip the search of the repository files.
- A plugin lesson is not in the `Conventions` question. Its draft changes no file, and the confirmation of the skill decides the filing.
- Keep the draft as `report-<n>.md` in the run folder before the `Conventions` question, and name it in `handoff.md`, with its title. The number `<n>` counts the drafts of the run from 1.
- The answers `Write` and `Skip` and an empty question do not drop the draft. With a plugin lesson alone, the lead asks no `Conventions` question, keeps the draft and starts the `pr-open` skill.
- The lead does not stop the turn at step 5 of the skill during the learn step, with a user or without one. The same holds for a match at step 3 of the skill.
- With a user, the lead asks for the confirmation after `harness run wait --pr <url>`. The stop rule allows a stop in `shipping` only when the run holds a pull request URL.
- Unattended means that the user told the lead to ask nothing, or that the session has no user. The lead then asks for no confirmation.
- The last message lists each kept draft that is not filed, with a user or without one.

## Question

Ask one round, with the header `Conventions` and the options `Write` (first) and `Skip`. List the candidates in the question text, each with its home. Then list the lessons, each with its category, its evidence and its home. The list holds no plugin lesson.

| Host | Tool | Options |
|---|---|---|
| Claude Code | `AskUserQuestion` | 2 to 4 |
| Codex | `request_user_input` | 2 to 3 |

- A free-text answer drops or changes candidates and lessons. Apply it, and ask again only when a candidate or a lesson stays unclear.
- A host with no question tool gets a plain-text round: the list, the options and the `Write` option first. Then stop and wait for the reply.
- On `Skip`, write nothing, print no follow-up prompt, write no entry in `docs/follow-ups.md` and start the `pr-open` skill. A kept draft of a plugin lesson stays.

## Write

1. Write each confirmed convention to its home, with the fields above. Write each confirmed lesson to its home too, and print each follow-up prompt. Write its entry in `docs/follow-ups.md`, or add the `Cited by` line to the entry of the cause.
2. Make one `docs:` commit for all the writes, the lesson writes included.
3. Run `harness verify --commits <default branch>`.
4. Start the `pr-open` skill.
