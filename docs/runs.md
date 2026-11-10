# Runs

This page holds a full run with its output, the plan view and the run phases, and the [README](../README.md) gives the short first run.

## A full run

This run is real output from a small Go repository. The agent normally runs these commands, and the hooks enforce the order.

```console
$ harness run start "Let a customer cancel an order before it ships"
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=planning review_round=0 live_round=0
next: write the plan in .harness/runs/20261108-132848-let-a-customer-cancel-an-order-before-it/plan.md, then harness run plan
```

Write `plan.md` in the run folder. `harness plan --help` gives the format. Write each diagram through `harness diagram`: a `+`, `*` or `-` at the end of a line marks it new, changed or removed. `harness diagram --help` gives the forms for each diagram kind.

```console
$ harness diagram < states.mmd
```

```mermaid
stateDiagram-v2
    [*] --> pending
    pending --> paid: pay
    paid --> shipped: ship
    pending --> cancelled: cancel (new)
    paid --> cancelled: cancel (new)
    shipped --> [*]
    cancelled --> [*]
```

Check the plan, render it, and submit it:

```console
$ harness plan check
plan.md:43: note: diagram kind flowchart is not checked
$ harness plan view
/home/dev/shop/.harness/runs/20261108-132848-let-a-customer-cancel-an-order-before-it/plan.html
$ harness run plan
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=awaiting_approval review_round=0 live_round=0
next: ask the Plan question; when the user picks Approve, run harness run approve
```

Before the plan, the `explorer` agent reads `CONTEXT.md`, `docs/adr/` and the touched code. Each decision that the code cannot make goes under `## Open questions` in `plan.md`. After `harness run plan`, the agent grills the user in rounds through the question tool of the host (`AskUserQuestion` on Claude Code, `request_user_input` on Codex). Each answer moves to `## Decisions`.

When no question is open, the agent asks the `Plan` question with the options `Approve` and `Change the plan`. On `Approve`, the `post-tool-use` hook records the approval for the hash of the current `plan.md`. A plain reply in the chat records nothing.

```console
$ harness run approve
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=working review_round=0 live_round=0
next: implement test first, run harness verify, then harness run review
```

`harness run approve` refuses when no record matches the current `plan.md`, so an edit after the answer needs a new answer. It also checks `plan.md` again and refuses an open question, because the grill edits the plan after `harness run plan`. A model that the user names approves with `harness run approve --by <model> --reason "<one line>"`, and the transition log keeps both.

Write the test first. Each acceptance criterion of the plan is also a gate, `criterion-N`, that runs its proof command:

```console
$ harness verify
PASS go-fmt 15ms
FAIL go-vet 83ms
FAIL go-test 89ms
FAIL criterion-1 81ms
FAIL criterion-2 87ms
...
order/cancel_test.go:16:15: o.Cancel undefined (type *Order has no field or method Cancel)
error: gates failed
```

Write the code, commit, and verify again. `verify --commits` checks each commit in a temporary worktree: a `test:` commit must fail a gate, and every other commit must pass.

```console
$ harness verify
PASS go-fmt 9ms
PASS go-vet 270ms
PASS go-test 614ms
PASS criterion-1 317ms
PASS criterion-2 314ms
$ harness verify --commits main
PASS c46c4071e27f feat
```

Submit the change for review. A plugin checks the branch against the plan first: the file trees, the commits, the owned files of each task, the tests and the proofs.

```console
$ harness run review
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=reviewing review_round=0 live_round=0
next: review the change, then harness run ship, or harness run rework --findings
$ harness run ship
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=shipping review_round=0 live_round=0
next: merge the change, then harness run done --pr URL
$ harness run done --pr https://github.com/acme/shop/pull/42
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=done review_round=0 live_round=0
next: the run is closed, start the next with harness run start
```

