import { readFile } from "node:fs/promises";
import { chromium, type Browser, type Page } from "@playwright/test";

const PAUSE_MS = 1800; // Readability only; use assertions for readiness.
const TIMEOUT_MS = 15_000;
type Ready = { appUrl: string; cdpUrl: string };
type Flow = (demo: {
  page: Page;
  ready: Ready;
  step: (label: string, action: () => Promise<void>) => Promise<void>;
  caption: (observed: string) => Promise<void>;
}) => Promise<void>;

function safeURL(value: string): string {
  try {
    const url = new URL(value);
    return `${url.protocol}//${url.host}${url.pathname}`;
  } catch {
    return "[invalid URL]";
  }
}

// Register app-specific secrets too; arbitrary free text cannot be fully classified.
function redact(value: string, secrets: string[]): string {
  let result = value.replace(/https?:\/\/[^\s<>"']+/gi, safeURL);
  result = result.replace(/\bBearer\s+[^\s,;]+/gi, "Bearer [redacted]")
    .replace(/\b(password|token|secret|api[-_]?key)\s*[:=]\s*[^\s,;]+/gi, "$1=[redacted]");
  for (const secret of secrets.filter(Boolean).sort((a, b) => b.length - a.length)) {
    result = result.split(secret).join("[redacted]");
  }
  return result;
}

export async function runDemo(readyFile: string, flow: Flow, secrets: string[] = []): Promise<0 | 1> {
  const steps: string[] = [];
  const events: string[] = [];
  const clean = (value: string) => redact(value, secrets);
  let browser: Browser | undefined;
  let page: Page | undefined;
  let stage = "Read readiness file";
  let monitoring = false;
  let finished = false;
  let failed = false;
  const removers: (() => void)[] = [];
  try {
    let ready: Ready;
    try {
      ready = JSON.parse(await readFile(readyFile, "utf8"));
      const app = new URL(ready.appUrl);
      const cdp = new URL(ready.cdpUrl);
      if (!["http:", "https:"].includes(app.protocol) ||
          !["http:", "https:"].includes(cdp.protocol) ||
          !["127.0.0.1", "localhost", "[::1]"].includes(cdp.hostname) ||
          app.username || app.password || cdp.username || cdp.password || cdp.search || cdp.hash) {
        throw new Error();
      }
    } catch {
      throw new Error("Readiness file must contain a valid HTTP appUrl and loopback cdpUrl without credentials");
    }
    stage = "Attach visible browser";
    browser = await chromium.connectOverCDP(ready.cdpUrl, { timeout: TIMEOUT_MS });
    const context = browser.contexts()[0];
    if (!context) throw new Error("Attached browser has no default context");
    page = await context.newPage(); // Never close or repurpose existing tabs.
    page.setDefaultTimeout(TIMEOUT_MS);
    page.setDefaultNavigationTimeout(TIMEOUT_MS);
    const watchedPage = page;
    const onConsole = (message: import("@playwright/test").ConsoleMessage) => {
      if (["warning", "error"].includes(message.type())) {
        events.push(clean(`console.${message.type()}: ${message.text()} (${message.location().url})`));
      }
    };
    const onError = (error: import("@playwright/test").WebError) => {
      if (error.page() === watchedPage) events.push(clean(`pageerror: ${error.error().message}`));
    };
    const onFailed = (request: import("@playwright/test").Request) =>
      events.push(clean(`requestfailed ${request.method()} ${safeURL(request.url())}: ${request.failure()?.errorText}`));
    const onResponse = (response: import("@playwright/test").Response) => {
      if (response.status() >= 400) events.push(clean(`HTTP ${response.status()} ${response.request().method()} ${safeURL(response.url())}`));
    };
    page.on("console", onConsole);
    context.on("weberror", onError);
    page.on("requestfailed", onFailed);
    page.on("response", onResponse);
    removers.push(() => {
      watchedPage.off("console", onConsole);
      context.off("weberror", onError);
      watchedPage.off("requestfailed", onFailed);
      watchedPage.off("response", onResponse);
    });
    monitoring = true;
    stage = "Bring demo page forward";
    await page.bringToFront();
    await flow({
      page, ready,
      step: async (label, action) => {
        stage = clean(label);
        await action();
        steps.push(stage);
      },
      caption: async (observed) => {
        const label = clean(observed);
        await watchedPage.evaluate((text) => {
          let box = document.getElementById("watched-demo-caption");
          if (!box) {
            box = document.createElement("div");
            box.id = "watched-demo-caption";
            box.setAttribute("aria-hidden", "true");
            box.style.cssText = "position:fixed;bottom:24px;left:50%;transform:translateX(-50%);z-index:2147483647;background:#111;color:white;padding:12px 18px;border-radius:8px;font:16px system-ui;max-width:90vw;pointer-events:none";
            document.body.appendChild(box);
          }
          box.textContent = text;
        }, label);
        steps.push(`Observed: ${label}`);
        await watchedPage.waitForTimeout(PAUSE_MS);
      },
    });
    finished = true;
  } catch (error) {
    failed = true;
    events.push(clean(`FAILED at ${stage}: ${error instanceof Error ? error.message : String(error)}`));
  } finally {
    // For connectOverCDP this disconnects; Chrome and its default-context tabs survive.
    try { await browser?.close(); } catch (error) {
      failed = true;
      events.push(clean(`Disconnect failed: ${error instanceof Error ? error.message : String(error)}`));
    }
    for (const remove of removers) remove();
    console.log(steps.map((step, i) => `${i + 1}. ${step}`).join("\n") || "No steps completed");
    console.log(events.join("\n") || (monitoring && finished && !failed
      ? "No warnings/errors or failed/HTTP-error requests observed during this run"
      : "Diagnostics incomplete; no clean result established"));
    console.log(`Monitoring: ${monitoring ? "demo page only, until disconnect" : "not started"}`);
    if (page) console.log(`Left open: ${clean(safeURL(page.url()))}`);
  }
  return failed ? 1 : 0;
}
