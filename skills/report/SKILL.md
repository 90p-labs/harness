---
name: report
description: "File a bug about the harness itself (the loop, the hooks, the guard rules, the gates, the CLI) as an issue in 90p-labs/harness, with reproduction steps, redacted traces, a fix suggestion and a workaround, after the user confirms the draft. Use when the user types /report (Codex: $harness:report), or asks to report a harness bug, a wrong guard deny or a wrong hook result."
argument-hint: "[what went wrong]"
---

# Report a harness bug

The issue goes to `90p-labs/harness`. A fixed format lets a maintainer reproduce the bug without a question. An issue is public: file it only after the user says yes to the draft.

## Decision tree

```
Is the problem in the harness?
├── yes: a hook result, a guard deny, a gate, a run phase or a CLI command is wrong → continue
└── no: the problem is in the user repository or the host → tell the user, and stop
```

## 1. Reproduce the bug

1. Make an empty folder in the scratchpad, or in the temp dir on a host with none. Run `git init` in it.
2. Write only the `.harness.yaml` keys that the bug needs.
3. Run the harness commands that show the bug. Feed a hook its payload on stdin, for example `harness hook --host claude pre-tool-use < payload.json`.
4. Keep the exact commands, the output and the exit code (`echo $?`).

If the bug does not show in the clean folder, find the missing part of the setup first. Do not file a bug that you cannot reproduce.

## 2. Collect the traces

- The host and its version: `claude --version` or `codex --version`.
- The hook payload or the full command output of step 1.
- `harness report` adds the harness version, the OS and the active run id and phase itself.

Do not read `.env` files, keys or credentials.

## 3. Look for a duplicate

Run `gh issue list -R 90p-labs/harness --state all --search "<two or three keywords>"`. On a match, show it to the user. Offer a comment on it (`gh issue comment <n> -R 90p-labs/harness --body-file <file>`) instead of a new issue. A comment is public too: show its text and wait for a yes before you post it.

## 4. Write the draft

1. Copy `references/issue-template.md` to a file in the scratchpad or the temp dir.
2. Replace each `[fill: ...]` placeholder. Keep every heading: `harness report` refuses a draft with a missing, empty or unfilled section.
3. Fix suggestion: read the harness source, then name the file, the function and the change.
4. Workaround: give a step that the user can do today, or write "none".
5. Title: one line of 70 characters or fewer, for example `hook: a bad Stop payload exits 2 and blocks the stop`.
6. Write in short sentences, one idea in each sentence.

## 5. Check the draft

Run:

```sh
harness report --dry-run --title "<title>" --body-file <draft>
```

The command adds the environment block to Traces and redacts emails, home paths, tokens, keys and URL credentials. It exits 2 and names each missing section when the draft is not complete: fill them and run it again.

Read the output once more for a leak. The run line copies words of the user prompt: if it names a customer or a secret, ask the user before the filing. Show the title and the body of the output to the user. Stop the turn, and wait for a yes or for corrections.

## 6. File the issue

After the yes, run the same command without `--dry-run`:

```sh
harness report --title "<title>" --body-file <draft>
```

It files the issue with the label `bug` and prints its URL. Delete the draft file. Report the URL to the user in one line. Do not block or change an active run for the report.
