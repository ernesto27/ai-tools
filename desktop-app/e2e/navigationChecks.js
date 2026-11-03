import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { runModelAutocompleteChecks } from "./modelAutocompleteChecks.js";

export async function runNavigationChecks(t, browser, fixture, artifacts) {
  const { repoName, branch, runId, logText } = fixture;
  const navButton = (text) =>
    `//nav[@data-testid="main-nav"]//button[normalize-space(.)="${text}"]`;

  async function visible(selector) {
    const element = await browser.$(selector);
    await element.waitForDisplayed({ timeout: 15_000 });
    return element;
  }

  async function text(selector, expected) {
    let actual;
    try {
      await browser.waitUntil(async () => {
        const element = await browser.$(selector);
        if (!await element.isDisplayed()) {
          actual = undefined;
          return false;
        }

        // WebKit's text endpoint returns empty text for some clipped labels.
        actual = await element.getProperty("innerText");
        return actual === expected;
      }, { timeout: 15_000, timeoutMsg: `Expected visible text: ${expected}` });
    } catch (error) {
      throw new Error(
        `Expected visible text ${JSON.stringify(expected)}; received ${JSON.stringify(actual)}`,
        { cause: error },
      );
    }
  }

  function mainHasText(expected) {
    return text(`//main//*[text()="${expected}"]`, expected);
  }

  async function click(selector) {
    const element = await visible(selector);
    await element.waitForClickable({ timeout: 15_000 });
    await element.click();
  }

  async function openRepo() {
    const selector = 'nav[data-testid="main-nav"] button[aria-haspopup="menu"]';
    await text(`${selector} span[title="${repoName}"]`, repoName);
    if (await (await browser.$(selector)).getAttribute("aria-expanded") !== "true") {
      await click(selector);
    }
  }

  async function openBranch() {
    await openRepo();
    const selector = `nav[data-testid="main-nav"] button[title="${branch}"]`;
    await visible(selector);
    if (await (await browser.$(selector)).getAttribute("aria-expanded") !== "true") {
      await click(selector);
    }
  }

  async function openRuns() {
    await openBranch();
    await click(navButton("Runs"));
    await text("main h1", branch);
    await text("#runs-heading", "1 run");
  }

  async function mode(name, description) {
    await text("main h1", "New run");
    await mainHasText(description);
    const tab = await visible(
      `//main//*[@role="tablist" and @aria-label="Mode"]` +
      `//button[@role="tab" and normalize-space(.)="${name}"]`,
    );
    assert.equal(await tab.getAttribute("aria-selected"), "true");
    await text("#config-preview-h", "software-factory.json preview");
  }

  async function screen(name, check) {
    await t.test(name, { timeout: 60_000 }, async (screenContext) => {
      try {
        await check();
        const toggle = await browser.$('[data-testid="theme-toggle"]');
        const startingTheme = await browser.execute(() => document.documentElement.dataset.theme);
        for (const theme of ["light", "dark"]) {
          if (await browser.execute(() => document.documentElement.dataset.theme) !== theme) {
            await toggle.click();
          }
          await browser.waitUntil(async () =>
            await browser.execute(() => document.documentElement.dataset.theme) === theme,
            { timeout: 5_000 });
          await browser.saveScreenshot(join(artifacts,
            `${name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${theme}.png`));
        }
        if (await browser.execute(() => document.documentElement.dataset.theme) !== startingTheme) {
          await toggle.click();
        }
        const alerts = await browser.$$("main [role=alert]");
        for (const alert of alerts) {
          assert.equal(await alert.isDisplayed(), false,
            `Unexpected screen error: ${await alert.getText()}`);
        }
      } catch (error) {
        const directory = join(artifacts, name.toLowerCase().replace(/[^a-z0-9]+/g, "-"));
        try {
          mkdirSync(directory, { recursive: true });
          const results = await Promise.allSettled([
            browser.saveScreenshot(join(directory, "failure.png")),
            browser.getPageSource().then((source) =>
              writeFileSync(join(directory, "failure.html"), source)),
          ]);
          screenContext.diagnostic(`Screen artifacts: ${directory}`);
          for (const result of results) {
            if (result.status === "rejected") {
              screenContext.diagnostic(`Could not capture diagnostic: ${result.reason}`);
            }
          }
        } catch (diagnosticError) {
          screenContext.diagnostic(`Could not create artifacts: ${diagnosticError}`);
        }
        throw error;
      }
    });
  }

  await screen("Home", async () => {
    await text("main h1", "Select a repository");
    await mainHasText(
      "Choose a repository from the sidebar to view its branches and runs.",
    );
    await text(`nav[data-testid="main-nav"] span[title="${repoName}"]`, repoName);
  });

  await screen("Add repository", async () => {
    await click(navButton("Add repository"));
    await text("main h1", "Add repository");
    await mainHasText(
      "Point Soft Factory at a git repository on this machine.",
    );
  });

  await screen("Config", async () => {
    await openRepo();
    await click(navButton("Config"));
    await text("main h1", "Config");
    await text('//main//h2[normalize-space(.)="Agent defaults"]', "Agent defaults");
    await text('main header [role="status"]', "Saved");
    await runModelAutocompleteChecks(browser, fixture, artifacts);
  });

  await screen("Branch runs", async () => {
    await openRuns();
    await text("main article h3", runId);
    await text("main article dd", "FAILED");
  });

  await screen("New run Run", async () => {
    await openRepo();
    await click(navButton("New run"));
    await mode("Run", "Full workflow: implement, review, classify risk, explain.");
  });

  await screen("New run Continue", async () => {
    await openBranch();
    await click(navButton("Continue"));
    await mode("Continue", "Rerun implementation and later stages on the existing resume.branch worktree.");
  });

  await screen("New run Resume", async () => {
    await openBranch();
    await click(navButton("Resume"));
    await mode("Resume", "Pick up an interrupted run from its log folder.");
    await text("#rs-h", "Failed or interrupted runs");
    await text(`//main//label//span[text()="${runId}"]`, runId);
  });

  await screen("Run detail", async () => {
    await openRuns();
    await click(`main button[aria-label="Open run ${runId}"]`);
    await text("main h1", runId);
    await text(
      `//main//*[@role="region" and @aria-label="Contents of 01-implementation.log"]` +
      `//span[text()="${logText}"]`,
      logText,
    );
    await text('//main//dl/div[dt[normalize-space(.)="Implement"]]/dd', "Failed");
  });
}
