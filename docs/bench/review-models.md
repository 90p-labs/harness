# Bench: review with one model against review with two lenses

This page compares two ways to review a change. Side A is the review before this change. Side B is the review after it. All values are measured on 2026-11-09, unless a line says otherwise.

- Side A: one `reviewer` agent, with no model in the Agent call, so it inherits the lead model `claude-opus-5-5`.
- Side B: a `Spec` reviewer on sonnet and a `Standards` reviewer on haiku, in parallel. A third `Spec` reviewer on opus joins when the diff changes more than 400 lines or the plan names a state, concurrency or security risk.

## Method

- The bench reviews the merged PRs 33, 36, 37 and 38 of this repository. Each review runs in a git worktree at the merge commit, with `git diff <first parent>...HEAD`.
- No PR has a `plan.md`. Each reviewer gets the PR body as the plan.
- Each review runs in a new agent. No agent sees the result of another agent.
- The 4 first side A runs, one for each PR, ran together before the plan, not alternated with side B. The 10 side B agents of those PRs also ran together. Only the PR 38 runs alternate.
- Side B gets the text of the new `agents/reviewer.md` in its prompt, because the plugin of this session still loads the old agent file.
- The third reviewer joins on PR 33 (932 changed lines) and on PR 38 (its body names a race).
- Tokens and cost come from the usage block of each request in the agent transcript. The Agent tool total is not the billed total, so the page does not use it.
- Wall time of a side is the time of its slowest agent, from the first to the last transcript entry.
- A blind check finds the real defects. One opus agent for each PR gets the union of the findings of both sides, shuffled, with no side label. It marks each finding CONFIRMED, REJECTED or UNVERIFIED, with a quoted line or a command and its output. It also groups the findings that name the same defect.
- A "distinct defect" is one confirmed group. The union of both sides gives the total.
- Each reviewer ran `git diff` with content on its base. The transcript of each agent shows this.
- No reviewer returned an empty result. So the error count is 0 on both sides.
- Limit: the session guard denied some probes. It denied a scratch `.go` file write to B38H, SA1 and SB1O, and a scratch push to SA1. These reviewers then marked a finding inferred, not observed.
- Limit: side A ran first in each worktree. Its `harness verify` ran the Go tests from cold. Side B got the cached result. This lowers the time of side B.

### Repeated runs on PR 38

PR 38 has the smallest diff. It gets alternated runs, A then B, 5 for each side. The other PRs get one run for each side, because a side A run costs 0.59 to 1.64 USD and 4.5 to 13 minutes.

## Load

`uptime` on the host before each batch. The host is a laptop that runs other sessions.

| Time (UTC) | Batch | Load (1, 5, 15 min) | Other agents at the same time |
|---|---|---|---|
| 00:05:19 | side A, 4 PRs | 3.41 4.60 6.61 | the 4 side A reviewers |
| 00:29:59 | side B, 4 PRs | 7.98 6.36 5.70 | the 10 side B reviewers and 1 worker |
| 00:35:16 | series A1 | 9.48 8.19 6.73 | 2 side B reviewers, 4 verifiers, a per-commit check |
| 00:41:17 | series B1 | 6.45 8.81 7.75 | 2 verifiers |
| 00:45:28 | series A2 | 4.97 6.95 7.24 | none |
| 00:49:45 | series B2 | 4.09 5.37 6.45 | none |
| 00:51:50 | series A3 | 5.56 5.52 6.36 | none; the host slept during the run |
| 09:04:41 | series B3 | 9.91 14.35 10.53 | none; the load fell from 47.20 after the host woke |
| 09:09:17 | series A4 | 5.75 9.45 9.34 | none |
| 09:11:16 | series B4 | 4.80 8.18 8.87 | none |
| 09:14:55 | series A5 | 5.21 6.69 8.09 | none |
| 09:17:45 | series B5 | 5.64 6.74 7.89 | none |

## Prices

List prices of the first-party API in USD per million tokens. Source: the model table of the `claude-api` skill, cached 2026-10-06.

| Model | Input | Output | Cache read |
|---|---|---|---|
| `claude-opus-5-5` | 4.00 | 20.00 | 0.20 |
| `claude-sonnet-5-5` | 2.00 | 10.00 | 0.20 |
| `claude-haiku-5-5` | 0.10 | 0.50 | 0.01 |

- A haiku request with a prompt over 100,000 tokens costs 0.50 input and 2.50 output. Its cache read then costs 0.05, which is 0.1 times the input price. The script prices each request on its own prompt size.
- A cache write costs 1.25 times the input price for the 5 minute cache and 2 times for the 1 hour cache. The script reads the split from each request.

## Results

### One run for each side on each PR

