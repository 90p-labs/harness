# Demo stack and browser

## Coherent, repeatable data

Use an isolated local/test environment and the project's seed/reset commands. Give demo records an explicit owner or deterministic namespace. Before each run, reset only that namespace, including records left by interrupted runs. If ownership cannot be proven, use a fresh isolated seed rather than deleting records.

Seed relationships together: database foreign keys, external identifiers and the matching fake-service records must resolve. Add roles only for permissions the walkthrough actually shows. Record non-secret connection facts in a temporary `demo-ready.json`:

```json
{"appUrl":"http://127.0.0.1:3000","cdpUrl":"http://127.0.0.1:9333"}
```

Keep credentials in the project's existing secret mechanism. Keep readiness files, browser profiles and logs out of source control. Confirm stack readiness before driving it. If a later test uses the same ports, stop only demo-owned processes first.

## Dedicated visible Chrome fallback

Use this when a supported visible browser tool cannot meet the request. Start Chrome with a disposable profile and a loopback-only debugging port; never expose an authenticated debugging endpoint to the network. On macOS (choose an unused port):

```sh
open -na "Google Chrome" --args --remote-debugging-address=127.0.0.1 --remote-debugging-port=9333 --user-data-dir=/tmp/watched-demo-profile
```

On Linux, run `google-chrome` with the same flags in a visible desktop session. Verify the endpoint is loopback-bound. Keep this dedicated browser open for handoff and reruns; do not attach a personal profile.

Copy [demo-template.ts](demo-template.ts) into a temporary directory that resolves the target project's `@playwright/test`. Supply a replaceable flow from a small runner, using the project's TypeScript runner:

```ts
import { expect } from "@playwright/test";
import { runDemo } from "./demo-template";

process.exitCode = await runDemo("demo-ready.json", async ({ page, ready, step, caption }) => {
  await step("Open the feature", async () => {
    await page.goto(ready.appUrl);
    const heading = page.getByRole("heading").first();
    await expect(heading).toBeVisible();
    await caption(`Page heading: ${await heading.innerText()}`);
  });
});
```

Replace the flow with the requested feature, authentication and explicit outcome assertions. Pass sensitive values as the third argument (`secrets`) so they are redacted in diagnostics and captions. Label steps with safe, non-secret names. The generic redactor handles URL credentials/query/fragment, bearer tokens and common secret assignments; it cannot recognize arbitrary secret values. Review free-text output before sharing.

The template opens a new demo page and keeps existing pages intact. For `connectOverCDP`, Playwright's `browser.close()` disconnects the CDP transport without terminating Chrome. Do not substitute a launched browser, close the default context, or send CDP `Browser.close`. Additional contexts you explicitly create belong to the demo and need their own cleanup.

The runner returns an exit code after reporting in `finally`; it does not force `process.exit`. Interruptions should use the same cleanup/report path in a long-running wrapper. When finished watching, quit the dedicated Chrome and stop the stack using its documented shutdown command. Remove only the disposable profile after Chrome exits. Do not kill every browser or every process on a port.
