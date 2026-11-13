# The panel

The panel is a local web page that shows the plans and the runs of the repository. This guide covers the background commands, the `panel:` config block, the state files and the host open.

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

The panel is running only when its lock is held and `/healthz` answers the repo and the pid of `panel.json`. A stale `panel.json` from a crashed server does not count as running.

`start` waits up to about 5 seconds for the panel to run. If it does not, `start` fails and prints the last lines of `panel.log`.

`stop` sends SIGTERM only to the pid that `status` verified. It waits up to about 10 seconds for the lock to free. If the server does not stop in that time, `stop` fails and keeps `panel.json`. A `panel.json` whose lock is free is removed, and no signal is sent.

The server binds to `127.0.0.1` only. Its URL is `http://127.0.0.1:<port>/`.

## State files

The state files sit in `<git common dir>/.harness/`. One panel serves every worktree of the repository, and git does not track these files.

| File | Content |
|---|---|
| `panel.json` | The pid, the port, the repo and the version of the running server. It is written after the server listens, and removed on exit. |
| `panel.lock` | The lock of the running server. The server holds it while it runs. |
| `panel.log` | The output of the background server. |

The files have mode `0600`. When the panel creates the folder, the folder has mode `0700`.

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
- `panel.open: host` prints the run URL on stderr when a plan waits for approval: `panel: <url>runs/<id>`. It opens no browser. The harness skill opens that URL in the host preview.

## The host open

With `panel.enabled: true` and `panel.open: host`, the plan approval prints `panel: <url>runs/<id>` on stderr instead of opening the browser. If the panel cannot start, the harness prints `panel: start failed: <err>` and `panel: opening the browser`, then opens the plan in the browser.

`harness plan view --open` stays browser only. It does not use the host open.

## Phone access

The server binds to `127.0.0.1`, so another device cannot reach it. To open the panel on a phone, run a tunnel that you control to the port. The harness does not start a tunnel.
