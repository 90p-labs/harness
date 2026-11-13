# 0023 The served control panel

## Status

Accepted, 2026-11-13. It adds an optional served mode to the plan view. It does not change the file mode of ADR 0010 or the render rule of ADR 0021.

## Context

- The plan page is a file. A reader reloads it by hand to see a change.
- The user wants an optional local panel. It shows a kanban board of runs and a live page for each run.
- A local server must not open the runs to the network or to other sites.
- The file mode must stay offline, with no server, as ADR 0010 requires.

## Decision

1. The served mode is optional. It runs behind `harness panel`. The file mode and the other commands work with no server.
2. The server uses the Go standard library: `net/http` with the Go 1.22 `ServeMux` patterns, `html/template` and `go:embed`. It uses no framework and no Node build.
3. The server binds 127.0.0.1 only. It checks the Host header against DNS rebinding.
4. htmx, its SSE extension and idiomorph are vendored in `internal/shared/infrastructure/webkit/third_party/`. Their pinned versions and checksums are in `NOTICE.md`. No page loads a CDN.
5. The file mode `plan.html` stays self-contained and offline. It stays byte for byte the same. It loads no htmx.
6. Use htmx for every call to the server. Use plain JavaScript for every other behaviour.
7. The page splits into named regions, with `{{define "plan/<name>"}}` blocks. The full page and a fragment render from the same block.
8. A registry lists the regions. The registry is the route whitelist. A name that is not listed gets no route.
9. Server-sent events carry only signals. They do not carry the page content.
10. Each region fetches its fragment with a version. The server answers 204 when nothing changed. The version has a plan part and a run part. The run part hashes the status version, the branch head and the proofs of the active run. It replaces `PRAGMA data_version`, which changes on any write to the state, also a guard call, and would refetch every region of every tab. The adapter caches the head and the proofs of each run for one second.
11. Polling every 5 s is the fallback when the events stop.
12. Approve and comments from the page are out of scope. They need their own ADR.
13. The background panel is started with `harness panel start` and ended with `harness panel stop`. Start is idempotent, and a running panel is one whose lock is held and whose `/healthz` answers the repo and the pid of `panel.json`. ADR 0027 amends this item: the health answer checks the pid only.
14. The state files `panel.json`, `panel.lock` and `panel.log` sit in `<git common dir>/.harness/` with mode 0600. Stop signals only a verified pid, and removes a stale `panel.json` only when the lock is free. ADR 0027 amends this item: the files sit in the user state folder.
15. The `panel:` block turns on the autostart and the host open. `panel.enabled` is the master switch. The host open prints the run URL on stderr, and the browser opens only as a fallback.

## Consequences

- The shared web kit holds the fonts, the mark tokens and mermaid in R1. A later run adds the base CSS, the icons and the shell JavaScript.
- A golden hash test holds the bytes of the file mode. A change to the file mode bytes fails the test.
- The served mode has a second render path, the fragments. Both paths must stay in step with each other.
- The vendored scripts need a review when their versions change. The checksums in `NOTICE.md` show what changed.
- The panel reads the local runs only. It is not a remote service.
