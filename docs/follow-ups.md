# Follow-ups

This file lists the open follow-ups of the learn step. Each entry holds a prompt that nobody has started yet. An empty section `## Open` means that no follow-up is open.

The rules and the form of an entry are in `skills/harness/references/learning.md`, section Follow-ups. A repeat of a cause adds one `Cited by` line to its entry. The run that fixes the cause removes the entry.

## Open

### `harness run review` reads its diff base from the plan, so a stale local `main` breaks the review

- Category: Information access
- Cited by: `20261114-155717-give-a-follow-up-lesson-of-the-learn-ste`, `handoff.md:27`
- Prompt: `harness run start "Make harness run review use the same diff base as harness verify --commits. Today the review takes the base of the PR sequence line of plan.md, which the plan template writes as main, and resolves it to the local branch. A local main behind origin/main then lists the files of other merges as untracked. Remove the entry of this cause from docs/follow-ups.md."`

### A doc test repeats a string literal that a constant of its package already holds

- Category: Convention
- Cited by: `20261114-155717-give-a-follow-up-lesson-of-the-learn-ste`, `findings.md:26`
- Prompt: `harness run start "Add a lint rule, with its canary, that fails when a Go test file repeats a string literal that a constant of its package already holds. One run met it five times, in the findings F14, F25 and F38 to F40. Remove the entry of this cause from docs/follow-ups.md."`

### An order check of a doc test passes when an anchor text is missing

- Category: Convention
- Cited by: `20261114-155717-give-a-follow-up-lesson-of-the-learn-ste`, `findings.md:25`
- Prompt: `harness run start "Add one helper to the doc tests under docs/ that fails the test when an anchor text is missing. Use it in each order check in place of a direct strings.Index call, and add a check that fails on a new direct call. One run met the cause in the findings F13 and F26. Remove the entry of this cause from docs/follow-ups.md."`
