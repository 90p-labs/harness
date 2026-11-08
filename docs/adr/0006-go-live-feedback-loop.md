# 0006 Go-live feedback loop

## Status

Accepted, 2026-11-07. Updated 2026-11-08.

## Context

Kodify ends the run at the PR. Production signals never return to the loop. The project `jorge07/billing-api` shows the direction: metrics, monitoring and a closed feedback loop.

## Decision

- The loop follows the change to production. Metrics and monitoring drive it.
- A `signals.check` plugin reads one signal source and denies on a breached threshold. The joined deny reasons become the regression findings.
- A regression sends the run back to `planning` with the findings.
- Each gate has an input hash of the files that match its `paths`. A docs change does not run the Go tests again.
- The deploy marker is the GitHub deployment status of the merge SHA. The harness never runs the rollback.
- No signal data for one more window after the watch window blocks the run.

## Consequences

- The run closes as `done` only after green signals for the whole watch window.
- The live module treats a plugin failure as a check not done and tries again.
- A rollback stays with the deploy system of the user.
- A silent signal source blocks the run, so a broken monitor cannot pass as healthy.
