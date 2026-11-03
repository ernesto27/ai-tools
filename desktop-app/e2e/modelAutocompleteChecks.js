import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function runModelAutocompleteChecks(browser, fixture, artifacts) {
  const configPath = join(fixture.repo, "software-factory.json");
  const original = readFileSync(configPath, "utf8");
  const nav = (label) => `//nav[@data-testid="main-nav"]//button[normalize-space(.)="${label}"]`;
  const sharedAgent = '(//main//label[normalize-space(text())="Agent"]/select)[1]';
  const model = () => browser.$("#default-model");
  const stage = () => browser.$("#implementation-model");
  const stageAgent = () => browser.$('fieldset[aria-label="Implement settings"] > label:first-child select');
  const options = () => browser.$$("#default-model-suggestions [role=option]");
  const save = () => browser.$('//main//header//button[normalize-space(.)="Save"]');
  let completed = false;

  async function reloadConfig() {
    await (await browser.$(nav("New run"))).click();
    await (await browser.$(nav("Config"))).click();
    await browser.waitUntil(async () =>
      await (await browser.$('main header [role="status"]')).getText() === "Saved",
      { timeout: 15_000 });
  }

  async function chooseAgent(value) {
    await (await browser.$(sharedAgent)).selectByAttribute("value", value);
  }

  async function ids() {
    const values = [];
    for (const option of await options()) values.push(await option.getProperty("innerText"));
    return values;
  }

  async function captureDropdown(input, name) {
    const startingTheme = await browser.execute(() => document.documentElement.dataset.theme);
    const startingSize = await browser.getWindowSize();
    const toggle = await browser.$('[data-testid="theme-toggle"]');
    for (const theme of ["light", "dark"]) {
      if (await browser.execute(() => document.documentElement.dataset.theme) !== theme) {
        await toggle.click();
      }
      await input.click();
      await assertPopupInViewport(input);
      await browser.saveScreenshot(join(artifacts, `model-autocomplete-${name}-${theme}.png`));
      await browser.setWindowSize(900, startingSize.height);
      await input.click();
      await assertPopupInViewport(input);
      await browser.saveScreenshot(join(artifacts, `model-autocomplete-${name}-${theme}-narrow.png`));
      await browser.setWindowSize(startingSize.width, startingSize.height);
    }
    if (await browser.execute(() => document.documentElement.dataset.theme) !== startingTheme) {
      await toggle.click();
    }
    await input.click();
  }

  async function assertPopupInViewport(input) {
    const listId = await input.getAttribute("aria-controls");
    const inViewport = await browser.execute((id) => {
      const bounds = document.getElementById(id).getBoundingClientRect();
      return bounds.top >= 0 && bounds.bottom <= window.innerHeight
        && bounds.left >= 0 && bounds.right <= window.innerWidth;
    }, listId);
    assert.equal(inViewport, true, "Model suggestions should fit in the viewport");
  }

  try {
    await (await model()).click();
    assert.deepEqual(await ids(), ["gpt-6-astra", "gpt-6.1-sol", "gpt-6.1-terra", "gpt-6-luna"]);
    await captureDropdown(await model(), "shared");
    for (const key of ["implementation", "codeReview", "securityReview", "riskClassification", "reviewChanges"]) {
      const input = await browser.$(`#${key}-model`);
      await input.click();
      assert.equal(await input.getAttribute("aria-label"), "Model");
      assert.equal(await input.getAttribute("aria-controls"), `${key}-model-suggestions`);
      assert.equal(await (await browser.$(`#${key}-model-suggestions [role=option]:first-child`)).getProperty("innerText"), "gpt-6-astra");
      await browser.keys("ArrowUp");
      assert.equal(await input.getAttribute("aria-activedescendant"), `${key}-model-suggestions-3`);
      await browser.keys("ArrowDown");
      assert.equal(await input.getAttribute("aria-activedescendant"), `${key}-model-suggestions-0`);
      await browser.keys("Escape");
      assert.equal(await input.getValue(), "");
    }
    await (await model()).setValue("SOL");
    assert.deepEqual(await ids(), ["gpt-6.1-sol"]);
    await browser.keys(["ArrowDown", "Enter"]);
    assert.equal(await (await model()).getValue(), "gpt-6.1-sol");
    assert.equal(await (await model()).getAttribute("aria-expanded"), "false");
    await (await model()).click();
    assert.equal(await (await model()).getAttribute("aria-expanded"), "true");
    await browser.keys("Escape");

    await chooseAgent("claude");
    assert.equal(await (await model()).getValue(), "gpt-6.1-sol");
    await (await model()).setValue("claude-");
    assert.deepEqual(await ids(), ["claude-fable-5-1", "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"]);
    await (await browser.$('#default-model-suggestions [role=option]:first-child')).click();
    assert.equal(await (await model()).getValue(), "claude-fable-5-1");
    assert.equal(await (await model()).getAttribute("aria-expanded"), "false");

    // Empty stage agent inherits shared Claude; choosing Codex overrides it.
    await (await stageAgent()).selectByAttribute("value", "");
    await (await stage()).click();
    assert.equal(await (await browser.$("#implementation-model-suggestions [role=option]:first-child")).getProperty("innerText"), "claude-fable-5-1");
    await browser.keys("Escape");
    await (await stageAgent()).selectByAttribute("value", "codex");
    await (await stage()).click();
    assert.equal(await (await browser.$("#implementation-model-suggestions [role=option]:first-child")).getProperty("innerText"), "gpt-6-astra");
    await captureDropdown(await stage(), "stage");
    await browser.keys(["ArrowUp", "Enter"]);
    assert.equal(await (await stage()).getValue(), "gpt-6-luna");

    // Custom text and Enter without a highlighted option never force a match.
    await (await model()).setValue("custom-model-desktop-check");
    assert.equal(await (await browser.$('#default-model-suggestions [role=option]')).isExisting(), false);
    await browser.keys("Enter");
    assert.equal(await (await model()).getValue(), "custom-model-desktop-check");
    await browser.keys("Escape");
    assert.equal(await (await model()).getAttribute("aria-expanded"), "false");
    await (await model()).click();
    await browser.keys("Tab");
    assert.equal(await (await model()).getAttribute("aria-expanded"), "false");

    await (await save()).click();
    await browser.waitUntil(async () =>
      await (await browser.$('main header [role="status"]')).getText() === "Saved",
      { timeout: 15_000 });
    const saved = JSON.parse(readFileSync(configPath, "utf8"));
    assert.equal(saved.agentSandbox.model, "custom-model-desktop-check");
    assert.equal(saved.stages.implementation.model, "gpt-6-luna");
    await reloadConfig();
    assert.equal(await (await model()).getValue(), "custom-model-desktop-check");
    assert.equal(await (await stage()).getValue(), "gpt-6-luna");

    await chooseAgent("opencode");
    await (await model()).click();
    assert.equal(await (await model()).getAttribute("aria-expanded"), "false");
    assert.equal(await (await model()).getValue(), "custom-model-desktop-check");

    await (await browser.$('input[aria-label="Disable Implement"]')).click();
    assert.equal(await (await stage()).isEnabled(), false);
    assert.equal(await (await stage()).getAttribute("aria-expanded"), "false");

    // Clearing keeps existing inheritance/omission semantics.
    await chooseAgent("codex");
    await (await model()).click();
    await browser.keys(["Control", "a"]);
    await browser.keys("Backspace");
    assert.equal(await (await model()).getValue(), "");
    await (await save()).click();
    await browser.waitUntil(async () =>
      await (await browser.$('main header [role="status"]')).getText() === "Saved",
      { timeout: 15_000 });
    assert.equal(JSON.parse(readFileSync(configPath, "utf8")).agentSandbox.model, undefined);
    completed = true;
  } finally {
    // This path belongs exclusively to the temporary navigation fixture.
    writeFileSync(configPath, original);
    if (completed) await reloadConfig();
  }
}
