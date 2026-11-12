# Replay: r2-guard-precision

- Run: 20261109-222739-make-the-guard-deny-only-what-it-must-an
- Base: c85583c
- Prompt: the original prompt of the run. Rework reasons: only those recorded before the run start (54 lines).
- Explorer: haiku, brief agents/explorer.md blob 7a4eaf0f013b7be6e101654a001256a7525df75d, one sample.
- Advisor: fable (agent type 90p:advisor, told to follow the new brief file), brief agents/advisor.md blob 2cdd7b856e726873a85edbba675089b5231fb028, one sample, on the first plan draft of the run.
- Redaction: the scratchpad prefix is written as <scratch>/.

## Explorer report

Explorer report, run R1 (repo-r2, HEAD c85583c). Baseline: go test passes for internal/guard/..., skills/pr-open, docs and cmd/harness.

Facts
1. The parser drops quote kind and separators. `$` stays literal in quotes (shell.go:194-223). Command holds no assignments or separators (shell.go:8-13). A pipe flushes like `&&` (shell.go:61-64).
2. programArgs drops `NAME=value` (rule.go:106-107) and the `-C` of `env` and `git` (rule.go:47-48, 110-111, 119-127).
3. Only TextFiles follows `cd`, with two args and one folder per line (rules_text.go:42-59). isSourcePath uses rc.Cwd and lexical Clean (rules_files.go:121-136).
4. writeTargets covers redirects, tee, sed -i, perl -i, cp/mv last operand (rules_files.go:138-177). `-t` and `--target-directory` are not handled.
5. checkBody and checkSections return the first problem and keep no line number (rules_pr.go:92-153). maxBodyWords is 40 (rules_pr.go:23).
6. agentRoles maps general-purpose to explorer and has no claude entry (rules_agent.go:17-28). IsReviewerAgent reads the same map (rules_review.go:17-19).
7. Rows that repeat the rules: DESIGN.md:206, 212; docs/configuration.md:205, 245, 251; skills/pr-open/SKILL.md:89. No test reads the agent-name table.
8. Existing want strings: rule_test.go:352-377 ("40 words", "no mark", "no bottom line", "## What is before ## Why").
9. harness plan check prints each problem and fails on any (internal/plan/ui/cli/check.go:93-104).

Checker conflict (first)
C1. SKILL.md:84 says a bottom line is one or two sentences. isBottomLine accepts any text line (rules_pr.go:155-157). Options: (a) reword the skill to one line; (b) add a sentence rule to pr check. Recommend (a).

Open decisions
D1. Body checker home. (a) Move it to internal/shared/domain/pr.go, next to CheckMarks. (b) Export it from guard/domain. Recommend (a).
D2. Guard deny text. (a) First problem, plus the count and `harness pr check`. (b) All problems joined. Recommend (a).
D3. Unresolved variable. Option: deny only when the literal ends in a source extension or has none. Recommend this; the replay counts decide.
D4. A cd before a pipe runs in a subshell. Recommend: a cd in a pipeline makes the folder unknown.
D5. Prefix assignment `D=/tmp cmd > $D/a.go`: the redirect uses the outer D. Recommend: deny.
D6. Unknown folder forms: env -C, git -C, pushd, popd, cd with no arg, `cd -`, `cd ~`. Recommend: relative targets deny.
D7. `cp -t /work/repo /tmp/x.go` is allowed today. Recommend: handle -t and --target-directory now.
D8. Agent mapping. (a) general-purpose and claude to worker. (b) general-purpose stays explorer, claude unchecked. Recommend (a), never reviewer, because IsReviewerAgent shares the map. Update configuration.md:205.
D9. Symlink into the repo passes the lexical check. Recommend: list as a known gap.
D10. Writers outside the list (python -c, dd of=, install, truncate, bash -c, eval). Recommend: keep allowed, list as gaps. Fail closed on `>|` and `>&name` (shell.go:104-112; read, not run).

Threat model
Attacker: an honest agent that wants its scratch write to pass.
Input forms (closed list):
- F1 redirect operand: position after the operator; folder is cwd.
- F2 tee operands: position; cwd.
- F3 sed -i or --in-place: each non-flag operand; cwd.
- F4 perl -i or -pi: as F3.
- F5 cp or mv: last operand, or each source in a folder form; flags -t, --target-directory; cwd.
- F6 folder: envelope cwd, cd argument, env -C, git -C, pushd, popd.
- F7 text: NAME=value, $NAME, ${NAME}, $(...), backticks, ~, glob characters.
Fail closed: an unresolved F6 or F7 value denies with "use a literal absolute path". Forms outside the list keep today's verdict (D10).

