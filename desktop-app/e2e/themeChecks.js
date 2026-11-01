import assert from "node:assert/strict";
import { join } from "node:path";

export async function runThemeChecks(browser, artifacts) {
  const key = "soft-factory.theme";
  const original = await browser.execute((name) => localStorage.getItem(name), key);
  const state = () => browser.execute(() => ({
    theme: document.documentElement.dataset.theme,
    background: getComputedStyle(document.body).backgroundColor,
    scheme: getComputedStyle(document.documentElement).colorScheme,
  }));
  const ready = async () => {
    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]'))
      .waitForDisplayed({ timeout: 15_000 });
  };
  const expectTheme = async (theme) => {
    await browser.waitUntil(async () => (await state()).theme === theme,
      { timeout: 5_000 });
    assert.equal((await state()).scheme, theme);
    const toggle = await browser.$('nav [role="switch"][aria-label="Dark mode"]');
    assert.equal(await toggle.getAttribute("aria-checked"), String(theme === "dark"));
    assert.equal((await state()).background,
      theme === "dark" ? "rgb(17, 17, 16)" : "rgb(255, 255, 255)");
  };
  try {
    await ready();
    await browser.execute((name) => localStorage.setItem(name, "dark"), key);
    await browser.refresh();
    await ready();
    await expectTheme("dark");
    await browser.saveScreenshot(join(artifacts, "theme-dark.png"));

    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]')).click();
    await expectTheme("light");
    assert.equal(await browser.execute((name) => localStorage.getItem(name), key), "light");
    await browser.saveScreenshot(join(artifacts, "theme-light.png"));
    await browser.refresh();
    await ready();
    await expectTheme("light");

    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]')).click();
    await expectTheme("dark");
    assert.equal(await browser.execute((name) => localStorage.getItem(name), key), "dark");
    await browser.refresh();
    await ready();
    await expectTheme("dark");

    for (const invalid of [null, "invalid"]) {
      await browser.execute((name, value) => {
        if (value === null) localStorage.removeItem(name);
        else localStorage.setItem(name, value);
      }, key, invalid);
      await browser.refresh();
      await ready();
      const expected = await browser.execute(() =>
        matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
      await expectTheme(expected);
    }
  } finally {
    await browser.execute((name, value) => {
      if (value === null) localStorage.removeItem(name);
      else localStorage.setItem(name, value);
    }, key, original);
    await browser.refresh();
    await ready();
  }
}
