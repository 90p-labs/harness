# Replay: readme-focus

- Run: 20261109-204315-make-readme-md-short-and-focused-on-usab
- Base: 183da4e
- Prompt: the original prompt of the run. Rework reasons: only those recorded before the run start (41 lines).
- Explorer: haiku, brief agents/explorer.md blob 7a4eaf0f013b7be6e101654a001256a7525df75d, one sample.
- Advisor: fable (agent type 90p:advisor, told to follow the new brief file), brief agents/advisor.md blob 2cdd7b856e726873a85edbba675089b5231fb028, one sample, on the first plan draft of the run.
- Redaction: the scratchpad prefix is written as <scratch>/.

## Explorer report

## Facts
1. README.md is 522 lines. Sections: Intro 1-11, Install 13-72, Set up a repository 74-175, Plugins 177-318, A full run 320-417, Plan view 419-427, Run phases 429-474, Report a harness bug 476-486, Repository layout and tests 488-507, Documents 509-518, License 520-522.
2. Baseline at 183da4e: `go test ./docs/... ./examples/... ./skills/...` passes.
3. docs/readme_test.go:27-46 pins the Install subsections to exactly `Claude Code`, `Codex`, `Build from source`. The section ends at the next `\n## `.
4. docs/readme_test.go:64-78 pins the Install lines `- The skills ` and `- The agents `. Each skill folder and agent name must appear in backticks on its line.
5. docs/design_test.go:58-68 pins README text: `` `Plan` question `` (ApprovalHeader, internal/shared/domain/question.go:13) and `harness run approve --by`. Both sit in A full run (README:362, :370).
6. skills/skills_test.go:125-132 forbids an em dash in README. Passes today.
7. skills/pr-open/pr_open_test.go:44-57 requires `pr-open` in README (lines 17 and 21).
8. docs/config_full_docs_test.go:196-202 requires a README link to configuration-full.md (README:175).
9. Inbound README links: docs/configuration.md:113 (#plugins), docs/configuration.md:413 (#set-up-a-repository), examples/README.md:61 (#plugins), examples/README.md:110 (#custom-lint-pack-and-lint-engine-plugin). skills/harness/SKILL.md:30 names the section as text. The `../README.md#rules-for-every-example` links in examples/*/README.md resolve to examples/README.md:29, so they stay valid.
10. docs/configuration.md:113 says "four examples". The README Plugins section has five ### subsections (lines 181, 203, 234, 277, 301).
11. Fix history: README.md commit 12c4cfd (lint.disabled sentence). docs/readme_test.go has no fix commit.
12. docs/runs.md does not exist at 183da4e. The lead worktree has an uncommitted modified docs/runs.md on feat/spec-quality.

## Files the change touches
- README.md: edit.
- docs/readme_test.go: line limit, moved-heading checks. Use named constants for paths and the limit.
- docs/configuration.md:113 and :413, examples/README.md:61 and :110: repoint links.
- skills/harness/SKILL.md:30: section name text.
- New: docs/plugins.md, the run walkthrough doc, docs/contributing.md (layout, tests, bug report).
- Do not edit skills_test.go or pr_open_test.go. The ddd-depth and pr-visuals runs edit them.

## Review risks
- R1, doc drift (29 cases): moved anchors break 4 links and one text reference. Test: a link test that reads each `](...#anchor)` in README and docs and checks the target heading exists.
- R2, length: README over the limit. Test: line count of README is at most N. Fails at 522 lines.
- R3, lost text: a moved heading is missing from its new file. Test: each moved heading exists in its new doc. Fails on base.
- R4, prose (20 cases): a new intro or quick start sentence over 25 words. No checker reads docs prose. Review, plus an em dash test on new docs.
- R5, duplication (10 cases): `make check` and `gh auth login` appear in README and docs. Review: one owner for each command.
- R6, bare literal (9 cases): new paths and the limit as named constants.
- R7, production path: the tests read the real files with repository.Read, not fixtures.
- R8, documented step (3 cases): read-only checks only. `harness --help`, and `gh release view v<version> -R 90p-labs/harness` for the asset name.
- R9, guard row: review run 20261109-010358 found the README guard row missing tool.after and run.transition.before. No test reads that row. Test: each guard rule id appears in the moved row.

## Threat model
Not required. The change touches no guard, security, approval, permission or secret path.

## Open decisions (checker conflicts first)
1. design_test pins `Plan` question and `harness run approve --by` in README. Moving A full run breaks it.
   - a: keep one sentence for each in the everyday commands. Recommended, because approve is an everyday command and the test stays.
   - b: repoint the test to the new walkthrough doc.
2. readme_test pins all of Install (60 lines). Install must stay in README. The "about 150 lines" target needs Set up under about 40 lines.
   - a: target about 160 lines. Recommended. Move the full .harness.yaml and the verify output to docs/configuration.md.
   - b: target about 200 lines, keeping Set up nearly whole.
3. Quick start path. The plugin puts no harness on PATH (README:36), and the setup commands need it.
   - a: plugin install, then `/harness:harness status`. Recommended, because it needs no PATH.
   - b: go install, then harness init --host claude.
4. Walkthrough file name. docs/runs.md collides with the lead worktree change.
   - a: docs/run-walkthrough.md. Recommended.
   - b: docs/walkthrough.md.
5. Bug report steps.
   - a: a section in docs/contributing.md. Recommended.
   - b: a new docs/report-a-bug.md.
   - c: a pointer to skills/report/SKILL.md.
6. Moved headings: keep the same names, so the repoint is mechanical. Recommended.
7. Fix the "four examples" count in docs/configuration.md:113 during the repoint.
8. Sentence length on docs: review only (recommended), or a new test shared with skills_test.

## Advisor report

Advisor report: plan `readme-focus-plan.md`, base 183da4e. All paths are under `<scratch>/`.

Findings:

1. blocker, plan.md:17-134, the plan fails the checker, so `harness run plan` refuses it. Observed: `go run ./cmd/harness plan check --file ../replay/readme-focus-plan.md` exits 1 with 9 `sentence-length` (lines 17, 29, 44, 45, 47, 48, 94, 97, 100; line 94 has 71 words), 12 `tree-format` (lines 117-125, 132-134) and 2 `trace-unlinked`. `splitTreeLine` (`repo-readme/internal/plan/domain/parse.go:426`) needs the `├── ` or `└── ` connector. The plan indents with spaces. The tree lines can be a fault of the replay copy. The sentence errors are not.
2. major, plan.md:95, quick start step 2 cannot work, against prompt item 1. Observed: `gh api repos/90p-labs/harness/releases -q length` prints `0`; `gh release view v0.1.0` prints `release not found`. The line also has the placeholders `v<plugin version>` and `OS-ARCH`, and needs chmod and a move. Decide: build from source, or publish a release first.
3. major, plan.md:18, criterion 2 checks headings only. A worker can drop the text under a heading, and the proof passes. The prompt says "no loss of content" (same class as rework line 10). Inferred. Pin one distinct line for each moved section.
4. major, plan.md:92-102 against plan.md:17, the outline does not fit 160 lines. My tally is about 185: Install keeps about 53 lines verbatim, the first run about 30, the quick start about 25. Inferred. The worker must cut text that the plan does not name.
5. major, plan.md:90, `docs/configuration.md:413` keeps "The README gives the step by step setup", but that text moves to `docs/setup.md`. The link resolves and the sentence is false. Observed. `examples/go/README.md:93` has the same pointer, and no task owns that file.
6. major, plan.md (no line), the plan has no `## Review risks`. Unanchored risks: text loss, doc drift, the line budget, headings in fences.
7. minor, plan.md:37 against plan.md:49, `links_test.go` is green on the base (my scan: 43 links, 0 bad). TestRelativeLinksResolve never fails in this run; no step proves that it can fail.
8. minor, plan.md:28 and :37, no backticked test name. `testName` (`repo-readme/internal/plan/domain/trace.go:10`) matches nothing, so `test-missing` skips both tasks. Observed.
9. minor, plan.md:19 and :29, "resolves to a file" rejects a folder link. The base has 8 folder links, and the outline links `docs/adr/`. Observed.
10. minor, plan.md:29, the heading scan for anchors does not skip code fences. `docs/go-live.md` has 1 heading inside a fence (rework line 1). The slug rule omits the `-1` suffix of a repeated heading.
11. minor, plan.md:96, README.md:21 (decision trees, the `pr-open` body, when the hooks act) has no destination page.
12. minor, plan.md:99, the commands table omits `harness run review` and `harness run ship`. The prompt names review and ship.
13. minor, plan.md:97 and :84, the `harness init` console example stays in the README and also moves to `docs/setup.md`: two copies.
14. minor, size: about 1,100 changed lines (a move counts twice), above 800. The move is mechanical and has 2 tasks. Record a decision.

Checks with no finding:
- `cells` (`parse.go:188`) restores `\|`, so the criterion 1 proof runs its tests.
- `go test ./docs/ -list` shows that TestReadmeIsShort, TestMovedSections and TestRelativeLinksResolve are absent on the base.
- The plan keeps all README pins: `pr-open`, the `Plan` question, `approve --by`, the configuration-full link, the Install subsections.
- No other file links a README anchor, except the 3 that the plan names.
- Each command of the outline exists. I did not run `harness gates init`, so its output at plan.md:107-110 is not verified.

Verdict: BLOCKED
