import assert from "node:assert/strict";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { Key } from "webdriverio";

export async function runWorktreeDeletionChecks(browser, fixture, artifacts, command) {
  const registry = join(fixture.environment.XDG_CONFIG_HOME, "agent-sandbox", "worktrees.jsonl");
  const originalRegistry = readFileSync(registry, "utf8");
  const candidates = ["delete-check/árbol", "delete-check/dirty"].map((branch, index) => ({
    repo: fixture.repo,
    branch,
    path: join(dirname(fixture.repo), `deletion-worktree-${index}`),
    base_branch: "main",
    created: "2026-11-20T12:00:00Z",
  }));
  const [clean, dirty] = candidates;
  const row = (branch) => browser.$(`#main-sidebar button[title="${branch}"]`);
  const openDialog = () => browser.$("dialog[open]");
  const menuAction = () => browser.$('//button[@role="menuitem" and normalize-space(.)="Delete worktree"]');
  const button = (label) => browser.$(`//dialog[@open]//button[normalize-space(.)="${label}"]`);
  const cliBranches = () => JSON.parse(command(
    "agent-sandbox", ["worktree-list", "--json", "--repo", fixture.repo],
    { env: fixture.environment },
  )).map((entry) => entry.branch);
  const localRef = (branch) => command("git", ["-C", fixture.repo, "branch", "--list", branch]).trim();

  async function openFor(target) {
    await (await row(target.branch)).click({ button: "right" });
    const action = await menuAction();
    await action.waitForDisplayed({ timeout: 15_000 });
    await action.click();
    const dialog = await openDialog();
    await dialog.waitForDisplayed({ timeout: 15_000 });
    return dialog;
  }

  try {
    for (const target of candidates) {
      command("git", ["-C", fixture.repo, "worktree", "add", "-b", target.branch, target.path]);
    }
    command("git", [
      "-C", clean.path,
      "-c", "user.name=Desktop UI Test",
      "-c", "user.email=desktop-ui-test@example.invalid",
      "-c", "commit.gpgsign=false",
      "-c", "core.hooksPath=/dev/null",
      "commit", "--allow-empty", "-m", "Unmerged worktree deletion fixture",
    ]);
    writeFileSync(join(dirty.path, "unsaved.txt"), "keep these local changes\n");
    writeFileSync(registry, originalRegistry + candidates.map((target) => JSON.stringify(target) + "\n").join(""));
    assert.ok(candidates.every((target) => cliBranches().includes(target.branch)));

    await browser.refresh();
    const search = await browser.$("#sidebar-search");
    await search.waitForDisplayed({ timeout: 15_000 });
    await search.setValue(fixture.repoName);
    await (await row(clean.branch)).waitForDisplayed({ timeout: 15_000 });
    await (await row(clean.branch)).click();
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === clean.branch,
    { timeout: 15_000 });

    await browser.keys([Key.Shift, Key.F10]);
    await (await menuAction()).waitForDisplayed({ timeout: 15_000 });
    await browser.keys(Key.Escape);
    await (await menuAction()).waitForExist({ reverse: true, timeout: 15_000 });
    assert.equal(await browser.execute(() => document.activeElement?.getAttribute("title")), clean.branch);

    const dialog = await openFor(clean);
    const copy = await dialog.getText();
    assert.ok(copy.includes(clean.branch) && copy.includes(fixture.repoName));
    assert.ok(copy.includes("Unmerged commits may become unreachable."));
    assert.equal(await browser.execute(() => document.activeElement?.textContent?.trim()), "Cancel");
    await browser.saveScreenshot(join(artifacts, "delete-worktree-confirmation.png"));
    await (await button("Cancel")).click();
    await dialog.waitForDisplayed({ reverse: true, timeout: 15_000 });
    await browser.waitUntil(async () =>
      await browser.execute(() => document.activeElement?.getAttribute("title")) === clean.branch,
    { timeout: 15_000 });
    assert.ok(existsSync(clean.path) && localRef(clean.branch) && cliBranches().includes(clean.branch));

    await openFor(dirty);
    assert.equal(await (await button("Force delete worktree")).isExisting(), false);
    await (await button("Delete worktree")).click();
    const failure = await browser.$('dialog[open] [role="alert"]');
    await failure.waitForDisplayed({ timeout: 15_000 });
    assert.match(await failure.getText(), /has uncommitted changes/);
    assert.match(await failure.getText(), /Uncommitted changes will be permanently discarded\./);
    await (await button("Force delete worktree")).waitForDisplayed({ timeout: 15_000 });
    assert.equal(await (await button("Delete worktree")).isExisting(), false);
    assert.equal(await browser.execute(() => document.activeElement?.textContent?.trim()), "Cancel");
    assert.ok(await (await row(dirty.branch)).isExisting());
    assert.ok(localRef(dirty.branch) && cliBranches().includes(dirty.branch));
    assert.equal(readFileSync(join(dirty.path, "unsaved.txt"), "utf8"), "keep these local changes\n");
    await (await button("Cancel")).click();
    await (await openDialog()).waitForExist({ reverse: true, timeout: 15_000 });
    assert.ok(localRef(dirty.branch) && cliBranches().includes(dirty.branch));
    assert.equal(readFileSync(join(dirty.path, "unsaved.txt"), "utf8"), "keep these local changes\n");

    await (await row(dirty.branch)).click();
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === dirty.branch,
    { timeout: 15_000 });
    await openFor(dirty);
    assert.equal(await (await button("Force delete worktree")).isExisting(), false);
    await (await button("Delete worktree")).click();
    const forceButton = await button("Force delete worktree");
    await forceButton.waitForDisplayed({ timeout: 15_000 });
    await browser.saveScreenshot(join(artifacts, "force-delete-worktree.png"));
    await forceButton.click();
    await (await openDialog()).waitForExist({ reverse: true, timeout: 15_000 });
    await (await row(dirty.branch)).waitForExist({ reverse: true, timeout: 15_000 });
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === "Config",
    { timeout: 15_000 });
    assert.equal(existsSync(dirty.path), false);
    assert.equal(localRef(dirty.branch), "");
    assert.equal(cliBranches().includes(dirty.branch), false);

    await (await row(clean.branch)).click();
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === clean.branch,
    { timeout: 15_000 });
    command("git", ["-C", fixture.repo, "worktree", "lock", clean.path]);
    try {
      await openFor(clean);
      await (await button("Delete worktree")).click();
      const lockedFailure = await browser.$('dialog[open] [role="alert"]');
      await lockedFailure.waitForDisplayed({ timeout: 15_000 });
      assert.match(await lockedFailure.getText(), /locked/i);
      assert.equal(await (await button("Force delete worktree")).isExisting(), false);
      assert.ok(await (await button("Delete worktree")).isEnabled());
      await (await button("Cancel")).click();
      await (await openDialog()).waitForExist({ reverse: true, timeout: 15_000 });
    } finally {
      command("git", ["-C", fixture.repo, "worktree", "unlock", clean.path]);
    }

    await openFor(clean);
    await (await button("Delete worktree")).click();
    await (await row(clean.branch)).waitForExist({ reverse: true, timeout: 15_000 });
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === "Config",
    { timeout: 15_000 });
    const configRepo = await browser.$('//main//header//h1[normalize-space(.)="Config"]/preceding-sibling::span[1]');
    assert.equal(await configRepo.getProperty("innerText"), `${fixture.repoName} · ${fixture.repo}`);
    assert.equal(existsSync(clean.path), false);
    assert.equal(localRef(clean.branch), "");
    assert.equal(cliBranches().includes(clean.branch), false);
    assert.ok(await (await row(fixture.branch)).isExisting());
    assert.ok(existsSync(fixture.worktree) && localRef(fixture.branch));
  } finally {
    try {
      const worktrees = new Set(command("git", [
        "-C", fixture.repo, "worktree", "list", "--porcelain", "-z",
      ]).split("\0"));

      for (const { branch, path } of candidates) {
        if (worktrees.has(`worktree ${path}`)) {
          command("git", ["-C", fixture.repo, "worktree", "remove", "--force", path]);
        }
        if (localRef(branch)) {
          command("git", ["-C", fixture.repo, "branch", "-D", "--", branch]);
        }
      }
    } finally {
      writeFileSync(registry, originalRegistry);
      await browser.refresh();
    }
  }
}
