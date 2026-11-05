import assert from "node:assert/strict";
import { readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runPullRequestChecks(browser, fixture, artifacts, { text, openRuns, click }) {
  const directory = join(fixture.projectLogs, fixture.branch, fixture.runId);
  const logFile = join(directory, "01-implementation.log");
  const statusFile = join(directory, "stage_status.txt");
  const originalLog = readFileSync(logFile, "utf8");
  const originalStatus = readFileSync(statusFile, "utf8");
  const selector = 'main header button[data-testid="run-pull-request"]';
  const first = "https://github.com/owner/repo/pull/123";
  const last = "https://github.com/owner/repo/pull/456";
  let revision = 0;

  async function check(content, statuses, expected) {
    const marker = `PR fixture revision ${++revision}`;
    writeFileSync(statusFile, statuses);
    writeFileSync(logFile, `${marker}\n${content}\n`);
    // Wait for this exact log snapshot before asserting absence or a changed URL.
    await text(`//main//*[@role="region"]//span[text()="${marker}"]`, marker);
    await browser.waitUntil(async () => {
      const link = await browser.$(selector);
      if (!expected) return !await link.isExisting();
      return await link.isExisting()
        && await link.isDisplayed()
        && await link.getAttribute("href") === null
        && await link.getProperty("innerText") === expected;
    }, { timeout: 15_000 });
  }

  async function reopen() {
    await openRuns();
    await click(`main button[aria-label="Open run ${fixture.runId}"]`);
    await text("main h1", fixture.runId);
    await text(selector, last);
  }

  const success = "implementation,SUCCEEDED\n";
  const published = `Pull request ready for review: ${first}`;
  try {
    await check("No publication output", success, null);
    for (const status of ["", "implementation,FAILED\n", "implementation,INTERRUPTED\n", "implementation,SKIPPED\n"]) {
      await check(published, status, null);
    }
    for (const prefix of [
      "Pull request ready for review",
      "Existing pull request",
      "Existing pull request (draft)",
      "PR description updated",
      "No net session changes. PR description unchanged",
    ]) {
      await check(`${prefix}: ${first}\r\n`, success, first);
    }
    await check(`Agent mentioned ${last}\n            ${published}`, success, null);
    for (const invalid of [
      "http://github.com/owner/repo/pull/1",
      "https://github.com.evil.invalid/owner/repo/pull/1",
      "https://github.com@evil.invalid/owner/repo/pull/1",
      "https://github.com/owner/repo/pull/0",
      "https://github.com/owner/repo/pull/1?x=1",
      "https://github.com/owner/repo/pull/1#comment",
      "https://github.com/owner/repo/pull/1/files",
      "https://github.com/../repo/pull/1",
      "javascript:alert(1)",
    ]) {
      await check(`Pull request ready for review: ${invalid}`, success, null);
    }
    await check(`${published}\nExisting pull request: invalid`, success, first);
    await check(`${published}\nExisting pull request: ${last}`, success, last);
    await check(published, `${success}implementation,FAILED\n`, null);
    await check(`Existing pull request: ${last}`, `${success}codeReview,RUNNING\n`, last);
    await check(`Existing pull request: ${last}`, `${success}codeReview,FAILED\n`, last);
    await reopen();
    const previousSession = browser.sessionId;
    await browser.reloadSession();
    assert.notEqual(browser.sessionId, previousSession);
    await text("main h1", "Select a repository");
    await reopen();
    await browser.saveScreenshot(join(artifacts, "run-pull-request-reopened.png"));

    const laterFile = join(directory, "02-codeReview.log");
    // Avoid owning or overwriting any existing later-stage log.
    writeFileSync(laterFile, published, { flag: "wx" });
    try {
      // Pin implementation so automatic stage following cannot select code review.
      await click('//main//button[span[normalize-space(.)="Implement"]]');
      await check("Implementation without a published PR", success, null);
    } finally {
      unlinkSync(laterFile);
    }
  } finally {
    writeFileSync(logFile, originalLog);
    writeFileSync(statusFile, originalStatus);
  }
  await text(`//main//*[@role="region"]//span[text()="${fixture.logText}"]`, fixture.logText);
  await browser.waitUntil(async () => !await (await browser.$(selector)).isExisting(), { timeout: 15_000 });
}
