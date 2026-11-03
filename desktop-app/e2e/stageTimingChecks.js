import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runStageTimingChecks(browser, fixture, artifacts, { text, openRuns, click }) {
  const file = join(fixture.projectLogs, fixture.branch, fixture.runId, "01-implementation.log");
  const original = readFileSync(file, "utf8");
  const separator = "=".repeat(50);
  const attempt = (start, finish) => [
    "EXECUTION",
    "Started: 2000-01-01T00:00:00Z",
    separator,
    "STAGE 1: IMPLEMENTATION",
    separator,
    `Started: ${start}`,
    ...(finish ? ["--- RESULT ---", "Status: FAILED", `Finished: ${finish}`] : []),
    "",
  ].join("\n");
  const selector = 'main span[aria-label="Implement duration"]';
  const value = async () => (await browser.$(selector)).getProperty("innerText");
  const seconds = (label) => {
    const match = /^(?:(\d+)h )?(?:(\d+)m )?(\d+)s$/.exec(label);
    assert.ok(match, `Invalid duration: ${label}`);
    return Number(match[1] ?? 0) * 3600 + Number(match[2] ?? 0) * 60 + Number(match[3]);
  };
  const reopen = async () => {
    await openRuns();
    await click(`main button[aria-label="Open run ${fixture.runId}"]`);
    await text("main h1", fixture.runId);
  };
  try {
    const completed = attempt("2026-11-03T12:00:00.000000000Z", "2026-11-03T12:02:08Z");
    writeFileSync(file, completed);
    await text(selector, "2m 08s");
    await reopen();
    await text(selector, "2m 08s");
    await browser.saveScreenshot(join(artifacts, "stage-timing-completed.png"));
    assert.equal(await (await browser.$('main span[aria-label="Code review duration"]')).isExisting(), false);

    const started = Date.now() - 4000;
    writeFileSync(file, completed + attempt(new Date(started).toISOString()));
    await browser.waitUntil(async () => {
      const elapsed = seconds(await value());
      return elapsed >= 4 && elapsed < 30;
    }, { timeout: 15_000 });
    const first = seconds(await value());
    await browser.waitUntil(async () => seconds(await value()) > first, { timeout: 5000 });
    // No desktop session exists for this prepared fixture: the log still drives timing.
    await reopen();
    await browser.waitUntil(async () => {
      const actual = seconds(await value());
      const expected = Math.floor((Date.now() - started) / 1000);
      return Math.abs(actual - expected) <= 2;
    }, { timeout: 15_000 });

    // Relaunch the real app through a fresh driver session, retaining fixture files.
    const previousSession = browser.sessionId;
    await browser.reloadSession();
    assert.notEqual(browser.sessionId, previousSession);
    await text("main h1", "Select a repository");
    await reopen();
    await browser.waitUntil(async () => {
      const actual = seconds(await value());
      return Math.abs(actual - Math.floor((Date.now() - started) / 1000)) <= 2;
    }, { timeout: 15_000 });
    await browser.saveScreenshot(join(artifacts, "stage-timing-reopened.png"));

    writeFileSync(file, completed + attempt(new Date(started).toISOString(), new Date(started + 8000).toISOString()));
    await text(selector, "8s");
    // Observe beyond two clock ticks; a completed attempt must remain fixed.
    const stableUntil = Date.now() + 2200;
    await browser.waitUntil(async () => {
      assert.equal(await value(), "8s");
      return Date.now() >= stableUntil;
    }, { timeout: 5000, interval: 200 });

    const cases = [
      [attempt("2026-11-03T12:00:00Z", "2026-11-03T13:02:03Z"), "1h 02m 03s"],
      [completed.replace(/\n/g, "\r\n"), "2m 08s"],
      [completed + attempt("invalid"), "Duration unavailable"],
      [attempt("2026-11-03T12:00:00Z", "2026-11-03T11:59:59Z"), "Duration unavailable"],
      ["Legacy log without timestamps\n", "Duration unavailable"],
      [attempt(new Date(Date.now() + 3600000).toISOString()), "0s"],
    ];
    for (const [content, expected] of cases) {
      writeFileSync(file, content);
      await text(selector, expected);
    }
  } finally {
    writeFileSync(file, original);
  }
  await text(selector, "Duration unavailable");
}
