import { toggleThemeInSettings } from "./themeChecks.js";
import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { runModelAutocompleteChecks } from "./modelAutocompleteChecks.js";
import { runSkillAutocompleteChecks } from "./skillAutocompleteChecks.js";
import { runStageTimingChecks } from "./stageTimingChecks.js";
import { runPullRequestChecks } from "./pullRequestChecks.js";
import { runWalkthroughChecks } from "./walkthroughChecks.js";

async function checkRunChanges(browser, fixture, artifacts, command, { openRuns, click, text }) {
  const tab = '//main//button[@role="tab" and normalize-space(.)="Changes"]';
  await openRuns();
  await click(`main button[aria-label="Open run ${fixture.runId}"]`);
  await click(tab);
  const load = '//main//button[normalize-space(.)="Load latest commit changes"]';
  const tabs = await browser.execute(() => Array.from(
    document.querySelectorAll('main section[aria-label="Output"] [role="tab"]'),
    (tab) => tab.textContent.trim(),
  ));
  assert.deepEqual(tabs.slice(-2), ["Walkthrough", "Changes"]);

  const git = (...args) => command("git", ["-C", fixture.worktree, ...args]);
  const commit = (message, ...extra) => git(
    "-c", "user.name=Desktop UI Test",
    "-c", "user.email=desktop-ui-test@example.invalid",
    "-c", "commit.gpgsign=false",
    "-c", "core.hooksPath=/dev/null",
    "commit", ...extra, "-m", message,
  );
  const paths = ["changes/a.txt", "changes/nested/b.txt", "changes/oversized.txt"];
  mkdirSync(dirname(join(fixture.worktree, paths[1])), { recursive: true });
  const original = Array.from({ length: 16 }, (_, index) => "Original line " + (index + 1));
  writeFileSync(join(fixture.worktree, paths[0]), original.join("\n") + "\n");
  git("add", "--", paths[0]);
  commit("Baseline for numbered diff");
  writeFileSync(join(fixture.worktree, paths[0]), [
    original[0], "First selected file", "Extra line", ...original.slice(2, -1), "Last selected line",
  ].join("\n") + "\n");
  writeFileSync(join(fixture.worktree, paths[1]), "Second selected file\n");
  writeFileSync(join(fixture.worktree, paths[2]), "generated\n".repeat(5_001));
  git("add", "--", ...paths);
  commit("Changes fixture");
  await click(load);
  const item = (path) => `main nav[aria-label="Changed files"] button[title="${path}"]`;
  const patch = (path) => `main section[aria-label="Changes in ${path}"] pre code`;
  await text(`${item(paths[0])} span`, "a.txt");
  await text(`${item(paths[1])} span`, "b.txt");
  assert.equal(await (await browser.$(item(paths[0]))).getAttribute("aria-current"), "true");
  const addedLine = (path) => patch(path) + ' :nth-child(1 of [data-diff-line="addition"]) > span:last-child';
  await text(addedLine(paths[0]), "First selected file");
  const rows = await browser.execute((selector) => Array.from(
    document.querySelectorAll(selector + ' [data-diff-line]'),
    (row) => ({
      kind: row.dataset.diffLine,
      cells: Array.from(row.children, (cell) => cell.textContent),
    }),
  ), patch(paths[0]));
  assert.equal(rows.filter((row) => row.kind === "hunk").length, 2);
  assert.deepEqual(rows.filter((row) => row.kind === "addition" || row.kind === "deletion").map((row) => row.cells), [
    ["2", "", "−", "Original line 2"],
    ["", "2", "+", "First selected file"],
    ["", "3", "+", "Extra line"],
    ["16", "", "−", "Original line 16"],
    ["", "17", "+", "Last selected line"],
  ]);
  assert.ok(rows.some((row) => row.kind === "context" && row.cells.join("|") === "3|4||Original line 3"));
  const totals = 'main section[aria-label="Changes in changes/a.txt"] [aria-label="Added lines: 3. Removed lines: 2."]';
  assert.equal(await (await browser.$(totals)).isDisplayed(), true);
  await browser.saveScreenshot(join(artifacts, "run-changes.png"));
  const folder = 'main nav[aria-label="Changed files"] button[title="nested"]';
  await click(folder);
  assert.equal(await (await browser.$(folder)).getAttribute("aria-expanded"), "false");
  assert.equal(await (await browser.$(item(paths[1]))).isDisplayed(), false);
  await click(folder);
  await text(`${item(paths[1])} span`, "b.txt");
  await click(item(paths[1]));
  assert.equal(await (await browser.$(item(paths[1]))).getAttribute("aria-current"), "true");
  await text(addedLine(paths[1]), "Second selected file");
  assert.equal(await browser.$(patch(paths[0])).isExisting(), false);
  await browser.execute((selector) => document.querySelector(selector).focus(), item(paths[0]));
  await browser.keys("Enter");
  assert.equal(await (await browser.$(item(paths[0]))).getAttribute("aria-current"), "true");
  await click(item(paths[2]));
  await text(`main section[aria-label="Changes in ${paths[2]}"] [role="status"]`, "Diff too large to display.");
  assert.equal(await browser.$(patch(paths[2])).isExisting(), false);
  await click(item(paths[0]));
  await text(addedLine(paths[0]), "First selected file");

  commit("Empty changes fixture", "--allow-empty");
  await click(load);
  await text('main section[aria-label="Output"] [role="status"]', "No changes found.");
}

