# Chart a map

Use a map for a large effort whose decisions are not taken yet. The map comes before the multi-run plan. A map needs no run. It is a file in the repository, for example `docs/programs/<name>/map.md`.

This step adapts the wayfinder skill of Matt Pocock (MIT license): <https://github.com/mattpocock/skills/blob/main/skills/engineering/wayfinder/SKILL.md>. The model of the map, the decision tickets and the frontier comes from that skill. The text here is new. For the cut of tickets, see also the to-tickets skill: <https://github.com/mattpocock/skills/blob/main/skills/engineering/to-tickets/SKILL.md>.

## The rule

- Plan and do not do. A ticket ends with an answer.
- No ticket changes the product.
- A prototype is thrown away after it gives its answer.
- A ticket of type `work` is not a task of a plan. It is a small action that a decision waits for.

## Chart the map

1. Write the destination in 3 sentences at most.
2. Write what is out of scope.
3. List what is not yet specified, one line for each unknown.
4. Cut each unknown into one decision ticket with one question.
5. Set `Blocked by` only where an answer needs another answer.

## Tickets

Each ticket is a heading `### T<n> slug` under `## Tickets`, then one line `- Field: value` for each field.

| Field | Form |
|---|---|
| `Type` | `research`, `prototype`, `grilling` or `work` |
| `Blocked by` | `none`, `T1`, `T1 and T3` or `T1, T2 and T3` |
| `Status` | `open` or `closed` |
| `Claimed by` | a name, only on an open ticket |
| `Question` | one question that ends with a question mark |
| `Answer` | the answer, only on a closed ticket |

The fields come in this order: Type, Blocked by, Status, Claimed by, Question, Answer. `Claimed by` is optional. The other fields are required, except `Answer`, which a closed ticket needs. For a full example, see the section Map of `docs/runs.md`.

## Work each type

- `research`: a fact that the code, the docs or the web gives. The explorer agent or a read-only session finds it.
- `prototype`: a question that only running code answers. Build it in a throwaway branch or folder, and keep only the answer.
- `grilling`: a decision of the owner. Ask it with a recommended answer, as `references/grill.md` tells.
- `work`: a small action that must finish before a decision, such as a request for access.

## Work the frontier

1. Run `harness plan check --file <path>`. It prints one `frontier:` line for each ticket that is open, unblocked and unclaimed.
2. Take one ticket. Add the line `- Claimed by: <name>` after `Status` and before `Question`, and commit that line before the work. Another session sees the claim after it pulls that commit. A session frees a claim by removing the line.
3. Do the work of the type. Never edit the product.
4. To close the ticket, set `Status: closed`, remove the claim line and add the `Answer` line.
5. Add the line `- T<n>: <answer>` under `## Decisions so far`.
6. An answer can open new tickets, and it can remove a line of `## Not yet specified`.

## When the map is clear

The check prints `map: clear` when no ticket is open and `## Not yet specified` holds only `- none`. Then the multi-run plan is final, with one slice for each run. See `docs/runs.md`. A plan written before that names the open ticket in each run that waits on it. `harness run plan` refuses a map.
