import assert from "node:assert/strict";
import { join } from "node:path";

export async function runLanguageChecks(browser, fixture, artifacts) {
  const key = "soft-factory.language";
  const original = await browser.execute((name) => localStorage.getItem(name), key);
  const visible = async (selector) => {
    const element = await browser.$(selector);
    await element.waitForDisplayed({ timeout: 15_000 });
    return element;
  };
  const expectText = async (selector, expected) => {
    await browser.waitUntil(async () =>
      await (await visible(selector)).getProperty("innerText") === expected,
    { timeout: 15_000, timeoutMsg: `Expected ${selector}: ${expected}` });
  };
  const nav = (label) => `//nav[@data-testid="main-nav"]//button[normalize-space(.)="${label}"]`;
  const click = async (selector) => {
    const element = await visible(selector);
    await element.waitForClickable({ timeout: 15_000 });
    await element.click();
  };
  const ready = () => visible('[data-testid="nav-settings"]');
  const expectLanguage = async (language) => {
    await browser.waitUntil(async () =>
      await browser.execute(() => document.documentElement.lang) === language,
    { timeout: 5_000 });
  };
  const openRepo = async () => {
    const selector = 'nav[data-testid="main-nav"] button[aria-haspopup="menu"]';
    const element = await visible(selector);
    if (await element.getAttribute("aria-expanded") !== "true") await click(selector);
  };
  const openBranch = async () => {
    await openRepo();
    const selector = `nav[data-testid="main-nav"] button[title="${fixture.branch}"]`;
    const element = await visible(selector);
    if (await element.getAttribute("aria-expanded") !== "true") await click(selector);
  };

  try {
    await ready();
    await expectLanguage("en");
    await click('[data-testid="nav-settings"]');
    await expectText("main h1", "Settings");
    assert.equal(await (await visible("#language")).getValue(), "en");
    const theme = await browser.execute(() => document.documentElement.dataset.theme);
    await (await visible("#language")).selectByAttribute("value", "es-AR");
    await expectLanguage("es-AR");
    await expectText("main h1", "Configuración");
    assert.equal(await browser.execute((name) => localStorage.getItem(name), key), "es-AR");
    assert.equal(await browser.execute(() => document.documentElement.dataset.theme), theme);
    await browser.saveScreenshot(join(artifacts, "settings-es-AR.png"));

    await browser.refresh();
    await ready();
    await expectLanguage("es-AR");
    await expectText("main h1", "Seleccioná un repositorio");
    await click('[data-testid="nav-settings"]');
    assert.equal(await (await visible("#language")).getValue(), "es-AR");

    await click(nav("Agregar repositorio"));
    await expectText("main h1", "Agregar repositorio");
    await openRepo();
    await click(nav("Config"));
    await expectText("main h1", "Config");
    await expectText('//main//h2[normalize-space(.)="Valores predeterminados del agente"]',
      "Valores predeterminados del agente");
    assert.equal(await (await visible('#default-model')).getAttribute("aria-label"), "Modelo");
    const skill = await visible("#codeReview-skill");
    await browser.waitUntil(async () => await skill.isEnabled(), { timeout: 15_000 });
    assert.equal(await skill.getAttribute("aria-label"), "Skill personalizada");
    assert.equal(await skill.getAttribute("placeholder"), "Ninguna");
    await skill.click();
    await expectText("#codeReview-skill-suggestions-0", "Ninguna");
    await browser.keys("Escape");
    await browser.saveScreenshot(join(artifacts, "config-es-AR.png"));

    await openBranch();
    await click(nav("Ejecuciones"));
    await expectText("#runs-heading", "1 ejecución");
    await expectText("main article h3", fixture.runId);
    await expectText("main article dd", "Fallida");

    for (const [label, expectedMode] of [["Nueva ejecución", "Ejecutar"],
      ["Continuar", "Continuar"], ["Reanudar", "Reanudar"]]) {
      await click(nav(label));
      await expectText("main h1", "Nueva ejecución");
      await expectText('main [role="tablist"][aria-label="Modo"] [role="tab"][aria-selected="true"]', expectedMode);
      await expectText("#config-preview-h", "Vista previa de software-factory.json");
    }

    await click(nav("Ejecuciones"));
    await click(`main button[aria-label="Abrir ejecución ${fixture.runId}"]`);
    await expectText("main h1", fixture.runId);
    await expectText('//main//button[span[normalize-space(.)="Implementar"]]/span[2]', "Fallida");
    await expectText('main span[aria-label="Duración de Implementar"]', "Duración no disponible");
    const output = await visible('main [role="region"][aria-label="Contenido de 01-implementation.log"]');
    assert.ok((await output.getProperty("innerText")).includes(fixture.logText));
    await browser.saveScreenshot(join(artifacts, "run-detail-es-AR.png"));

    await click('[data-testid="nav-settings"]');
    await (await visible("#language")).selectByAttribute("value", "en");
    await expectLanguage("en");
    await expectText("main h1", "Settings");
    await browser.refresh();
    await ready();
    await expectLanguage("en");

    for (const value of [null, "", "unsupported", "es", "es_AR"]) {
      await browser.execute((name, saved) => {
        if (saved === null) localStorage.removeItem(name);
        else localStorage.setItem(name, saved);
      }, key, value);
      await browser.refresh();
      await ready();
      await expectLanguage("en");
      await expectText("main h1", "Select a repository");
    }
  } finally {
    await browser.execute((name, saved) => {
      if (saved === null) localStorage.removeItem(name);
      else localStorage.setItem(name, saved);
    }, key, original);
    await browser.refresh();
    await ready();
  }
}
