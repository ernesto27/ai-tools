import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runRecoveryChecks(t, browser, fixture, artifacts, command) {
  const key = "soft-factory.desktop-runs";
  const original = await browser.execute((name) => localStorage.getItem(name), key);
  const preferences = await browser.execute(() => ({
    theme: localStorage.getItem("soft-factory.theme"),
    language: localStorage.getItem("soft-factory.language"),
  }));
  const branchDirectory = join(fixture.projectLogs, fixture.branch);
  const originalRuns = readdirSync(branchDirectory).sort();
  const ownedDirectories = [];
  const saved = {
    id: `desktop:${randomUUID()}`,
    repo: fixture.repoName,
    repoPath: fixture.repo,
    branch: fixture.branch,
    runId: fixture.runId,
    directory: join(branchDirectory, fixture.runId),
    state: "running",
    mode: "run",
    previousDirectories: [],
  };
  const stop = '//main//button[normalize-space(.)="Stop"]';
  const continueButton = '//main//button[normalize-space(.)="Continue"]';

  async function visible(selector) {
    const element = await browser.$(selector);
    await element.waitForDisplayed({ timeout: 15_000 });
    return element;
  }

  async function click(selector) {
    const element = await visible(selector);
    await element.waitForClickable({ timeout: 15_000 });
    await element.click();
  }

  async function snapshot() {
    return browser.execute((name) => {
      const text = localStorage.getItem(name);
      return text === null ? null : JSON.parse(text);
    }, key);
  }

  async function reloadWith(raw) {
    // The app flushes its current state on unload. Prepare the next window's
    // storage after that flush, rather than replacing any native command.
    await browser.execute((name, value) => {
      window.addEventListener("pagehide", () => {
        if (value === null) localStorage.removeItem(name);
        else localStorage.setItem(name, value);
      }, { once: true });
    }, key, raw);
    await browser.refresh();
    await visible('[data-testid="nav-settings"]');
  }

  async function openRun(name = fixture.runId) {
    const repo = await visible(`nav[data-testid="main-nav"] [data-sidebar-repo="${fixture.repo}"] > button`);
    if (await repo.getAttribute("aria-expanded") !== "true") await repo.click();
    const branch = await visible(`nav[data-testid="main-nav"] button[title="${fixture.branch}"]`);
    if (await branch.getAttribute("aria-expanded") !== "true") await branch.click();
    await click('//nav[@data-testid="main-nav"]//button[normalize-space(.)="Runs"]');
    await click(`main button[aria-label="Open run ${name}"]`);
    await browser.waitUntil(async () => await (await visible("main h1")).getText() === name,
      { timeout: 15_000 });
  }

  async function noRecoveryMessages() {
    assert.equal(await browser.$('[aria-label="Saved run controls"]').isExisting(), false);
    assert.equal(await browser.$('//main//button[normalize-space(.)="Clear run controls"]').isExisting(), false);
    const text = await browser.execute(() => document.body.innerText);
    assert.doesNotMatch(text, /Saved run controls|Restored saved|Status may be outdated|tracking was|Window refresh recovery/);
  }

  function visibleWindows() {
    return command("xdotool", ["search", "--onlyvisible", "--name", ".*"])
      .trim().split(/\s+/).filter(Boolean);
  }

  async function nativeConfirmation(selector, accept) {
    const before = new Set(visibleWindows());
    await click(selector);
    let dialog;
    await browser.waitUntil(() => {
      dialog = visibleWindows().filter((id) => !before.has(id)).pop();
      return Boolean(dialog);
    }, { timeout: 10_000, timeoutMsg: "Expected the real native confirmation dialog" });
    command("xdotool", ["windowfocus", "--sync", dialog, "key", "--clearmodifiers", accept ? "Return" : "Escape"]);
  }

  function addLogRun(name) {
    const directory = join(branchDirectory, name);
    mkdirSync(directory); // Never claim or remove an existing directory.
    ownedDirectories.push(directory);
    writeFileSync(join(directory, "stage_status.txt"), "implementation,FAILED\n");
    writeFileSync(join(directory, "01-implementation.log"), "Prepared refresh recovery fixture.\n");
    return directory;
  }

  try {
    await t.test("Refresh restores controls without messages or replaying workflows", async () => {
      await reloadWith(JSON.stringify({ version: 1, runs: [saved] }));
      await openRun();
      assert.equal(await (await visible(stop)).isEnabled(), true);
      assert.equal(await (await visible(continueButton)).isEnabled(), true);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      await noRecoveryMessages();
      await browser.saveScreenshot(join(artifacts, "run-controls-after-refresh.png"));

      // Exercise an ordinary second refresh without reseeding the snapshot.
      await browser.refresh();
      await visible('[data-testid="nav-settings"]');
      await openRun();
      assert.equal(await (await visible(stop)).isEnabled(), true);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      await noRecoveryMessages();
      assert.deepEqual(readdirSync(branchDirectory).sort(), originalRuns);
    });

    await t.test("Native stale-ID response clears Stop and preserves historical logs", async () => {
      await nativeConfirmation(stop, false);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      assert.equal(await (await visible(stop)).isEnabled(), true);

      // This UUID is a fixture, so the real backend must report it untracked.
      // Confirming Stop cannot interrupt any process owned by the user.
      await nativeConfirmation(stop, true);
      await browser.waitUntil(async () => !await browser.$(stop).isExisting(), { timeout: 15_000 });
      assert.equal(await snapshot(), null);
      assert.equal(await (await visible(continueButton)).isEnabled(), true);
      const alert = await visible('main [role="alert"]');
      assert.match(await alert.getText(), /This run is not tracked\./);
      assert.equal(await (await visible("main h1")).getText(), fixture.runId);
      await noRecoveryMessages();
    });

    await t.test("Saved stopping state permits a manual Stop retry without extra controls", async () => {
      await reloadWith(JSON.stringify({ version: 1, runs: [{ ...saved, state: "stopping" }] }));
      await openRun();
      // Saved stopping state permits a manual retry, never an automatic signal.
      assert.equal(await (await visible(stop)).isEnabled(), true);
      await nativeConfirmation(stop, false);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      assert.equal(await (await visible(stop)).isEnabled(), true);
      assert.equal(await (await visible(continueButton)).isEnabled(), true);
      assert.equal(await (await visible("main h1")).getText(), fixture.runId);
      await noRecoveryMessages();
    });

    await t.test("Invalid snapshots do not restore Stop or show storage messages", async () => {
      for (const raw of [
        "{",
        JSON.stringify({ version: 2, runs: [saved] }),
        JSON.stringify({ version: 1, runs: [saved, saved] }),
        JSON.stringify({ version: 1, runs: [{ ...saved, previousDirectories: [42] }] }),
        JSON.stringify({ version: 1, runs: [{ ...saved, id: "wrong-id" }] }),
        JSON.stringify({ version: 1, runs: [{ ...saved, directory: null }] }),
      ]) {
        await reloadWith(raw);
        await openRun();
        assert.equal(await browser.$(stop).isExisting(), false);
        assert.equal(await (await visible(continueButton)).isEnabled(), true);
        await noRecoveryMessages();
      }
    });

    await t.test("Resume restores its original run and ignores invalid sibling records", async () => {
      await reloadWith(JSON.stringify({ version: 1, runs: [
        { ...saved, mode: "resume" }, { ...saved, id: "invalid" },
      ] }));
      await openRun();
      assert.equal(await (await visible(stop)).isEnabled(), true);
      await noRecoveryMessages();
    });

    await t.test("Another branch's snapshot does not attach to a historical run", async () => {
      await reloadWith(JSON.stringify({ version: 1, runs: [{
        ...saved, branch: `${fixture.branch}-other`,
        directory: join(fixture.projectLogs, `${fixture.branch}-other`, fixture.runId),
      }] }));
      await openRun();
      assert.equal(await browser.$(stop).isExisting(), false);
      await noRecoveryMessages();
    });

    await t.test("Run discovers its log directory after an early refresh", async () => {
      await reloadWith(JSON.stringify({ version: 1, runs: [{ ...saved, runId: null, directory: null }] }));
      await browser.waitUntil(async () => (await snapshot())?.runs[0]?.directory === saved.directory,
        { timeout: 15_000 });
      await openRun();
      assert.equal(await (await visible(stop)).isEnabled(), true);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      await noRecoveryMessages();
    });

    await t.test("Continue discovers logs from its saved baseline after refresh", async () => {
      const name = "continue-2026-11-22_12-00-00";
      const directory = addLogRun(name);
      await reloadWith(JSON.stringify({ version: 1, runs: [{
        ...saved, mode: "continue", runId: null, directory: null,
        previousDirectories: [saved.directory],
      }] }));
      await browser.waitUntil(async () => (await snapshot())?.runs[0]?.directory === directory,
        { timeout: 15_000 });
      await openRun(name);
      assert.equal(await (await visible(stop)).isEnabled(), true);
      assert.equal((await snapshot()).runs[0].id, saved.id);
      await noRecoveryMessages();
    });

    await t.test("Ambiguous log discovery preserves the snapshot without guessing", async () => {
      const second = addLogRun("run-2026-11-22_12-00-01");
      await reloadWith(JSON.stringify({ version: 1, runs: [{ ...saved, runId: null, directory: null }] }));
      // Existing historical entries must not gain controls through guessed binding.
      await openRun();
      assert.equal(await browser.$(stop).isExisting(), false);
      assert.equal((await snapshot()).runs[0].runId, null);
      assert.equal((await snapshot()).runs[0].directory, null);
      assert.ok(readdirSync(branchDirectory).includes(second.split("/").pop()));
      await noRecoveryMessages();
    });

    assert.deepEqual(await browser.execute(() => ({
      theme: localStorage.getItem("soft-factory.theme"),
      language: localStorage.getItem("soft-factory.language"),
    })), preferences);
  } finally {
    try {
      await reloadWith(original);
    } finally {
      for (const directory of ownedDirectories) rmSync(directory, { recursive: true, force: true });
    }
  }
}
