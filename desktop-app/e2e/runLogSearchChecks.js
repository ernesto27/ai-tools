import assert from "node:assert/strict";
import { readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runLogSearchChecks(browser, fixture, artifacts, { click, openRuns, text }) {
  const runButton = `main button[aria-label="Open run ${fixture.runId}"]`;
  const trigger = '//main//button[@aria-haspopup="dialog" and normalize-space(.)="Search run logs"]';
  const dialog = "main dialog[open]";
  const query = `${dialog} input[aria-label="Search run logs"]`;
  const results = `${dialog} [aria-label="Log search results"]`;
  const noMatch = '//main//dialog//*[@role="status" and text()="No matching log lines."]';
  const truncatedText = "Showing 200 of 250 matches. Refine your search to see the rest.";
  const truncated = `//main//dialog//p[text()="${truncatedText}"]`;
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
  async function checkReviewTarget() {
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
    }, reviewPanel), { focused: true, visible: true, highlighted: true });
  }

  async function openSearch() {
    await click(trigger);
    const input = await browser.$(query);
    await input.waitForDisplayed({ timeout: 15_000 });
    assert.equal(await browser.execute((selector) => document.activeElement.matches(selector), query), true);
    return input;
  }

  async function checkDismissed() {
    await (await browser.$(dialog)).waitForExist({ reverse: true, timeout: 5000 });
    assert.equal(await (await browser.$(trigger)).isFocused(), true);
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
    create("03-securityReview.log", Array.from({ length: 250 }, (_, index) => `Bulk result ${index + 1}`).join("\n"));

    await openRuns();
    await click(runButton);
    await (await browser.$(trigger)).waitForEnabled({ timeout: 15_000 });
    assert.equal(await (await browser.$('main input[aria-label="Search run logs"]')).isExisting(), false);
    let input = await openSearch();
    assert.equal(await (await browser.$(results)).isExisting(), false);

    await input.setValue(" search[42] ");
    await text(`${implementationMatch} span:last-child`, implementationLine);
    await text(`${reviewMatch} span:last-child`, "SEARCH[42] and SEARCH[42]");
    await text(`${summaryMatch} span:last-child`, "Summary includes search[42].");
    assert.deepEqual(await browser.execute((selector) =>
      Array.from(document.querySelectorAll(`${selector} h3`), (node) => node.textContent),
    results), ["Implement", "Code review", "Summary"]);
    assert.equal((await browser.$$(`${results} [data-log-match]`)).length, 3);
    assert.equal(await (await browser.$(truncated)).isExisting(), false);
    await browser.saveScreenshot(join(artifacts, "run-log-search.png"));

    await click(implementationMatch);
    assert.equal(await (await browser.$(dialog)).isExisting(), false);
    await text('main [aria-label="Contents of 01-implementation.log"] [data-log-line="2"] span:last-child',
      implementationLine);
    input = await openSearch();
    assert.equal(await input.getValue(), " search[42] ");

    writeFileSync(join(directory, "summary.log"), "Summary includes search[42].\nA new search[42] line.\n");
    await text(`${results} [data-log-match="summary.log:2"] span:last-child`, "A new search[42] line.");

    await browser.execute((selector) => document.querySelector(selector).focus(), reviewMatch);
    await browser.keys("Enter");
    assert.equal(await (await browser.$(dialog)).isExisting(), false);
    await text(`${reviewPanel} [data-log-line="2"] span:last-child`, "SEARCH[42] and SEARCH[42]");
    await text(`${reviewPanel} [data-log-line="3"] span:last-child`, "Review context below");
    await checkReviewTarget();

    writeFileSync(join(directory, "02-codeReview.log"), review + "\nNew review output\n");
    await text(`${reviewPanel} [data-log-line="94"] span:last-child`, "New review output");
    await checkReviewTarget();

    input = await openSearch();
    await browser.keys(["Control", "a"]);
    await browser.keys("Backspace");
    assert.equal(await (await browser.$(results)).isExisting(), false);

    await input.setValue(" search[42] ");
    await click(summaryMatch);
    assert.equal(await (await browser.$(dialog)).isExisting(), false);
    const summaryPanel = 'main [aria-label="Contents of summary.log"]';
    await text(`${summaryPanel} [data-log-line="1"] span:last-child`, "Summary includes search[42].");
    await click('//main//button[@role="tab" and normalize-space(.)="Walkthrough"]');
    input = await openSearch();
    assert.equal(await input.getValue(), " search[42] ");
    await text(`${summaryMatch} span:last-child`, "Summary includes search[42].");

    await input.setValue("bulk result");
    await text(truncated, truncatedText);
    assert.equal((await browser.$$(`${results} [data-log-match]`)).length, 200);

    await input.setValue("absent query");
    await text(noMatch, "No matching log lines.");
    assert.equal(await (await browser.$(results)).isExisting(), false);
    await input.setValue("   ");
    assert.equal(await (await browser.$(results)).isExisting(), false);
    assert.equal(await (await browser.$(noMatch)).isExisting(), false);
    assert.equal(await (await browser.$(truncated)).isExisting(), false);

    await input.setValue("search[42]");
    await browser.keys("Escape");
    await checkDismissed();
    input = await openSearch();
    assert.equal(await input.getValue(), "search[42]");
    await click('//main//dialog//button[normalize-space(.)="Close"]');
    await checkDismissed();

    await openRuns();
    await click(runButton);
    await (await browser.$(trigger)).waitForEnabled({ timeout: 15_000 });
    const reopenedInput = await openSearch();
    assert.equal(await reopenedInput.getValue(), "");
    await browser.keys("Escape");
    await checkDismissed();
  } finally {
    writeFileSync(implementation, original);
    for (const path of created) unlinkSync(path);
  }
}
