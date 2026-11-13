# The panel

The panel is a local web page that shows the plans and the runs of every repo that used it. One panel serves each user. This guide covers the background commands, the `panel:` config block, the state files and the host open.

## Commands

| Command | Output | Effect |
|---|---|---|
| `harness panel start [--port N]` | `panel: <url>` | Runs the panel in the background. A panel that already runs returns its URL at once. |
| `harness panel stop` | `panel: stopped` | Ends the background panel. |
| `harness panel status` | `panel: <url> pid <n>`, or `panel: stopped` with exit 1 | Prints the running panel. A script can test the exit code. |
| `harness panel serve` | `panel: <url>` | Runs the panel in the foreground. |

`--port` sets the port of the panel. Without it, the `panel.port` key decides the port, and `0` lets the system pick a free port. `status` takes no `--port`.

```sh
harness panel start
# panel: http://127.0.0.1:52811/
harness panel start
# panel: http://127.0.0.1:52811/
harness panel stop
# panel: stopped
```

The second start prints the same URL, and it starts no second server.

## Running or stopped

The panel is running only when its lock is held and `/healthz` answers the pid of `panel.json`. A stale `panel.json` from a crashed server does not count as running. Any repo can ask, so one panel serves them all.

`start` waits up to about 5 seconds for the panel to run. If it does not, `start` fails and prints the last lines of `panel.log`.

`stop` sends SIGTERM only to the pid that `status` verified. It waits up to about 10 seconds for the lock to free. If the server does not stop in that time, `stop` fails and keeps `panel.json`. A `panel.json` whose lock is free is removed, and no signal is sent.

The server binds to `127.0.0.1` only. Its URL is `http://127.0.0.1:<port>/`.

## State files

The state files sit in the user state folder. It is `$HARNESS_HOME` when that is an absolute path. Otherwise it is `~/.harness/`. One panel serves every repo of the user, and git does not track these files.

| File | Content |
|---|---|
| `panel.json` | The pid, the port and the version of the running server. It is written after the server listens, and removed on exit. |
| `panel.lock` | The lock of the running server. The server holds it while it runs. |
| `panel.log` | The output of the background server. |
| `repos.json` | The repos that started or used the panel. Each entry has the key, the name, the common dir, the worktree, the state file and the last use time. |
| `repos.lock` | The lock of `repos.json`. Each write holds it. |

The files have mode `0600`. The folder has mode `0700`, and the panel sets that mode on an existing folder too. A relative `HARNESS_HOME` is an error, since it would put the files in the current folder.

## Repos and routes

`harness panel start`, `harness panel serve` and the host open add the repo they run in to `repos.json`. `status` and `stop` add nothing. A repo key is the first 12 hex digits of a sha256 over the absolute git common dir. Two clones of one origin get two keys.

The list drops a repo whose common dir or sqlite state file is gone. It logs `panel: dropped repo <path>` on the panel log.

The board has one set of six columns with the cards of every repo. Each card has a chip with its repo name. The newest run comes first. Done keeps the 20 most recent cards of each repo.

A repo list shows "All repos" and each repo with its count and a mark when it waits for approval. The repos sort by their newest run. The counts ignore the filter. On a phone the repos show as a chip row. A problem notice list sits above the columns.

The filter `/?repo=<key>` shows only that repo. A malformed key answers 404, and an unknown key redirects to `/`. Each run has its own page at `/repos/<key>/runs/<id>`. An old `/runs/<id>` link redirects there when exactly one repo has the run. Otherwise it answers 404. It also answers 404 while any repo fails to load, since a unique match cannot be known.

Each repo reads only the `.harness.yaml` of its repo. A spawned panel (`panel start`, autostart, host open) reads every repo from its file only. Only a hand-run `harness panel serve` keeps the `HARNESS_*` env variables, for its own repo.

A repo whose state DSN is set only by env or by a flag shows its default store, or the problem line `state.db not found`.

The spawned panel drops the git location variables of the start shell, such as `GIT_DIR`. Each repo is then found from its worktree. It keeps every other variable. A repo whose worktree belongs to another repo shows the problem line `worktree <path> belongs to another repo`.

A hand-run `panel serve` keeps its shell's variables. Under a `GIT_DIR`, a repo whose entry names another common dir shows the problem line `worktree <path> belongs to another repo`. The `GIT_DIR` repo shows its runs under the name and worktree its entry recorded.

## Old panels

A panel of the old layout runs from the `.harness` folder under the git common dir of the repo. `start` and `stop` stop that panel, then remove its `panel.json`, `panel.lock` and `panel.log`.

A failed stop warns on start with `panel: could not stop the old panel in <dir>: <err>`. The files stay, and the start goes on. `stop` returns the error.

A start that finds another version prints `panel: running version <v>, run harness panel stop to switch to <w>`. It then prints the URL, reuses the panel and exits 0.

## The `panel:` block

The `panel:` block of `.harness.yaml` turns on the autostart and the host open. It is off by default. See [configuration](configuration.md) for each key.

```yaml
panel:
  enabled: true
  port: 0
  autostart: true
  open: host
```

- `panel.enabled` is the master switch. With `false`, the autostart and the host open are off. The explicit `harness panel` commands work with any value.
- `panel.autostart: true` starts the panel after `harness run start` succeeds. It prints `panel: <url>`. A failed start prints `panel: start failed: <err>` on stderr, and the run goes on.
- `panel.open: host` prints the run URL on stderr when a plan waits for approval: `panel: <url>repos/<key>/runs/<id>`. It opens no browser. The harness skill opens that URL in the host preview.

## The host open

With `panel.enabled: true` and `panel.open: host`, the plan approval prints `panel: <url>repos/<key>/runs/<id>` on stderr instead of opening the browser. If the panel cannot start, the harness prints `panel: start failed: <err>` and `panel: opening the browser`, then opens the plan in the browser.

`harness plan view --open` stays browser only. It does not use the host open.

## Phone access

The server binds to `127.0.0.1`, so another device cannot reach it. To open the panel on a phone, run a tunnel that you control to the port. The harness does not start a tunnel.