Review risks (each with its test)
- F1 to F5: one cd case and one variable case each; each fails on base.
- F5: `cp -t /work/repo /tmp/x.go` denies (D7).
- F6: `cd /tmp | echo x > main.go` denies (D4).
- F7: `D=/work/repo; echo x > $D/main.go` denies; `D=/tmp; echo x > $D/a.go` allows.
- Doc drift: DESIGN.md:212 and configuration.md:251 keep their meaning. Test: docs test reads both.
- Pr check and guard must share one checker. Test: both call it on one body.
- Replay test must run through guardplugin Handle with an envelope cwd (plugin_test.go:98), not the internal functions.
- Replay JSONL goes in testdata. Feed it to Commands only; never run it in a shell.
- Bare literals: the reason text, -t, --target-directory, pushd and -C need named constants. cmdCd exists (rules_text.go:14).
- Add `harness pr check` to prOpenSteps (skills/pr-open/pr_open_test.go:19).
- Keep the want substrings at rule_test.go:352-377.

Not done: the ddd skill path was not given, so no layer review was run.

## Advisor report

Advisor report: plan `r2-plan.md` (guard precision), base `c85583c`.

1. blocker | plan.md:20, plan.md:23 | A bare `|` in the proofs `'TestRuleMatches|TestDenyReplay'` and `'TestPRBodyShape|TestBodyProblems'` splits the row into 4 cells. Lines 23 and 84 also hold sentences of 32 and 34 words. `harness run plan` refuses the plan. | observed | `go run ./cmd/harness plan check --file r2-plan.md` prints `plan.md:20: criteria-table: row has 4 cells, want 3`, `plan.md:23: sentence-length: sentence has 32 words`, `plan.md:84: sentence-length: sentence has 34 words`, `error: plan has problems`, exit 1. Fix: write `\|` (`internal/plan/domain/parse.go:191`).

2. major | plan.md:62, plan.md:85 | Bypass. The plan joins an unresolved target with the walked folder. `W=$PWD; cd /tmp/s && echo x > $W/main.go` gives `/tmp/s/$W/main.go`, so the rule allows it, and the shell writes the worktree. Today `rules_files.go:125-126` joins with `rc.Cwd` and denies. The same defect applies to a loop variable, a backtick value (the plan tests only `$`), `~`, a `cd` with 3 arguments, and a relative `cd` after an unknown `cd` ("last known folder"). | inferred | Code reading. Fix: check an unresolved target against the session folder, and keep "unknown" until an absolute `cd`. Add one test for each form.

3. major | plan.md:67 | The testdata fails criterion 13. `internal/repo_names_test.go:15` refuses the old repository name (owner, slash, `harness`) in each tracked file. 282 of 284 replay rows hold it, and the decision replaces only the home folder name. | observed | Python count on the JSONL; test source. Also: 2 rows hold a personal email, 14 rows have a `cwd` that is not the checkout root (no worktree field stays), and 170 rows (line 35) is not a sample.

4. major | plan.md (whole) | No `## Review risks`, no `## Threat model`, and no list of the relaxed cases that prompt line 15 requires. Missing cases: a failed `cd`, an assignment after `&&`, a `cd` into a sibling checkout, a subshell `(cd` (5 rows). | observed

5. major | plan.md:15-19, 22, 25 | The proofs of criteria 1 to 5, 8 and 11 pass on the base. | observed | `go test ./internal/guard/domain -run 'TestShellCommands|TestRuleMatches' -count=1` gives `ok`; `go test ./skills/... -count=1` gives `ok`. No task owns `skills/pr-open/pr_open_test.go` (criterion 11) or `internal/guard/application/guardplugin/plugin_test.go`, which holds the only `TextFiles` test (criterion 8).

6. major | plan.md:107 | `BodyProblems` covers the shape only. `writing-style` also reads `--body-file` (`rules_text.go:24-26`). `harness pr check` prints `ok`, then the guard denies an em dash. This contradicts "every PR body problem". | inferred

7. minor | plan.md:65 | `claude` to `worker` adds a new deny (model `opus` or no model), against "deny only what it must". The case name "explorer model" at `rule_test.go:265` becomes wrong, and task 3 does not own that file. The docs part of task 3 DoD has no test.

8. minor | plan.md:64 | The data has 27 `python` deny rows (4 in the `guard.` period), not 23. Criterion 7 says "each row".

9. minor | plan.md:22 | The cited source (section 3, step 2) wants "pass a literal absolute path" for an unknown `--body-file` variable. No criterion covers it.

Size: 3 tasks, about 600 code lines. No split is necessary. The `ui/cli` import of the domain passes `internal/architecture_test.go`.

Paths: `<scratch>/replay/r2-plan.md`, `<scratch>/repo-r2`.

Verdict: BLOCKED
