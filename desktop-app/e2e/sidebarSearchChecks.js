import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

export async function runSidebarSearchChecks(browser, fixture, artifacts, command) {
  const root = dirname(fixture.repo);
  const reposFile = join(fixture.environment.XDG_DATA_HOME, "com.ernesto.desktop-app", "repos.json");
  const registryFile = join(fixture.environment.XDG_CONFIG_HOME, "agent-sandbox", "worktrees.jsonl");
  const originalRepos = readFileSync(reposFile, "utf8");
  const originalRegistry = readFileSync(registryFile, "utf8");
  const secondRepo = join(root, "search-library");
  const sibling = "feature/payments";
  const siblingPath = join(root, "search-payments-worktree");
  const secondPath = join(root, "search-library-worktree");
  const secondBranch = "library-check";
  const languageKey = "soft-factory.language";
  const originalLanguage = await browser.execute((key) => localStorage.getItem(key), languageKey);
  const search = () => browser.$("#sidebar-search");
  const repoSelector = (name) => `//nav[@id="main-sidebar"]//button[@aria-haspopup="menu"][.//span[@title="${name}"]]`;
  const branchSelector = (name) => `#main-sidebar button[title="${name}"]`;
  const nav = (label) => `//nav[@id="main-sidebar"]//button[normalize-space(.)="${label}"]`;

  async function ready() {
    await (await search()).waitForDisplayed({ timeout: 15_000 });
  }

  async function shown(selector, expected = true) {
    await browser.waitUntil(async () => {
      const element = await browser.$(selector);
      return (await element.isExisting() && await element.isDisplayed()) === expected;
    }, { timeout: 15_000, timeoutMsg: `Visibility ${expected}: ${selector}` });
  }

  async function expanded(selector, expected) {
    await browser.waitUntil(async () =>
      await (await browser.$(selector)).getAttribute("aria-expanded") === String(expected),
    { timeout: 15_000, timeoutMsg: `Expansion ${expected}: ${selector}` });
  }

  async function type(value) {
    await (await search()).setValue(value);
  }

  async function clear() {
    await (await browser.$('[data-testid="sidebar-search-clear"]')).click();
    assert.equal(await (await search()).getValue(), "");
    assert.equal(await browser.execute(() => document.activeElement?.id), "sidebar-search");
  }

  async function language(value) {
    await (await browser.$('[data-testid="nav-settings"]')).click();
    await (await browser.$("#language")).waitForDisplayed({ timeout: 15_000 });
    await (await browser.$("#language")).selectByAttribute("value", value);
    await browser.waitUntil(async () =>
      await browser.execute(() => document.documentElement.lang) === value,
    { timeout: 5_000 });
  }

  try {
    command("git", ["-C", fixture.repo, "worktree", "add", "-b", sibling, siblingPath]);
    command("git", ["clone", "--no-hardlinks", fixture.repo, secondRepo]);
    command("git", ["-C", secondRepo, "worktree", "add", "-b", secondBranch, secondPath]);
    const records = [
      { repo: fixture.repo, path: siblingPath, branch: sibling, base_branch: "main", created: "2026-11-06T12:00:00Z" },
      { repo: secondRepo, path: secondPath, branch: secondBranch, base_branch: "main", created: "2026-11-06T12:00:00Z" },
    ];
    writeFileSync(registryFile, originalRegistry + records.map((record) => JSON.stringify(record) + "\n").join(""));
    writeFileSync(reposFile, JSON.stringify([
      ...JSON.parse(originalRepos),
      { name: "search-library", path: secondRepo },
    ], null, 2) + "\n");
    for (const record of records) {
      const entries = JSON.parse(command("agent-sandbox", ["worktree-list", "--json", "--repo", record.repo],
        { env: fixture.environment }));
      assert.ok(entries.some((entry) => entry.branch === record.branch && entry.path === record.path));
    }

    await browser.refresh();
    await ready();
    const primary = repoSelector(fixture.repoName);
    const secondary = repoSelector("search-library");
    const originalBranch = branchSelector(fixture.branch);
    await shown(primary);
    await shown(secondary);
    await expanded(primary, false);
    await expanded(secondary, false);

    // Case-insensitive branch substring reveals a collapsed repository.
    await type("  PAY  ");
    await shown(branchSelector(sibling));
    await shown(originalBranch, false);
    await shown(secondary, false);
    await expanded(primary, true);
    await (await browser.$(primary)).click();
    await expanded(primary, true);
    await shown(nav("Config"));
    await shown(nav("New run"));
    await clear();
    await expanded(primary, false);

    // Capture an open repository/branch, then alter only search expansion.
    await (await browser.$(primary)).click();
    await shown(originalBranch);
    await (await browser.$(originalBranch)).click();
    await expanded(originalBranch, true);
    await type("PAY");
    await shown(branchSelector(sibling));
    await (await browser.$(branchSelector(sibling))).click();
    await browser.waitUntil(async () =>
      await (await browser.$("main h1")).getProperty("innerText") === sibling,
    { timeout: 15_000 });
    assert.equal(await (await search()).getValue(), "PAY");
    await clear();
    await expanded(primary, true);
    await expanded(originalBranch, true);
    await expanded(branchSelector(sibling), false);

    // Repository matches expose every branch; broader branch matches span repos.
    await type("REPOSITORY");
    await shown(originalBranch);
    await shown(branchSelector(sibling));
    await shown(secondary, false);
    await type("CHECK");
    await shown(originalBranch);
    await shown(branchSelector(secondBranch));
    await shown(branchSelector(sibling), false);
    const repoOrder = await browser.$$("#main-sidebar [data-sidebar-repo]");
    assert.deepEqual(await Promise.all(repoOrder.map((element) => element.getAttribute("data-sidebar-repo"))),
      [fixture.repo, secondRepo]);
    await browser.saveScreenshot(join(artifacts, "sidebar-search-multiple-repositories.png"));

    await type("no-such-sidebar-match");
    await shown('[data-testid="sidebar-search-empty"]');
    assert.equal(await (await browser.$('[data-testid="sidebar-search-empty"]')).getProperty("innerText"),
      "No matching repositories or branches");
    await shown(primary, false);
    await shown(secondary, false);
    await shown(nav("Add repository"));
    await shown('[data-testid="nav-settings"]');
    await (await search()).click();
    await browser.keys("Escape");
    assert.equal(await (await search()).getValue(), "");
    assert.equal(await browser.execute(() => document.activeElement?.id), "sidebar-search");
    await shown('[data-testid="sidebar-search-empty"]', false);
    await expanded(originalBranch, true);

    await type("   ");
    await shown(primary);
    await shown(secondary);
    await expanded(primary, true);
    await expanded(secondary, false);
    await shown('[data-testid="sidebar-search-empty"]', false);
    await clear();

    // Search lifetime across settings and sidebar visibility.
    await type("PAY");
    await (await browser.$('[data-testid="nav-settings"]')).click();
    assert.equal(await (await search()).getValue(), "PAY");
    const toggle = await browser.$('button[aria-controls="main-sidebar"]');
    await toggle.click();
    await shown("#main-sidebar", false);
    await toggle.click();
    await ready();
    assert.equal(await (await search()).getValue(), "PAY");

    await language("es-AR");
    assert.equal(await (await search()).getAttribute("placeholder"), "Buscar repositorios y ramas");
    assert.equal(await (await search()).getAttribute("aria-label"), "Buscar repositorios y ramas");
    assert.equal(await (await browser.$('[data-testid="sidebar-search-clear"]')).getAttribute("aria-label"),
      "Limpiar búsqueda");
    await type("no-such-sidebar-match");
    await browser.waitUntil(async () =>
      await (await browser.$('[data-testid="sidebar-search-empty"]')).getProperty("innerText")
        === "No hay repositorios ni ramas que coincidan",
    { timeout: 5_000 });
    await browser.saveScreenshot(join(artifacts, "sidebar-search-empty-es-AR.png"));
    await language("en");
    assert.equal(await (await search()).getAttribute("placeholder"), "Search repositories and branches");
    assert.equal(await (await search()).getAttribute("aria-label"), "Search repositories and branches");
    assert.equal(await (await browser.$('[data-testid="sidebar-search-clear"]')).getAttribute("aria-label"),
      "Clear search");
    await browser.refresh();
    await ready();
    assert.equal(await (await search()).getValue(), "");
  } finally {
    writeFileSync(reposFile, originalRepos);
    writeFileSync(registryFile, originalRegistry);
    await browser.execute((key, value) => {
      if (value === null) localStorage.removeItem(key);
      else localStorage.setItem(key, value);
    }, languageKey, originalLanguage);
    await browser.refresh();
    await ready();
  }
}
