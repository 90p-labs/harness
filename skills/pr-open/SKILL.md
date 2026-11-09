---
name: pr-open
description: "Opens the pull request of a branch as a draft, or edits the body of the open one. The body answers Why, What and How, in this order, with the plan diagrams in How. Use in the ship step of the harness loop, or when the user types /harness:pr-open (Codex: $harness:pr-open)."
argument-hint: "[base branch]"
---

# Open a pull request

A reviewer reads the body before the diff. The body answers three questions, in this order: Why, What and How. The guard rule `pr-body-shape` denies another shape.

Arguments: `$ARGUMENTS`

## Decision tree

```
Start: run harness version ("Binary"), then step 1
├── harness run status shows a run → step 2 keeps the plan diagrams of the changed files
├── no run → step 2 draws a diagram only for a changed state machine or flow
├── no pull request for the branch → step 4 creates a draft
├── an open pull request → step 4 edits its body
└── the guard denies the body → fix the body file of step 3, and run the command again
```

## Binary

- In each command below, `harness` means the full path `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`. Write the full path in each command.
- On Codex, `${CLAUDE_PLUGIN_ROOT}` shows unexpanded. Use `bin/harness` two folders above this `SKILL.md`.
- Run `harness version` first. If it fails, stop and report the error.

## 1. Collect the sources

1. The base is the argument. With no argument, it is the `PR sequence` base of the plan, else `git.default_branch` of `.harness.yaml`.
2. Run `harness run status`. Its first line is `run <id> phase=...`. The plan is `.harness/runs/<id>/plan.md`. With `active: no`, there is no plan.
3. Read the commits: `git log --format='%h %s' <base>..HEAD`.
4. Read the changed files: `git diff --name-only <base>...HEAD`. Read the diff of each file that a reviewer must understand.

## 2. Pick the diagrams

- Read the `mermaid` fences of the plan, each under its `### ` heading of `## Tech specs`.
- Keep a diagram when the diff changes a file of its section: the `Code: task n` line names the tasks, and each task names its owned files.
- Drop a diagram when the diff changes none of its files.
- Copy a kept fence as it is. Its marks show what is new, changed and removed.
- With no plan, draw a diagram only when the diff changes a state machine or a flow. Pipe the diagram to `harness diagram` and paste its output.
- A `sequenceDiagram` takes no mark. Paste it in a plain `mermaid` fence.

## 3. Write the body

Write the body in a file `pr-body.md` in the scratchpad, or in the temp dir on a host with none. Keep its absolute path, `<body>` below. Do not write it in the repository.

```
## Why

<the problem and the reason for the change, from the plan Goal and Decisions>

## What

- <one bullet for each change that a reviewer sees in the diff>

## How

<how the change works, with the kept diagrams>
```

- `## Why`: two to four sentences. Name the problem first. Use the plan `## Goal` and the decisions that a reviewer can question.
- `## What`: one bullet for each change, in the order of the commits. Name the file or the command.
- `## How`: the mechanism. Put a diagram where it explains faster than text: a state machine, a flow, a sequence, a class or an ER change. Put one sentence above each diagram.
- A line outside a fence has 40 words at most. A table row is one line.
- Use no em dash and no emoji.

## 4. Open or edit the pull request

1. Run `harness verify --commits <base>`. The guard denies a push and a create before it passes.
2. Push the branch: `git push -u origin HEAD`.
3. Run `gh pr view --json url,state`. With an open pull request, run `gh pr edit --body-file <body>`.
4. With no pull request, run `gh pr create --draft --base <base> --title "<the first commit subject>" --body-file <body>`.
5. Report the URL. Do not mark the pull request ready and do not merge it.

When the guard denies the body, read the reason, fix the body file and run the command again.
