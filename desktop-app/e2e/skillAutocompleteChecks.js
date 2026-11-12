import assert from "node:assert/strict";
import { chmodSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { toggleThemeInSettings } from "./themeChecks.js";

export async function runSkillAutocompleteChecks(browser, fixture, artifacts) {
  const configPath = join(fixture.repo, "software-factory.json");
  const original = readFileSync(configPath, "utf8");
  // Only this function creates/owns these paths within the temporary fixture.
  const agents = join(fixture.repo, ".agents");
  const claude = join(fixture.repo, ".claude");
  const input = () => browser.$("#codeReview-skill");
  const nav = (label) => `//nav[@data-testid="main-nav"]//button[normalize-space(.)="${label}"]`;
  const save = () => browser.$('//main//header//button[normalize-space(.)="Save"]');
  async function reload(expectError = false) {
    await (await browser.$(nav("New run"))).click();
    await (await browser.$(nav("Config"))).click();
    if (expectError) {
      await (await browser.$("#skills-error")).waitForDisplayed({ timeout: 15_000 });
    } else {
      await browser.waitUntil(async () =>
        await (await input()).isEnabled(), { timeout: 15_000 });
    }
  }
  async function persist() {
    await (await save()).click();
    await browser.waitUntil(async () =>
      await (await browser.$('main header [role="status"]')).getText() === "Saved",
      { timeout: 15_000 });
    return JSON.parse(readFileSync(configPath, "utf8"));
  }
  async function labels() {
    const result = [];
    for (const option of await browser.$$("#codeReview-skill-suggestions [role=option]")) {
      result.push(await option.getProperty("innerText"));
    }
    return result;
  }
  async function captureDropdown() {
    const startingTheme = await browser.execute(() => document.documentElement.dataset.theme);
    const size = await browser.getWindowSize();
    try {
      for (const theme of ["light", "dark"]) {
        if (await browser.execute(() => document.documentElement.dataset.theme) !== theme) {
          await toggleThemeInSettings(browser);
        }
        for (const width of [size.width, 900]) {
          await browser.setWindowSize(width, size.height);
          await browser.waitUntil(async () => await (await input()).isEnabled(), { timeout: 15_000 });
          await (await input()).click();
          const fits = await browser.execute(() => {
            const bounds = document.getElementById("codeReview-skill-suggestions").getBoundingClientRect();
            return bounds.top >= 0 && bounds.bottom <= window.innerHeight
              && bounds.left >= 0 && bounds.right <= window.innerWidth;
          });
          assert.equal(fits, true, "Skill dropdown should fit in the viewport");
          await browser.saveScreenshot(join(artifacts, `skills-${theme}-${width}.png`));
          await browser.keys("Escape");
        }
      }
    } finally {
      await browser.setWindowSize(size.width, size.height);
      if (await browser.execute(() => document.documentElement.dataset.theme) !== startingTheme) {
        await toggleThemeInSettings(browser);
      }
    }
  }
  let ownsAgents = false;
  let ownsClaude = false;
  let completed = false;
  try {
    mkdirSync(agents);
    ownsAgents = true;
    mkdirSync(claude);
    ownsClaude = true;
    mkdirSync(join(agents, "skills", "shared"), { recursive: true });
    mkdirSync(join(claude, "skills", "claude-only"), { recursive: true });
    writeFileSync(join(agents, "skills", "shared", "SKILL.md"), "---\nname: shared\n---\nShared skill\n");
    writeFileSync(join(claude, "skills", "claude-only", "SKILL.md"), "---\nname: claude-only\n---\nClaude skill\n");
    symlinkSync("../../.agents/skills/shared", join(claude, "skills", "shared"));
    writeFileSync(configPath, JSON.stringify({
      ...JSON.parse(original), skillSelectorSentinel: { preserved: true },
    }));
    await reload();
    assert.equal(await (await browser.$("#implementation-skill")).isExisting(), false);
    for (const stage of ["codeReview", "securityReview", "riskClassification", "reviewChanges"]) {
      assert.equal(await (await browser.$(`#${stage}-skill`)).isEnabled(), true);
    }
    await (await input()).click();
    assert.deepEqual(await labels(), ["None", "claude-only", "shared"]);
    assert.equal(await (await browser.$("#codeReview-skill-suggestions-0")).getAttribute("aria-selected"), "true");
    await browser.keys("ArrowUp");
    assert.equal(await (await input()).getAttribute("aria-activedescendant"), "codeReview-skill-suggestions-2");
    assert.equal(await (await browser.$("#codeReview-skill-suggestions-2")).getAttribute("aria-selected"), "false");
    await browser.keys("ArrowDown");
    assert.equal(await (await input()).getAttribute("aria-activedescendant"), "codeReview-skill-suggestions-0");
    await (await input()).setValue("SHAR");
    assert.deepEqual(await labels(), ["None", "shared"]);
    await browser.keys("ArrowDown");
    assert.equal(await (await input()).getAttribute("aria-activedescendant"), "codeReview-skill-suggestions-0");
    await browser.keys("ArrowDown");
    assert.equal(await (await input()).getAttribute("aria-activedescendant"), "codeReview-skill-suggestions-1");
    await browser.keys("Enter");
    assert.equal(await (await input()).getValue(), "shared");
    const saved = await persist();
    assert.equal(saved.stages.codeReview.customSkill, "shared");
    assert.deepEqual(saved.skillSelectorSentinel, { preserved: true });
    await reload();
    assert.equal(await (await input()).getValue(), "shared");
    await captureDropdown();
    await (await input()).click();
    assert.equal(await (await browser.$("#codeReview-skill-suggestions-2")).getAttribute("aria-selected"), "true");
    await browser.keys("ArrowDown");
    assert.equal(await (await input()).getAttribute("aria-activedescendant"), "codeReview-skill-suggestions-0");
    assert.equal(await (await browser.$("#codeReview-skill-suggestions-0")).getAttribute("aria-selected"), "false");
    await browser.keys("Escape");

    await (await input()).setValue("unlisted-skill");
    await browser.keys("Enter");
    await browser.keys("Escape");
    assert.equal(await (await input()).getValue(), "shared");
    await (await input()).setValue("claude");
    await browser.keys("Tab");
    assert.equal(await (await input()).getValue(), "shared");
    assert.equal((await persist()).stages.codeReview.customSkill, "shared");

    await (await browser.$("#securityReview-skill")).click();
    await (await browser.$("#securityReview-skill-suggestions-1")).click();
    assert.equal((await persist()).stages.securityReview.customSkill, "claude-only");
    await (await browser.$('input[aria-label="Disable Code review"]')).click();
    assert.equal(await (await input()).isEnabled(), false);
    await (await browser.$('input[aria-label="Disable Code review"]')).click();

    rmSync(join(agents, "skills", "shared"), { recursive: true });
    await reload();
    assert.equal(await (await input()).getValue(), "shared");
    assert.equal(await (await browser.$("#codeReview-skill-unavailable")).getText(), "Unavailable in this repository");
    assert.equal((await persist()).stages.codeReview.customSkill, "shared");
    await (await input()).click();
    await browser.keys(["ArrowDown", "Enter"]);
    assert.equal(await (await input()).getValue(), "");
    assert.equal((await persist()).stages.codeReview.customSkill, undefined);

    // Exercise a real native read error, with permission restoration even on failure.
    if (process.getuid?.() !== 0) {
      const skillFolder = join(agents, "skills");
      chmodSync(skillFolder, 0o100);
      try {
        await reload(true);
        assert.equal(await (await input()).isEnabled(), false);
        const model = await browser.$("#default-model");
        assert.equal(await model.isEnabled(), true);
        await model.setValue("skill-error-model-check");
        assert.equal((await persist()).agentSandbox.model, "skill-error-model-check");
        await model.setValue("skill-retry-preserves-edits");
      } finally {
        chmodSync(skillFolder, 0o755);
      }
      await (await browser.$("#skills-error button")).click();
      await browser.waitUntil(async () => await (await input()).isEnabled(), { timeout: 15_000 });
      assert.equal(await (await browser.$("#default-model")).getValue(), "skill-retry-preserves-edits");
      assert.equal((await persist()).agentSandbox.model, "skill-retry-preserves-edits");
    }

    rmSync(join(agents, "skills"), { recursive: true });
    rmSync(join(claude, "skills"), { recursive: true });
    await reload();
    await (await input()).click();
    assert.deepEqual(await labels(), ["None"]);
    assert.equal(await (await browser.$("#codeReview-skill-suggestions")).getText(), "None");
    await browser.keys("Escape");
    completed = true;
  } finally {
    writeFileSync(configPath, original);
    if (ownsClaude) rmSync(claude, { recursive: true });
    if (ownsAgents) rmSync(agents, { recursive: true });
    if (completed) await reload();
  }
}
