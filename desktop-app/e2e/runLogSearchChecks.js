import assert from "node:assert/strict";
import { readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runLogSearchChecks(browser, fixture, artifacts, { click, openRuns, text }) {
  const runButton = `main button[aria-label="Open run ${fixture.runId}"]`;
  const query = 'main input[aria-label="Search run logs"]';
  const results = 'main [aria-label="Log search results"]';
  const noMatch = '//main//*[@role="status" and text()="No matching log lines."]';
  const implementationMatch = `${results} [data-log-match="01-implementation.log:2"]`;
  const reviewMatch = `${results} [data-log-match="02-codeReview.log:2"]`;
  const summaryMatch = `${results} [data-log-match="summary.log:1"]`;
  const directory = join(fixture.projectLogs, fixture.branch, fixture.runId);
  const implementation = join(directory, "01-implementation.log");
  const original = readFileSync(implementation);
  const created = [];
  const implementationLine = "Search[42] appears in <output>.";

  function create(name, content) {
    const path = join(directory, name);
    writeFileSync(path, content, { flag: "wx" });
    created.push(path);
  }

  const reviewPanel = 'main [aria-label="Contents of 02-codeReview.log"]';
  async function checkReviewTarget(expectFocus) {
    assert.deepEqual(await browser.execute((selector) => {
      const panel = document.querySelector(selector);
      const row = panel.querySelector('[data-log-line="2"]');
      const viewport = panel.getBoundingClientRect();
      const target = row.getBoundingClientRect();
      return {
        focused: document.activeElement === row,
        visible: target.top >= viewport.top && target.bottom <= viewport.bottom,
        highlighted: row.classList.contains("bg-info/10"),
      };
    }, reviewPanel), { focused: expectFocus, visible: true, highlighted: true });
  }

  try {
    writeFileSync(implementation, `${fixture.logText}\r\n${implementationLine}\r\n`);
    const review = [
      "Review context above",
      "SEARCH[42] and SEARCH[42]",
      "Review context below",
      ...Array.from({ length: 90 }, (_, index) => `Review output ${index + 1}`),
    ].join("\n");
    create("02-codeReview.log", review);
    create("summary.log", "Summary includes search[42].\n");
    create("05-reviewChanges.md", "SEARCH[42] in the walkthrough.\n");

    await openRuns();
    await click(runButton);
    const input = await browser.$(query);
    await input.waitForEnabled({ timeout: 15_000 });
    assert.equal(await (await browser.$(results)).isExisting(), false);

    await input.setValue(" search[42] ");
    await text(`${implementationMatch} span:last-child`, implementationLine);
    await text(`${reviewMatch} span:last-child`, "SEARCH[42] and SEARCH[42]");
    await text(`${summaryMatch} span:last-child`, "Summary includes search[42].");
    assert.deepEqual(await browser.execute((selector) =>
      Array.from(document.querySelectorAll(`${selector} h3`), (node) => node.textContent),
    results), ["Implement", "Code review", "Summary"]);
    assert.equal((await browser.$$(`${results} [data-log-match]`)).length, 3);

    await click(implementationMatch);
    await text('main [aria-label="Contents of 01-implementation.log"] [data-log-line="2"] span:last-child',
      implementationLine);
    await browser.execute((selector) => document.querySelector(selector).focus(), reviewMatch);
    await browser.keys("Enter");
    await text(`${reviewPanel} [data-log-line="2"] span:last-child`, "SEARCH[42] and SEARCH[42]");
    await text(`${reviewPanel} [data-log-line="3"] span:last-child`, "Review context below");
    await checkReviewTarget(true);
    await browser.saveScreenshot(join(artifacts, "run-log-search.png"));

    writeFileSync(join(directory, "02-codeReview.log"), review + "\nNew review output\n");
    writeFileSync(join(directory, "summary.log"), "Summary includes search[42].\nA new search[42] line.\n");
    await text(`${results} [data-log-match="summary.log:2"] span:last-child`, "A new search[42] line.");
    await text(`${reviewPanel} [data-log-line="94"] span:last-child`, "New review output");
    await checkReviewTarget(true);

    await input.click();
    await browser.keys(["Control", "a"]);
    await browser.keys("Backspace");
    assert.equal(await (await browser.$(results)).isExisting(), false);
    await checkReviewTarget(false);

    await input.setValue(" search[42] ");
    await click(summaryMatch);
    const summaryPanel = 'main [aria-label="Contents of summary.log"]';
    await text(`${summaryPanel} [data-log-line="1"] span:last-child`, "Summary includes search[42].");
    assert.equal(await input.getValue(), " search[42] ");
    await click('//main//button[@role="tab" and normalize-space(.)="Walkthrough"]');
    assert.equal(await input.getValue(), " search[42] ");
    await text(`${summaryMatch} span:last-child`, "Summary includes search[42].");

    await input.setValue("absent query");
    await text(noMatch, "No matching log lines.");
    assert.equal(await (await browser.$(results)).isExisting(), false);
    await input.setValue("   ");
    assert.equal(await (await browser.$(results)).isExisting(), false);
    assert.equal(await (await browser.$(noMatch)).isExisting(), false);

    await input.setValue("search[42]");
    await openRuns();
    await click(runButton);
    const reopenedInput = await browser.$(query);
    await reopenedInput.waitForEnabled({ timeout: 15_000 });
    assert.equal(await reopenedInput.getValue(), "");
  } finally {
    writeFileSync(implementation, original);
    for (const path of created) unlinkSync(path);
  }
}
