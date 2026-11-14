# 0027 One panel per user

## Status

Accepted, 2026-11-13. It amends items 13 and 14 of ADR 0023. It does not change the served mode, the file mode or the Host guard of ADR 0023.

## Context

- Each repo ran its own panel. Its state files sat in its git common dir.
- A user with several repos ran several panels, each on its own port.
- The board showed the runs of one repo only.
- The user wants one panel for every repo that started or used it.

## Decision

1. One panel serves each user. Its state files sit in the user state folder, not in the git common dir.
2. The user state folder is `$HARNESS_HOME` when that is an absolute path. Otherwise it is `~/.harness/`.
3. A relative `HARNESS_HOME` is an error. It would put the state files in the current folder.
4. The folder has mode 0700, and an existing folder is chmod to 0700. The files have mode 0600.
5. The state files are `panel.json`, `panel.lock`, `panel.log`, `repos.json` and `repos.lock`.
6. A repo key is the first 12 lowercase hex digits of a sha256 over the absolute git common dir. Two clones of one origin get two keys.
7. `repos.json` lists the repos. Each entry holds the key, the name, the common dir, the worktree, the state file and the last use time.
8. `panel start`, `panel serve` and the host open register the repo they run in. A start registers the repo and stops a legacy panel even when it finds a panel of another version (item 12). Status and stop register nothing. A serve whose working folder is the user folder registers nothing, since the spawned serve runs there.
9. The registry is written under a flock on `repos.lock`, through a temp file and a rename.
10. The registry drops a repo whose common dir or sqlite state file is gone. A missing worktree falls back to the main worktree.
11. A running panel is one whose lock is held and whose `/healthz` answers the pid of `panel.json`. Any repo may ask. A start from another repo reuses the panel.
12. A start that finds a panel of another version still stops a legacy panel and registers the repo. It then prints a warning line and reuses the running panel.
13. The board shows one set of six columns, and each card has a chip with its repo name. A repo list shows every repo with its count, and a phone shows it as a chip row. A problem notice list sits above the columns. The filter `/?repo=<key>` shows only that repo. A malformed key answers 404, and an unknown key redirects to `/`. Each repo keeps its own Done limit.
14. The run routes name the repo: `/repos/{key}/runs/{id}`. The event stream is `/events?repo=<key>&run=<id>`.
15. The server checks the key before any store call. A bad key or an unknown run answers 404.
16. An old `/runs/{id}` link answers 301 to the repo route when exactly one repo has the run. Otherwise it answers 404. It also answers 404 while any repo fails to load, since a unique match cannot be known.
17. The host open prints `panel: <url>repos/<key>/runs/<id>`.
18. Start and stop also stop an old panel in `<git common dir>/.harness/`, then remove its files. A failed stop warns on start and leaves the files. Stop returns the error.
19. The loopback bind and the Host guard of ADR 0023 stay. Registry paths come from git, never from a request.
20. Each repo reads only the `.harness.yaml` of its repo. A spawned panel applies no `HARNESS_*` variable and no serve flag. Only a hand-run `panel serve` keeps its `HARNESS_*` variables, for its own repo. A repo whose state DSN is set only by env or flag shows its default store, or `state.db not found`. The spawned panel drops the git location variables of the start shell, such as `GIT_DIR`, so each repo is found from its worktree. A repo whose worktree belongs to another repo shows the problem line `worktree <path> belongs to another repo`.
21. A hand-run `panel serve` keeps its shell's variables. Under a `GIT_DIR`, a repo whose entry names another common dir shows the problem line `worktree <path> belongs to another repo`. The `GIT_DIR` repo shows its runs under the name and worktree its entry recorded.

## Consequences

- A user runs one panel, and one URL shows the runs of every repo.
- A user who keeps two checkouts of one origin sees two repos on the board, one for each key.
- A moved or deleted repo leaves the registry on the next list, and the board stays up.
- An old panel of the per-repo layout stops on the first start or stop after the upgrade.
- The state folder sits outside the repos, and git does not track it. Mode 0700 keeps it private to the user.
- A repo that is not yet in the registry shows no cards until its first start.