export async function runNavigationChecks(t, browser, fixture, artifacts, command) {
  const { repoName, branch, runId, logText } = fixture;
  const navButton = (text) =>
    `//nav[@data-testid="main-nav"]//button[normalize-space(.)="${text}"]`;

  const taskTab = (name) =>
    `//main//*[@role="tablist" and @aria-label="Task source"]//button[normalize-space(.)="${name}"]`;
  const imagesPanel = 'main [role="region"][aria-label="Images"]';
  const imageRows = () => browser.$$(`${imagesPanel} input[placeholder="Path to your image"]`);
  const imageValues = async () => Promise.all([...(await imageRows())].map((row) => row.getValue()));

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
    const selector = `nav[data-testid="main-nav"] [data-sidebar-repo="${fixture.repo}"] > button`;
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

  async function screen(name, check, timeout = 60_000) {
    await t.test(name, { timeout }, async (screenContext) => {
      try {
        await check();
        const startingTheme = await browser.execute(() => document.documentElement.dataset.theme);
        for (const theme of ["light", "dark"]) {
          if (await browser.execute(() => document.documentElement.dataset.theme) !== theme) {
            await toggleThemeInSettings(browser);
          }
          await browser.waitUntil(async () =>
            await browser.execute(() => document.documentElement.dataset.theme) === theme,
            { timeout: 5_000 });
          await browser.saveScreenshot(join(artifacts,
            `${name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${theme}.png`));
        }
        if (await browser.execute(() => document.documentElement.dataset.theme) !== startingTheme) {
          await toggleThemeInSettings(browser);
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

  await t.test("Sidebar visibility", async () => {
    const sidebar = await visible("#main-sidebar");
    const toggleSelector = 'button[aria-controls="main-sidebar"]';
    const toggle = await visible(toggleSelector);
    assert.equal(await toggle.getAttribute("aria-expanded"), "true");
    await click(toggleSelector);
    await sidebar.waitForDisplayed({ reverse: true, timeout: 5_000 });
    assert.equal(await toggle.getAttribute("aria-expanded"), "false");
    await click(toggleSelector);
    await sidebar.waitForDisplayed({ timeout: 5_000 });
    assert.equal(await toggle.getAttribute("aria-expanded"), "true");
  });

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

  await screen("Base image autocomplete", async () => {
    await openRepo();
    await click(navButton("Config"));
    await text('main header [role="status"]', "Saved");
    const configPath = join(fixture.repo, "software-factory.json");
    const original = readFileSync(configPath, "utf8");
    const input = await visible("#base-image");
    const popupStatus = "#base-image-suggestions + [role=status]";
    let images;
    try {
      const output = command("docker", ["images", "--format", "{{.Repository}}:{{.Tag}}"], { env: fixture.environment });
      images = [...new Set(output.split("\n").map((line) => line.trim())
        .filter((line) => line && !line.includes("<none>")))].sort();
    } catch {
      images = null;
    }

    async function saveBaseImage(expected) {
      await click('//main//header//button[normalize-space(.)="Save"]');
      await text('main header [role="status"]', "Saved");
      assert.equal(JSON.parse(readFileSync(configPath, "utf8")).agentSandbox.baseImage, expected);
    }

    try {
      await input.clearValue();
      await input.click();
      if (images?.length) {
        const expected = images[0];
        await input.setValue(expected.toUpperCase());
        await text("#base-image-suggestions-0", expected);
        await browser.keys(["ArrowDown", "Enter"]);
        assert.equal(await input.getValue(), expected);
        assert.equal(await input.getAttribute("aria-expanded"), "false");
        await saveBaseImage(expected);
      } else {
        await text(popupStatus, images
          ? "No matching local Docker images"
          : "Could not list local Docker images. Enter an image reference manually.");
      }

      const custom = "example.invalid/desktop-e2e/base-image:custom";
      await input.setValue(custom);
      await browser.keys("Tab");
      await saveBaseImage(custom);
    } finally {
      writeFileSync(configPath, original);
    }
    await click(navButton("New run"));
    await click(navButton("Config"));
    await text('main header [role="status"]', "Saved");
  });

  await screen("Clear autocomplete selections", async () => {
    const configPath = join(fixture.repo, "software-factory.json");
    const original = readFileSync(configPath, "utf8");
    const config = JSON.parse(original);
    config.agentSandbox.model = "saved-agent-model";
    config.agentSandbox.baseImage = "example.invalid/saved-image:latest";
    config.stages = { implementation: { model: "saved-stage-model" }, codeReview: { customSkill: "saved-skill" } };

    async function reloadConfig() {
      await click(navButton("New run"));
      await click(navButton("Config"));
      await text('main header [role="status"]', "Saved");
    }

    try {
      writeFileSync(configPath, JSON.stringify(config));
      await openRepo();
      await click(navButton("Config"));
      await reloadConfig();
      for (const id of ["default-model", "base-image", "implementation-model", "codeReview-skill"]) {
        const input = await visible(`#${id}`);
        const clear = await visible(`#${id}-clear`);
        await clear.waitForClickable({ timeout: 15_000 });
        await click(`label[for="${id}"]`);
        assert.equal(await browser.execute(() => document.activeElement.id), id);
        if (id === "default-model") {
          await browser.keys("Tab");
          assert.equal(await browser.execute(() => document.activeElement.id), `${id}-clear`);
          await browser.keys("Enter");
        } else {
          await clear.click();
        }
        assert.equal(await input.getValue(), "");
        assert.equal(await input.getAttribute("aria-expanded"), "false");
        assert.equal(await browser.execute(() => document.activeElement.id), id);
        assert.equal(await (await browser.$(`#${id}-clear`)).isExisting(), false);
      }
      await click('//main//header//button[normalize-space(.)="Save"]');
      await text('main header [role="status"]', "Saved");
      const saved = JSON.parse(readFileSync(configPath, "utf8"));
      assert.equal(saved.agentSandbox.model, undefined);
      assert.equal(saved.agentSandbox.baseImage, undefined);
      assert.equal(saved.stages.implementation.model, undefined);
      assert.equal(saved.stages.codeReview.customSkill, undefined);
    } finally {
      writeFileSync(configPath, original);
    }
    await reloadConfig();
  });

  await screen("Config", async () => {
    await openRepo();
    await click(navButton("Config"));
    await text("main h1", "Config");
    await text('//main//h2[normalize-space(.)="Agent defaults"]', "Agent defaults");
    await text('main header [role="status"]', "Saved");
    await runModelAutocompleteChecks(browser, fixture, artifacts);
    await runSkillAutocompleteChecks(browser, fixture, artifacts);
  }, 180_000);

  await screen("Branch runs", async () => {
    await openRuns();
    await text("main article h3", runId);
    await text("main article dd", "FAILED");
  });

  await screen("New run Run", async () => {
    await openRepo();
    await click(navButton("New run"));
    await mode("Run", "Full workflow: implement, review, classify risk, explain.");
    const branchInput = await visible('main input[placeholder="feature/my-task"]');
    const startButton = await visible('//main//button[normalize-space(.)="Start run"]');
    assert.equal(await startButton.isEnabled(), true);
    await branchInput.click();
    await browser.keys(["Control", "a"]);
    await browser.keys("Backspace");
    assert.equal(await branchInput.getValue(), "");
    assert.equal(await startButton.isEnabled(), false);
    await branchInput.setValue("   ");
    assert.equal(await branchInput.getValue(), "   ");
    assert.equal(await startButton.isEnabled(), false);
    await branchInput.setValue(branch);
    assert.equal(await startButton.isEnabled(), true);

    await checkTaskFiles();

    const configPath = join(fixture.repo, "software-factory.json");
    const configBefore = readFileSync(configPath, "utf8");
    const previewBefore = await (await visible("main pre code")).getText();
    const taskTabs = await browser.$$('main [role="tablist"][aria-label="Task source"] [role="tab"]');
    assert.deepEqual(await Promise.all([...taskTabs].map((tab) => tab.getText())),
      ["Task", "File", "Folder", "Images", "Jira URL"]);

    const png = Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGPQCm8CAAGyAQRUxrVQAAAAAElFTkSuQmCC",
      "base64",
    );
    const imagePaths = ["design review.png", "second,reference.png"]
      .map((name) => join(fixture.repo, "..", name));
    for (const path of imagePaths) writeFileSync(path, png);

    await click(taskTab("Images"));
    await visible(`${imagesPanel} input[placeholder="Path to your image"]`);
    assert.deepEqual(await imageValues(), [""]);
    for (const [index, path] of imagePaths.entries()) {
      if (index > 0) {
        await click('//main//*[@role="region" and @aria-label="Images"]//button[normalize-space(.)="Add image"]');
        assert.equal(await browser.execute((input) => document.activeElement === input, (await imageRows())[index]), true);
      }
      await click(`${imagesPanel} button[aria-label="Browse for Image ${index + 1}"]`);
      const chooser = command("xdotool", ["search", "--sync", "--onlyvisible", "--name", "^Select images$"],
        { timeout: 10_000 }).trim();
      assert.match(chooser, /^\d+$/);
      command("xdotool", ["windowfocus", "--sync", chooser, "key", "--clearmodifiers", "ctrl+l", "ctrl+a"]);
      command("xdotool", ["type", "--clearmodifiers", path + " "]);
      // Allow GTK to resolve the typed path before activating Open.
      command("xdotool", ["sleep", "0.5", "key", "--clearmodifiers", "Return"]);
      await browser.waitUntil(async () => (await imageValues())[index] === path,
        { timeout: 15_000, timeoutMsg: "Expected the chosen path in the image row" });
    }

    await click(taskTab("File"));
    await visible('main button[aria-label="Browse for Task file 1"]');
    await click(taskTab("Images"));
    assert.deepEqual(await imageValues(), imagePaths);
    await click(`${imagesPanel} button[aria-label="Remove image 1"]`);
    assert.deepEqual(await imageValues(), [imagePaths[1]]);
    assert.equal(await (await visible("main pre code")).getText(), previewBefore);
    assert.equal(readFileSync(configPath, "utf8"), configBefore);

    await click('//main//*[@role="tablist" and @aria-label="Mode"]//button[normalize-space(.)="Continue"]');
    await visible(`${imagesPanel} input[placeholder="Path to your image"]`);
    assert.deepEqual(await imageValues(), [imagePaths[1]]);
    await click('//main//*[@role="tablist" and @aria-label="Mode"]//button[normalize-space(.)="Run"]');
    await click(taskTab("File"));

    const reviewersInput = await visible('main input[placeholder="alice, bob"]');
    const preview = await visible('main pre code');
    const previewReviewers = async () => JSON.parse(await preview.getText()).agentSandbox.run?.reviewers;
    assert.equal(await reviewersInput.getValue(), "");
    assert.equal(await previewReviewers(), undefined);
    await mainHasText("Pull request creation is off in this run’s configuration.");
    await reviewersInput.setValue(" alice, , bob-2, ");
    assert.deepEqual(await previewReviewers(), ["alice", "bob-2"]);
    assert.equal(await startButton.isEnabled(), true);
    await reviewersInput.setValue("@alice, alice bob");
    await text("#pr-reviewers-error",
      "Use plain GitHub usernames for @alice, alice bob. Start each name with a letter or digit and use only letters, digits, or hyphens.");
    assert.equal(await startButton.isEnabled(), false);
    await browser.keys(["Control", "a"]);
    await browser.keys("Backspace");
    assert.equal(await (await browser.$("#pr-reviewers-error")).isExisting(), false);
    assert.equal(await previewReviewers(), undefined);
    assert.equal(await startButton.isEnabled(), true);
  });

  async function checkTaskFiles() {
    const sourceTab = (name) => `//main//*[@role="tablist" and @aria-label="Task source"]//button[normalize-space(.)="${name}"]`;
    const rows = () => browser.$$('main input[placeholder="Path to your task file"]');
    const files = async () => JSON.parse(await (await visible("main pre code")).getText()).agentSandbox.run?.filePrompt;
    const remove = (number) => click(`main button[aria-label="Remove task file ${number}"]`);
    const add = () => click('//main//button[normalize-space(.)="Add task file"]');
    const expectFocus = async (index) => {
      assert.equal(await browser.execute((input) => document.activeElement === input, (await rows())[index]), true);
    };

    await click(sourceTab("File"));
    assert.deepEqual(await Promise.all([...(await rows())].map((row) => row.getValue())), ["task.txt", "context notes.md"]);
    await add();
    assert.equal((await rows()).length, 3);
    await expectFocus(2);
    await (await rows())[2].setValue("task.txt");
    assert.deepEqual(await files(), ["task.txt", "context notes.md", "task.txt"]);
    await remove(2);
    await expectFocus(1);
    assert.deepEqual(await files(), ["task.txt", "task.txt"]);
    await (await rows())[1].setValue("  ");
    assert.deepEqual(await files(), ["task.txt"]);
    await click(sourceTab("Task"));
    await click(sourceTab("File"));
    assert.deepEqual(await Promise.all([...(await rows())].map((row) => row.getValue())), ["task.txt", "  "]);
    await remove(1);
    await remove(1);
    assert.equal((await rows()).length, 1);
    assert.equal(await (await rows())[0].getValue(), "");
    await expectFocus(0);
    assert.equal(await files(), undefined);
    await (await rows())[0].setValue("task.txt");
    await add();
    await (await rows())[1].setValue("context notes.md");
    assert.deepEqual(await files(), ["task.txt", "context notes.md"]);
    // Switching modes within this view must retain unsaved file values.
    await (await rows())[1].setValue("unsaved context.md");
    await click('//main//*[@role="tablist" and @aria-label="Mode"]//button[normalize-space(.)="Continue"]');
    assert.deepEqual(await files(), ["task.txt", "unsaved context.md"]);
    await click('//main//*[@role="tablist" and @aria-label="Mode"]//button[normalize-space(.)="Run"]');
    assert.deepEqual(await files(), ["task.txt", "unsaved context.md"]);
    await (await rows())[1].setValue("context notes.md");
  }

  await screen("New run Continue", async () => {
    await openRuns();
    await click(navButton("Continue"));
    await mode("Continue", "Rerun implementation and later stages on the existing resume.branch worktree.");
    await click(taskTab("Images"));
    await visible(imagesPanel);
    assert.deepEqual(await imageValues(), [""]);
    await click(taskTab("Task"));
    await checkTaskFiles();
    await click('//main//*[@role="tablist" and @aria-label="Mode"]//button[normalize-space(.)="Continue"]');
  });

  await t.test("Task file configuration schema", async () => {
    const configPath = join(fixture.repo, "software-factory.json");
    const original = readFileSync(configPath, "utf8");
    try {
      for (const filePrompt of [undefined, [], ["one.md"], ["one.md", "two.md"], "legacy.md", null, ["one.md", 42]]) {
        const config = JSON.parse(original);
        if (filePrompt === undefined) delete config.agentSandbox.run.filePrompt;
        else config.agentSandbox.run.filePrompt = filePrompt;
        writeFileSync(configPath, JSON.stringify(config));
        await openRuns();
        await click(navButton("Continue"));
        const valid = filePrompt === undefined || (Array.isArray(filePrompt) && filePrompt.every((path) => typeof path === "string"));
        if (valid) {
          const preview = await visible("main pre code");
          assert.deepEqual(JSON.parse(await preview.getText()).agentSandbox.run?.filePrompt,
            filePrompt?.length ? filePrompt : undefined);
          await click('//main//*[@role="tablist" and @aria-label="Task source"]//button[normalize-space(.)="File"]');
          assert.equal((await browser.$$('main input[placeholder="Path to your task file"]')).length,
            filePrompt?.length || 1);
        } else {
          const alert = await visible('main [role="alert"]');
          assert.equal(await alert.getText(), "agentSandbox.run.filePrompt must be a list of strings.");
          assert.equal(await (await visible('//main//button[not(@role) and normalize-space(.)="Continue"]')).isEnabled(), false);
        }
      }
    } finally {
      writeFileSync(configPath, original);
      await openRuns();
    }
  });

  await screen("New run Resume", async () => {
    await openBranch();
    await click(navButton("Resume"));
    await mode("Resume", "Pick up an interrupted run from its log folder.");
    assert.equal(await (await browser.$(taskTab("Images"))).isExisting(), false);
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
    await text('//main//button[span[normalize-space(.)="Implement"]]/span[2]', "Failed");
    assert.equal((await browser.$$('main [data-testid^="audit-"] button')).length, 0);
    const statusFile = join(fixture.projectLogs, branch, runId, "stage_status.txt");
    const originalStatuses = readFileSync(statusFile, "utf8");
    try {
      writeFileSync(statusFile, [
        "implementation,SUCCEEDED", "codeReview,SUCCEEDED", "securityReview,SUCCEEDED",
        "riskClassification,SUCCEEDED", "reviewChanges,SUCCEEDED", "",
      ].join("\n"));
      await browser.waitUntil(async () =>
        (await browser.$$('main [data-testid^="audit-"] button')).length === 3,
        { timeout: 15_000 });
      assert.equal(await (await browser.$('main button[aria-label="Audit Implement"]')).isEnabled(), true);
      assert.equal(await (await browser.$('main button[aria-label="Audit Code review"]')).isEnabled(), false);
      assert.equal(await (await browser.$('main button[aria-label="Audit Security review"]')).isEnabled(), false);
      assert.equal(await browser.$('main button button').isExisting(), false);
    } finally {
      writeFileSync(statusFile, originalStatuses);
    }
    await browser.waitUntil(async () =>
      (await browser.$$('main [data-testid^="audit-"] button')).length === 0,
      { timeout: 15_000 });
    await runStageTimingChecks(browser, fixture, artifacts, { text, openRuns, click });
    await runPullRequestChecks(browser, fixture, artifacts, { text, openRuns, click });
    await runWalkthroughChecks(browser, fixture, artifacts, { text, click });
  }, 120_000);
  await screen("Run changes", async () => {
    await checkRunChanges(browser, fixture, artifacts, command, { openRuns, click, text });
  });
}
