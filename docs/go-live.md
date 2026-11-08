# Go-live

The `live` module follows a run from the merged PR to production. This page tells you how to use it.

## Turn it on

Set `run.go_live: true` in `.harness.yaml`. Set the live keys if you need them.

```yaml
run:
  go_live: true
live:
  environment: production
  rollback: helm rollback api
```

- `live.environment` is the GitHub deployment environment. The default is `production`.
- `live.rollback` is the command that the findings show on a regression. The default is empty. The harness never runs it.

## Flow

1. Merge the PR.
2. Run `harness run done --pr URL`. The URL must be a GitHub PR URL. The run goes to `releasing`.
3. `harness live check` reads the merge and the deploy from GitHub. A deploy moves the run to `observing`.
4. In `observing`, the check reads the `## Signals` table of the plan. It asks the `signals.check` plugins about each row.
5. A breach sends the run back to `planning` with the findings. A healthy window closes the run.
6. A failed deploy, or a PR closed with no merge, also sends the run back to `planning`.
7. The last live round blocks the run, not `planning`.

A plugin that fails gives no answer. The check stays `pending`. More than one window after the window end, the run is `blocked` with "signals unavailable". A plan that does not read blocks the run at once, and the reason names the read error.

## The Signals table

The plan has one `## Signals` table. The header is fixed.

```markdown
## Signals

| Criterion | Signal | Source | Query | Breach | Window | Panel |
|---|---|---|---|---|---|---|
| 3 | error rate | prometheus | `api:http_errors:ratio_rate5m` | `> 0.01` | 30m | https://grafana.example/d/api |
```

- `Criterion` is the number of the criterion that the row proves.
- `Source` names the plugin that answers the row.
- `Breach` is `>`, `>=`, `<` or `<=`, then a number.
- `Window` is a Go duration, for example `30m` or `24h`.
- The watch window is the longest window. It starts when the deploy succeeds.
- With no rows, the watch window is zero. The first check in `observing` closes the run.

## The signals.check plugin

The check sends one `signals.check` event for each row. The plugin is an exec plugin. Add it in `.harness.yaml`.

```yaml
plugins:
  - name: prom
    command: [./scripts/prom-check.sh]
    events: [signals.check]
```

The payload is in the envelope field `payload`:

```json
{"signal":"error rate","source":"prometheus","query":"api:http_errors:ratio_rate5m","breach":{"op":">","value":0.01},"window":{"from":"2026-11-08T10:00:00Z","to":"2026-11-08T10:30:00Z"},"sha":"abc1234","environment":"production"}
```

The plugin answers on stdout with one JSON verdict.

- No breach: `{"verdict":"allow"}`.
- A breach: `{"verdict":"deny","reason":"error rate 0.034 > 0.01"}`. Each line of the reason is one breach in the findings.
- An exit code other than 0 is a plugin failure, not a breach.

A plugin must read only the rows of its own `source`. For any other source, answer `allow`.

A source with no plugin gives no breach. The bus returns an allow for it. So a run can close with no check on that row. Add a plugin for every source that you list.

## A pipeline outside GitHub Actions

The harness reads the deploy from the GitHub deployment status. A pipeline outside GitHub Actions must write it. Use the id of the deployment for the merge SHA.

```sh
gh api repos/OWNER/REPO/deployments/ID/statuses -f state=success -f environment=production
```

- `success` is a deploy.
- `failure` and `error` are a failed deploy.
- Other states are pending. The newest status that is not `inactive` wins.

## Run the check

`harness live check` checks each run of the repo in `releasing` or `observing`. It prints one line for each run.

```text
20261108-011805-add-the-login-page observing healthy
```

The line is `<id> <phase> <outcome>`. An error adds `: <error>`. The command exits 1 when a run fails. It exits 0 after a full pass.

Run it from cron, where the SQLite state is:

```cron
*/10 * * * * cd /path/to/repo && harness live check
```

The session plugin `live.session` runs the same check on `session.started`. It adds one context line `live: <id> <phase> <outcome>` for each live run. A check error with no run line adds the line `live: check failed: <error>`. The check stops after 20 seconds. With no live run, the plugin calls no `gh` command.

## The rollback line

On a regression, the last line of the findings is the rollback line.

```text
Rollback: helm rollback api
```

With no `live.rollback`, the line is `Rollback: not configured, set live.rollback`. The maintainer runs the rollback.
