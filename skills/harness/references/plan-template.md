# Plan template

This page holds the template of `plan.md` and the rules that `harness plan check` enforces. Run `harness plan --help` for the full form of each rule.

The explorer and the advisor read this page. The classes of common review findings help them to find the risks before the work starts.

## Skeleton

The level-2 headings come in this order. `## Threat model` is optional unless a tree path has a `guard` or `security` folder.

````markdown
# Plan: <title>

## Goal

Track: full

<What the change does, in 3 sentences at most.>

## Acceptance criteria

| # | Criterion | Proof |
|---|---|---|
| 1 | <One observable result.> | `go test ./internal/thing/domain -run TestThing -count=1` |

## Tasks

### 1. <Name>

- Side: backend
- Owned files: `internal/thing/domain/thing.go`, `internal/thing/domain/thing_test.go`
- Test first: `thing_test.go`: `TestThing` fails, because `Thing` does not exist.
- DoD: criterion 1.
- Depends on: none

## Review risks

- <A likely review finding.> Test: `TestThing`.
- <A likely review finding that no test shows.> Decision: <the choice and the reason>.

## Threat model

- Attacker: an honest agent that wants to finish faster. It is not a hostile agent.
- Input forms: <the closed list of the input forms that the change handles>.
- Fail closed: <the verdict for each input outside the list>.

## Decisions

- <date>: <topic>: <the decision and the reason>.

## Open questions

- none

## Tech specs

### Thing domain

Code: task 1.
Criteria: 1

<What the code does.>

### Final code tree

```tree
harness/
└── internal/
    └── thing/
        └── domain/
            └── thing.go     new
```

### Final test tree

```tree
harness/
└── internal/
    └── thing/
        └── domain/
            └── thing_test.go     new
```

### Commits

1. `feat: add the thing` (task 1)

### PR sequence

1. `feat/thing` (this run), based on `main`: the thing.
````

## Rules

Each line gives the rule id, what it checks, and the fix.

- `heading-missing`: a required heading is absent or out of order. Add the heading in the order of the skeleton.
- `criteria-table`: the table header, the numbering or a proof is wrong. Number from 1 and give one command in backticks.
- `task-field`: a task heading or a task field is wrong. Give each field of the skeleton.
- `criterion-unowned`: a criterion is in no DoD line. Name it in the DoD of one task.
- `dod-unknown`: a DoD line names a criterion that does not exist. Fix the number.
- `task-depends`: a dependency names a missing task or makes a cycle. Fix the `Depends on` line.
- `tree-format`: a tree line has a wrong indent, root or mark. Copy the format of the skeleton.
- `tree-unowned`: a tree file is in the owned files of no task. Add the path to `Owned files`.
- `task-uncommitted`: a task is in no commit. Add the task to a commit line.
- `commit-format`: a commit line is not a conventional commit with its tasks. Use `type: subject` and `(task n)`.
- `pr-sequence`: the PR list does not mark exactly one `(this run)`. Mark one PR.
- `spec-criteria`: a tech spec has no valid `Criteria` line. List the criteria that it covers.
- `trace-unlinked`: a criterion has no tech spec, or a task owns no tree file. Link it.
- `signals-missing`: the code tree adds a command, a query, an endpoint or a consumer, and the plan has no `## Signals`. Add the table.
- `signals-table`: the signals table header, breach or window is wrong. Use an operator with a number, and a Go duration.
- `regression-impact`: a live round needs `## Regression impact` with an archived plan file. Add the section.
- `diagram-required`: a file needs a diagram or a sketch link that the plan lacks. Add the mermaid fence.
- `diagram`: a diagram fails its syntax or its mark check. Fix the fence.
- `open-questions`: the section is not `- none` or one question on each line. Fix the list.
- `sentence-length`: a sentence has more than 25 words. Split the sentence.
- `em-dash`: the text holds an em dash. Use a comma or a full stop.
- `risk-unanchored`: a risk has no `Test:` with a backticked name or no `Decision:` text, or the test is in no proof. Anchor the risk.
- `threat-model`: a tree path in a `guard` or `security` folder needs the 3 bullets. Add `## Threat model`.
- `plan-size`: a single-run plan has more than 6 tasks. Split the change into a multi-run plan.
- `run-field`: a run heading or a run field is wrong. Fix the field.
- `run-depends`: a run depends on a run that is not in an earlier wave. Fix the dependency.
- `run-overlap`: two runs of one wave own a common path. Move the path to one run.
- `plan-kind`: `harness run plan` takes only a single-run plan. Submit one run at a time.
- `tree-diff`: a tree mark does not match the state of the file in the diff. Fix the mark or the file.
- `file-untracked`: a changed file is in neither final tree. Add it to the tree.
- `commit-missing`: a planned commit is not in the branch log. Make the commit.
- `commit-scope`: a commit touches a file outside the owned files of its tasks. Move the change.
- `test-missing`: a `Test first` name is not in its test file. Write the test.
- `proof-missing`: a criterion has no passed proof for the current files. Run `harness verify`.

