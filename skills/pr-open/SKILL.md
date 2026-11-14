---
name: pr-open
description: "Opens the pull request of a branch, as a draft or ready by the pull request policy, or edits its body. The body starts with a bottom line, then answers Why, What, How, Evidence and Merge danger, in this order. How holds the plan diagrams and the visuals of the done tasks. Evidence shows the behaviour before and after. Merge danger states the door and the blast radius. Use in the ship step of the harness loop, or when the user types /90p:pr-open (Codex: $90p:pr-open)."
argument-hint: "[base branch]"
---

# Open a pull request

A reviewer reads the body before the diff. The body starts with a bottom line. Then it answers three questions, in this order: Why, What and How. Two parts follow: the evidence that the change works, and the danger of the merge. The guard rule `pr-body-shape` denies another shape.

This skill adapts the pull request skill of Matt Pocock (MIT license): <https://github.com/mattpocock/skills/blob/main/skills/engineering/pr/SKILL.md>. The Evidence and Merge danger parts come from that skill. The text here is new. That skill credits the show-me skill of Dex Horthy: <https://github.com/humanlayer/skills/blob/main/plugins/show-me/skills/show-me/SKILL.md>.

Arguments: `$ARGUMENTS`

## Decision tree

```
Start: run harness version ("Binary"), then step 1
├── harness run status shows a run → step 2 keeps the plan diagrams and adds the visuals of the done tasks
├── no run → step 2 applies the visual table to the changed files
├── a UI change → the wireframe row of the visual table
├── a visual to draw → references/visuals.md shows each kind
├── no pull request for the branch → step 4 creates it, as a draft or ready by the policy
├── a merged or closed pull request for the branch → step 4 creates a new one by the policy
├── an open pull request → step 4 edits its body
└── the guard denies the body → fix the body file of step 3, and run the command again
```

## Binary

- In each command below, `harness` means the full path `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`. Write the full path in each command.
- On Codex, `${CLAUDE_PLUGIN_ROOT}` shows unexpanded. Use `bin/harness` two folders above this `SKILL.md`.
- Run `harness version` first. If it fails, stop and report the error. Skip it when the harness skill started this skill: the loop already ran it.

## 1. Collect the sources

1. The base is the argument. With no argument, it is the `PR sequence` base of the plan, else `git.default_branch` of `.harness.yaml`.
2. Run `harness run status`. Its first line is `run <id> phase=...`. The plan is `.harness/runs/<id>/plan.md`. With `active: no`, there is no plan. Skip the command when you hold the run id from the harness loop.
3. Read the commits: `git log --format='%h %s' <base>..HEAD`.
4. A done task is a plan task whose `### Commits` line has the subject of a commit in that log. That line names the task with `(task n)`. The commit subject has no `(task n)`.
5. Read the changed files: `git diff --name-only <base>...HEAD`. Read the diff of each file that a reviewer must understand.

## 2. Pick the visuals

- Read the `mermaid` fences of the plan, each under its `### ` heading of `## Tech specs`.
- Keep a diagram when the diff changes a file of its section: the `Code: task n` line names the tasks, and each task names its owned files.
- Drop a diagram when the diff changes none of its files.
- Copy a kept fence as it is. Its marks show what is new, changed and removed.
- Add each visual that a done task needs and the plan lacks. With no plan, use the changed files. Pick it from this table. `references/visuals.md` has an example of each.

| Kind of change | Visual |
| --- | --- |
| A state or phase change | `stateDiagram-v2` |
| A new command, hook or pipeline | `flowchart` |
| A call across processes, an HTTP edge or a consumer | `sequenceDiagram` |
| A user-facing flow: the steps of a CLI command, a page flow | `journey` |
| A UI change | A wireframe in a `text` fence |
| A domain model or contract change | `classDiagram` |
| A schema change | `erDiagram` |

- Pipe a new state, flow, class or ER visual to `harness diagram` and paste its output. The guard denies an unmarked one.
- A `sequenceDiagram` and a `journey` take no mark. Paste each in a plain `mermaid` fence.
- Do not link the plan `.excalidraw` sketch. `.harness/runs/<id>/` is not in git. Copy its layout in the text wireframe.

## 3. Write the body

Run `harness pr body --base <base>` with the base of step 1. It prints the skeleton with placeholders for the bottom line, Why, Evidence and Merge danger. What has one bullet for each branch commit, and How has the `mermaid` fences of the plan. Write its output in a file `pr-body.md` in the scratchpad, or in the temp dir on a host with none. Keep its absolute path, `<body>` below. Do not write it in the repository.