The lead merges the findings of the reviewers and starts a verifier for each one. For the real findings, the lead sends the run back once with `harness run rework --findings "..."`. The guard acts on each tool call of the agent. For example, a merge goes to `harness hook --host claude pre-tool-use`, and the hook denies it:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"guard.pr-no-merge: a pull request merge is for the user. leave it to the user"}}
```

## Plan view

`harness plan view` writes the plan as one self-contained HTML page, with the diagrams, the file trees, the tasks and the state of each proof. The run also writes it when the plan waits for approval.

![The plan summary with the acceptance criteria and their proof state](images/plan-view-summary.png)

![The tech specs: a marked flowchart, a stateDiagram-v2 and a classDiagram](images/plan-view-specs.png)

![The files, the tasks, the commits and the traceability matrix](images/plan-view-tasks.png)

## Multi-run plan

A coordinator shows a program of runs as one page. The plan has `## Runs` and one `### R<n> slug` heading for each run. Each run has the fields Wave, Depends on, Branch, Owned files and Done when. A numbered scope list follows the bullets, with one step on each line.

```markdown
# Guard and coordinator plan

## Goal

Two runs make the guard exact and let the lead watch all runs.

## Runs

### R1 guard-precision

- Wave: 1
- Depends on: none
- Branch: feat/guard-precision
- Owned files: `internal/guard/domain/shell.go`
- Done when: a replay of the denied commands matches the expected result.

1. Follow `cd` before the source-path check.
2. Name the resolved file in each deny.

### R2 coordinator

- Wave: 2
- Depends on: R1
- Branch: feat/coordinator
- Owned files: `internal/run/ui/cli/list.go`
- Done when: the lead watches all runs with 1 command.

1. Add the list command for all runs.
```

- `harness plan check --file plan.md` lists each problem of the plan.
- `harness plan view --file plan.md` writes the page with the wave lanes, the dependency flowchart and the measurement.
- Each dependency names a run in an earlier wave.
- Runs in one wave own no common path.
- `harness run plan` refuses a multi-run plan with the rule `plan-kind`.

## Run phases

The phases and the events come from the transition table in `internal/run/domain`. A label in brackets is the guard that picks the row.

```mermaid
stateDiagram-v2
    [*] --> planning: start
    state active {
        planning --> awaiting_approval: submit-plan [approval on]
        planning --> working: submit-plan [approval off]
        awaiting_approval --> working: approve-plan
        working --> reviewing: submit-for-review
        reviewing --> working: request-rework [rounds left]
        reviewing --> shipping: approve-review
        working --> awaiting_approval: amend [approval on]
        reviewing --> awaiting_approval: amend [approval on]
        shipping --> awaiting_approval: amend [approval on]
        shipping --> releasing: complete [go-live on]
        releasing --> observing: deploy
        releasing --> planning: regress [live rounds left]
        observing --> planning: regress [live rounds left]
    }
    shipping --> done: complete [go-live off]
    observing --> done: close
    reviewing --> blocked: request-rework [last round]
    releasing --> blocked: regress [last live round]
    observing --> blocked: regress [last live round]
    active --> blocked: block
    active --> abandoned: abandon
    blocked --> active: resume [to the phase it left]
    blocked --> abandoned: abandon
    done --> [*]
    abandoned --> [*]
```

With `run.approval` off, `amend` from `working`, `reviewing` or `shipping` goes to `working`. A plan with only `docs` commits and `- none` under `## Open questions` also skips the approval on `submit-plan`, but never on `amend`. `docs/go-live.md` describes `releasing` and `observing`.

| Part | Where it acts |
|---|---|
| Plan check | `submit-plan`, `amend` and `approve-plan`: the plan rules, then the `plan.submitted` plugins |
| Gates | `harness verify` in `working`; the review needs a passing proof of each criterion |
| Trace check | `submit-for-review`: the trees, the commits and the owned files against the branch |
| Reviewers | `reviewing`: the agents of the `reviewer` role review the work. The common list runs on every listed model. Lens Spec runs on the first model, lens Standards on the second, and lens Spec on every other model. A verifier checks each finding. With real findings, the lead runs `harness run rework` once with the verified list. A worker fixes each finding and a re-review checks each fix, both in `working`. The final panel then runs the common list on the final `HEAD`. Then the phase goes to `ship` or `rework`. `ship` needs every listed model of the host on `HEAD` |
| Guard | each tool call (`tool.before` and `tool.after`), each stop (`turn.stopping`) of the agent and each run transition (`run.transition.before`), in every phase |
| Plugins | every transition (`run.transition.before`, `run.transition.after`), each gate, each plan, each host hook, gate detection, lint and live signals |
| Live check | `releasing` and `observing`: `harness live check` reads the deploy and the signals |