| PR | Side | Wall (s) | Tokens | Cost (USD) | Findings | Confirmed | Distinct defects | Majors |
|---|---|---|---|---|---|---|---|---|
| 33 | A | 769 | 2,070,046 | 1.6434 | 5 | 3 | 3 | 1 |
| 33 | B | 385 | 2,676,333 | 1.1831 | 13 | 5 | 3 | 1 |
| 36 | A | 387 | 1,410,902 | 1.1766 | 7 | 2 | 2 | 1 |
| 36 | B | 79 | 470,519 | 0.1524 | 6 | 1 | 1 | 0 |
| 37 | A | 321 | 935,264 | 0.9391 | 3 | 3 | 3 | 0 |
| 37 | B | 68 | 211,998 | 0.1049 | 3 | 1 | 1 | 0 |
| 38 | A | 274 | 387,437 | 0.5877 | 5 | 3 | 3 | 1 |
| 38 | B | 234 | 1,062,330 | 0.6591 | 7 | 4 | 3 | 1 |
| All | A | 1,751 | 4,803,649 | 4.3468 | 20 | 11 | 11 of 15 | 3 of 3 |
| All | B | 766 | 4,421,180 | 2.0995 | 29 | 11 | 8 of 15 | 2 of 3 |

Side B by model:

| Model | Reviews | Tokens | Cost (USD) | Findings | Confirmed | Defects no other side B model found | Defects no other reviewer found |
|---|---|---|---|---|---|---|---|
| sonnet (Spec) | 4 | 268,014 | 0.4077 | 8 | 3 | 1 | 1 |
| haiku (Standards) | 4 | 2,673,124 | 0.1067 | 13 | 2 | 1 | 0 |
| opus (Spec, third) | 2 | 1,480,042 | 1.5851 | 8 | 6 | 3 | 1 |

Side A is one model, `claude-opus-5-5`, so the table above gives its tokens.

### Repeated runs on PR 38

Each side B run has all 3 reviewers, because the PR 38 body names a race. The major defect of PR 38 is a `--tags` push that skips the commit check (G1). The blind check confirmed it in the batch.

| Run | Wall (s) | Tokens | Cost (USD) | Found G1 |
|---|---|---|---|---|
| A1 | 349 | 615,179 | 0.7266 | yes |
| B1 | 211 | 605,837 | 0.6197 | yes (opus) |
| A2 | 247 | 454,619 | 0.5852 | yes |
| B2 | 98 | 576,678 | 0.4884 | yes (opus) |
| A3 | excluded | 292,683 | 0.8429 | no |
| B3 | 168 | 849,589 | 0.5871 | yes (opus) |
| A4 | 109 | 270,859 | 0.4578 | yes |
| B4 | 194 | 439,611 | 0.4912 | no |
| A5 | 161 | 296,880 | 0.4080 | yes |
| B5 | 224 | 724,508 | 0.5905 | yes (opus, haiku) |

- The host slept during A3: its transcript has a gap of 29,234 s. The page leaves out its wall time. Its cache expired in the gap, so its cost is high.
- A1 ran while other agents ran (see Load).
- Each reviewer ran `git diff` with content on the base. No reviewer returned an empty result, so the error count is 0.

| Measure | Side A median (range) | Side B median (range) | Sonnet and haiku only, median (range) |
|---|---|---|---|
| Wall (s) | 204 (109 to 349), 4 runs | 194 (98 to 224) | 86 (53 to 163) |
| Tokens | 296,880 (270,859 to 615,179) | 605,837 (439,611 to 849,589) | 299,999 (186,074 to 614,110) |
| Cost (USD) | 0.5852 (0.4080 to 0.8429) | 0.5871 (0.4884 to 0.6197) | 0.0895 (0.0612 to 0.1198) |
| Found G1 | 4 of 5 | 4 of 5 | 1 of 5 |

- Wall time and cost: the gap between side A and side B is smaller than the range. So the series shows no measurable difference.
- Tokens: the gap between the medians (308,957) is smaller than the side A range (344,320). So the series shows no measurable difference in tokens either.
- By model, G1 was found by opus in 4 of 5 runs, by haiku in 1 of 5 runs and by sonnet in 0 of 5 runs. The batch adds 1 run of each: opus found G1, sonnet and haiku did not.
- The last column is not a run. It is the sum of the sonnet and haiku agents of each side B run, and it shows the cost of side B with no third reviewer.

## Conclusion

- On PR 36 and PR 37 no third reviewer joined. There side B cost 87% and 89% less than side A and took 80% and 79% less wall time. It also found fewer defects. On PR 36 it found 0 of the 2 defects that side A found, and 1 defect that side A did not find. It missed the major defect of PR 36. On PR 37 it found 1 of the 3 defects that side A found.
- On PR 33 and PR 38 the opus reviewer joined. There side B cost 28% less on PR 33 and 12% more on PR 38. On the PR 38 series, the cost and wall time show no measurable difference.
- Over the 4 PRs, side B cost 52% less and took 56% less wall time. It found 8 of 15 distinct defects and 2 of 3 majors. Side A found 11 of 15 and 3 of 3.
- The opus reviewer carries the majors. Sonnet and haiku alone found the PR 38 major in 1 of 6 runs. Haiku gave the most findings that the blind check did not confirm: 11 of its 13 batch findings.
- So two cheap models do not replace one strong reviewer on a risky change. The rule for the third reviewer (diff over 400 lines, or a named state, concurrency or security risk) decides the quality. This change keeps that rule.
- Follow-up for the user: decide whether the third reviewer joins on every change. On this sample that removes most of the cost gain. ADR 0017 answered it with the pre-ship panel. ADR 0018 replaced that panel with the final panel.
