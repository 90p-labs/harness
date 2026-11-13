# 0021 The plan page follows the run

## Status

Accepted, 2026-11-11. It changes the render rule of ADR 0010, and it makes the approval on the page a reply to the agent. Amended by [ADR 0023](0023-served-control-panel.md): the served mode.

## Context

- ADR 0010 wrote the plan page only when a run entered `awaiting_approval`. A human who opened the page during `working` or `reviewing` saw no state of the tasks.
- The page named a CLI step for the approval. The human had to leave the chat to run `harness run approve`.
- The page had no task state, so the human could not see which commits of a task were on the branch.

## Decision

1. The view plugin writes `plan.html` after every `run.transition.after`, in every phase. A render error never blocks the transition. The plugin opens the page in the browser only when the new phase is `awaiting_approval`. With `panel.open: host` the plugin prints the run URL of the panel instead, see [ADR 0023](0023-served-control-panel.md), and the browser stays the fallback.
2. The page shows the phase as a status label, such as "Waiting for approval" or "Building". Each task shows open or committed, from the commits of the branch.
3. In `awaiting_approval` the approval is a reply to the agent. Approve copies "Plan approved. Go ahead with the build." Send corrections copies the corrections as a list.
4. The human pastes the copied reply into the chat. The page never runs a command and shows no CLI step.
5. The approve button reads "Approve" with a key hint for Cmd+Enter. When the reply box holds a draft, Approve first asks for a confirm.

The `Plan` question and `harness run approve` keep their rules. The page prepares the reply, and it does not record an approval.

## Consequences

- The human sees the current plan and the state of each task in every phase, with no new command.
- The page is rewritten after each transition. It is one file, so disk use does not grow with the transitions.
- The approval from the page reaches the agent only when the human pastes the reply. A reply that is not pasted leaves the run in `awaiting_approval`.
- A human with no chat cannot approve from the page. The CLI approval still works.