Replace each placeholder. Keep the fences as the command printed them. Edit a What bullet only when the diff needs a clearer name. Add the visuals of step 2 that the plan lacks.

```
<bottom line: what the PR changes and what the reviewer must check>

## Why

<why: the problem and the reason for the change>

## What

- <one bullet for each branch commit, printed by the command>

## How

The change adds no diagram.

## Evidence

Before: <what the behaviour was, in one line>

<proof: a screenshot, or a fence with the run command and its output>

After: <what the behaviour is now, in one line>

<proof: a screenshot, or a fence with the run command and its output>

## Merge danger

- Door: <one-way or two-way>. <how to reverse the merge, or why it cannot be reversed>
- Blast radius: <small, medium or large>. <what breaks and for whom if the change is wrong>
```

- Write the body BLUF (bottom line up front). The bottom line is one or two sentences in one line. Each section starts with its main point.
- `## Why`: two to four sentences. Name the problem first. Use the plan `## Goal` and the decisions that a reviewer can question.
- `## What`: the command prints one bullet for each commit, in the order of the commits. Edit a bullet only when the diff needs a clearer name.
- `## How`: the mechanism. Put a visual of step 2 where it explains faster than text. Put one sentence above each visual: its main point.
- A How with no visual says why in one sentence.
- `## Evidence`: show the behaviour, do not state it. `Before:` and `After:` each take one line and a proof below it.
- A change that a user sees takes a screenshot, by an http or https URL that the reviewer can open. A local path does not render. The check counts an image only with an http or https URL.
- Any other change takes a fence with the run command and its output. The red and the green run of the task test fit well.
- The Before comes from the base: the red run, or the command on the base branch.
- A docs-only change shows the old and the new text in a `diff` fence, or the run of its doc test. No change is waived.
- Drop a fence line from a pasted output, because the check reads it as the end of the fence.
- Replace an em dash or an emoji of a pasted output, because the guard reads a fence too.
- `## Merge danger`: two lines, `Door:` and `Blast radius:`, each with a value and a reason.
- `Door:` is `two-way` when a revert of the merge restores the old behaviour. It is `one-way` when a revert does not undo the effect, such as a data migration or a published contract.
- `Blast radius:` is `small`, `medium` or `large`. The reason says what breaks and for whom if the change is wrong.
- `small` is one command, page or job. `medium` is one service or one group of users. `large` is every user, shared data or money.
- A line outside a fence has 40 words at most. A table row is one line.
- Use no em dash and no emoji.

## 4. Open or edit the pull request

1. Run `harness pr policy`. It prints `draft: required` or `draft: optional`, then `merge: user` or `merge: agent`. If `harness pr policy` fails, keep the draft, and the user merges.
2. Run `harness verify --commits <base>`. The guard denies a push and a create before it passes.
3. Run `harness pr check --body-file <literal absolute path of the body file>`. Fix every problem that it prints. Write the path as a literal, not as a variable.
4. Push the branch: `git push -u origin HEAD`.
5. Run `gh pr view --json url,state,isDraft`. A failed call means no pull request. Only with `state` `OPEN`, run `gh pr edit <url> --body-file <body>`, with the `url` field.
6. With no pull request, or with a `MERGED` or `CLOSED` one, create it by the `draft` line of the policy.
   - `draft: required`, or a failed policy: run `gh pr create --draft --base <base> --title "<the first commit subject>" --body-file <body>`.
   - `draft: optional`: run the same command without `--draft`.
7. With `draft: optional` and an open draft, run `gh pr ready <url>`. With `draft: required`, leave the draft as it is.
8. Act on the `merge` line, after the create or the ready.
   - `draft: required` with `merge: agent`: report the URL. The user marks it ready and merges, because the guard denies `gh pr ready`.
   - A failed `harness pr policy`: report the URL. The user merges.
   - `merge: user`: report the URL. The user merges.
   - `draft: optional` with `merge: agent`: run `gh pr checks <url>`. Only when it exits 0, run `gh pr merge <url> --squash`. A failed, pending or missing check means stop and report.
9. Report the URL and the state. The merge comes before `harness run report` and `harness run done --pr <url>` of the harness skill.

When the guard denies the body, read the reason, fix the body file and run the command again.
