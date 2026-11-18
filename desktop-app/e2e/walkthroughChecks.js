import assert from "node:assert/strict";
import { unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runWalkthroughChecks(browser, fixture, artifacts, { click, text }) {
  const file = join(fixture.projectLogs, fixture.branch, fixture.runId, "05-reviewChanges.md");
  const tab = '//main//button[@role="tab" and normalize-space(.)="Walkthrough"]';
  const panel = 'main [aria-label="Contents of 05-reviewChanges.md"]';
  const expand = 'main button[aria-haspopup="dialog"]';
  const dialog = "main dialog[open]";
  const title = "Branch walkthrough fixture";
  const content = [
    `# ${title}`,
    "This document describes the branch changes and how to verify them. **Read each section in order.**",
    "```text\nExample output\n```",
    ...Array.from({ length: 24 }, (_, index) =>
      `## Change ${index + 1}\n\nRead the implementation notes and confirm the expected behavior.`),
  ].join("\n\n");

  await click(tab);
  await text(`${panel} [role="status"]`, "No walkthrough recorded for this run yet.");
  assert.equal(await (await browser.$(expand)).isExisting(), false);

  writeFileSync(file, "", { flag: "wx" });
  try {
    await text(`${panel} [role="status"]`, "This file is empty.");
    assert.equal(await (await browser.$(expand)).isExisting(), false);

    writeFileSync(file, content);
    await text(`${panel} article h1`, title);
    const inlineScrollTop = await browser.execute((selector) => {
      const element = document.querySelector(selector);
      element.scrollTop = 100;
      return element.scrollTop;
    }, panel);
    assert.ok(inlineScrollTop > 0);

    await click(expand);
    await text(`${dialog} article h1`, title);
    await text(`${dialog} article pre code`, "Example output\n");
    const layout = await browser.execute((dialogSelector, panelSelector) => {
      const modal = document.querySelector(dialogSelector);
      const area = modal.querySelector('[role="region"]');
      const modalRect = modal.getBoundingClientRect();
      const areaRect = area.getBoundingClientRect();
      const articleRect = area.querySelector("article").getBoundingClientRect();
      return {
        fillsWindow: modalRect.height > innerHeight * 0.85 && modalRect.width > innerWidth * 0.85,
        centered: Math.abs(articleRect.left - areaRect.left - (area.clientWidth - articleRect.width) / 2) < 2,
        biggerThanInline: area.clientHeight > document.querySelector(panelSelector).clientHeight,
        startsAtTop: area.scrollTop === 0,
        focusedAndScrollable: document.activeElement === area && area.scrollHeight > area.clientHeight,
      };
    }, dialog, panel);
    assert.deepEqual(layout, {
      fillsWindow: true, centered: true, biggerThanInline: true, startsAtTop: true, focusedAndScrollable: true,
    });
    await browser.saveScreenshot(join(artifacts, "walkthrough-expanded.png"));
    await browser.keys("PageDown");
    await browser.waitUntil(async () =>
      (await (await browser.$(`${dialog} [role="region"]`)).getProperty("scrollTop")) > 0,
      { timeout: 5000 });

    async function verifyClosed() {
      await browser.waitUntil(async () => !await (await browser.$(dialog)).isExisting(), { timeout: 5000 });
      assert.equal(await (await browser.$(tab)).getAttribute("aria-selected"), "true");
      assert.equal(await (await browser.$(panel)).getProperty("scrollTop"), inlineScrollTop);
      assert.equal(await browser.execute((selector) => document.activeElement.matches(selector), expand), true);
    }

    await browser.keys("Escape");
    await verifyClosed();
    await click(expand);
    await click(`${dialog} button`);
    await verifyClosed();
  } finally {
    unlinkSync(file);
  }
  await click('//main//button[@role="tab" and normalize-space(.)="Log"]');
  assert.equal(await (await browser.$(expand)).isExisting(), false);
}