## Proofs

`harness verify` runs each proof as the gate `criterion-N`. So each proof is one command.

A `go test -run TestNew` proof passes on the base when the test is missing. Go prints `no tests to run` and exits 0.

The test-first commit gate proves that the new test fails on the parent. Use `go test -list` to check that the test exists.

## Size

A single-run plan has 6 tasks at most (rule `plan-size`). Split a change of more than 6 tasks into a multi-run plan with `## Runs`.

Split a change of about 800 changed lines in the same way. The plan has no line count, so the advisor estimates it. See `harness plan --help`.

## Tree-only amend

An amend in `working` with no live round needs no new approval in 2 cases.

1. The plan changes only inside the 2 tree fences.
2. The plan adds or removes a test path in an `Owned files` line.

A test path comes from a closed list:

- A Go name that ends in `_test.go`.
- A Python name that starts with `test_` or ends in `_test.py`.
- A JavaScript or TypeScript name with `.test.` or `.spec.` before its code extension.
- A data file under a folder named exactly `testdata`, with the extension `.md`, `.json`, `.jsonl`, `.txt`, `.yaml`, `.yml` or `.golden`.

Any other change needs the approval. This includes an amend from `reviewing` or `shipping`.

## Threat model

Write `## Threat model` for a change under a `guard` or `security` folder. Write it also for a change of an approval, a permission or a secret path. The path trigger does not see these paths.

- Attacker: an honest agent, not a hostile one. See ADR 0019 in `docs/adr/0019-guard-threat-model.md`.
- Input forms: a closed list of the input forms that the change handles. Include each form of the input that the changed code reads, and each way that the input selects its target: its position, an option or flag, and the working folder. Each item is also a review risk line with its test.
- Fail closed: an input outside the list keeps the old verdict or denies.

## Common review findings

The lead audited 50 rework reasons of past runs. Each reason counts once for each class that it matches, so the counts add to more than 50.

| Class | Count | How the plan prevents it |
|---|---|---|
| A doc sentence drifts from the code or from another doc | 29 | List each doc that repeats the fact in a risk, with a test that reads both. |
| A prose rule fails: sentence length, ASD wording, spelling | 20 | Run the prose gate on each changed doc, and keep one idea in each sentence. |
| A proof or a test cannot fail or misses a case | 15 | Give each risk a test that fails on the base, and one case for each branch. |
| Guidance conflicts with a checker | 10 | Read the checker code, and record each conflict as a decision with options. |
| Dead code, duplication or speculative surface | 10 | Add no code without a caller in the plan, and name the one owner of each rule. |
| A bare literal or a naming rule fails | 9 | Decide the named constants and the naming pattern before the code. |
| An input form that the code does not handle | 8 | List the input forms in a risk, with one test for each form. |
| A test does not exercise the production path | 6 | Run the production entry point in the test, with the real setup. |
| A documented step fails in the real world | 3 | Run each read-only command of the docs, and check that each download target exists. |
