---
name: pr-open
description: "Opens the pull request of a branch as a draft, or edits the body of the open one. The body starts with a bottom line, then answers Why, What and How, in this order. How holds the plan diagrams and the visuals of the done tasks. Use in the ship step of the harness loop, or when the user types /90p:pr-open (Codex: $90p:pr-open)."
argument-hint: "[base branch]"
---

# Open a pull request

A reviewer reads the body before the diff. The body starts with a bottom line. Then it answers three questions, in this order: Why, What and How. The guard rule `pr-body-shape` denies another shape.

Arguments: `$ARGUMENTS`

## Decision tree

```
Start: run harness version ("Binary"), then step 1
├── harness run status shows a run → step 2 keeps the plan diagrams and adds the visuals of the done tasks
├── no run → step 2 applies the visual table to the changed files
├── a UI change → the wireframe row of the visual table
├── a visual to draw → references/visuals.md shows each kind
├── no pull request for the branch → step 4 creates a draft
├── a merged or closed pull request for the branch → step 4 creates a new draft
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

Run `harness pr body --base <base>` with the base of step 1. It prints the skeleton with the bottom line and Why placeholders. What has one bullet for each branch commit, and How has the `mermaid` fences of the plan. Write its output in a file `pr-body.md` in the scratchpad, or in the temp dir on a host with none. Keep its absolute path, `<body>` below. Do not write it in the repository.

Replace the two placeholders. Keep the fences as the command printed them. Edit a What bullet only when the diff needs a clearer name. Add the visuals of step 2 that the plan lacks.

```
<bottom line: what the PR changes and what the reviewer must check>

## Why

<why: the problem and the reason for the change>

## What

- <one bullet for each branch commit, printed by the command>

## How

The change adds no diagram.
```

- Write the body BLUF (bottom line up front). The bottom line is one or two sentences in one line. Each section starts with its main point.
- `## Why`: two to four sentences. Name the problem first. Use the plan `## Goal` and the decisions that a reviewer can question.
- `## What`: the command prints one bullet for each commit, in the order of the commits. Edit a bullet only when the diff needs a clearer name.
- `## How`: the mechanism. Put a visual of step 2 where it explains faster than text. Put one sentence above each visual: its main point.
- A How with no visual says why in one sentence.
- A line outside a fence has 40 words at most. A table row is one line.
- Use no em dash and no emoji.

## 4. Open or edit the pull request

1. Run `harness verify --commits <base>`. The guard denies a push and a create before it passes.
2. Run `harness pr check --body-file <literal absolute path of the body file>`. Fix every problem that it prints. Write the path as a literal, not as a variable.
3. Push the branch: `git push -u origin HEAD`.
4. Run `gh pr view --json url,state`. A failed call means no pull request. Only with `state` `OPEN`, run `gh pr edit <url> --body-file <body>`, with the `url` field.
5. With no pull request, or with a `MERGED` or `CLOSED` one, run `gh pr create --draft --base <base> --title "<the first commit subject>" --body-file <body>`.
6. Report the URL. Do not mark the pull request ready and do not merge it.

When the guard denies the body, read the reason, fix the body file and run the command again.
