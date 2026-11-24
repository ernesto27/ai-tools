import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

export async function runScheduleChecks(browser, fixture, command, artifacts) {
  const registry = join(fixture.environment.HOME, ".config", "software-factory", "scheduled-tasks", "cron.json");
  const other = join(fixture.environment.HOME, "other-schedule-repo");
  const outside = join(fixture.environment.HOME, "outside prompt.txt");
  const registryEntries = {
    [fixture.repo]: [
      { name: "Weekday review", filePrompt: "task.txt", cron: "5 10 * * 1-5" },
      { name: "Legacy prompt", filePrompt: "context notes.md" },
      { name: "Monthly check", filePrompt: "task.txt", cron: "0 17 1 * *" },
    ],
    [other]: [{ name: "Other repo only", filePrompt: outside, cron: "0 9 * * *" }],
  };
  const apps = join(fixture.environment.XDG_DATA_HOME, "com.ernesto.desktop-app", "repos.json");
  const originalRepo = { name: fixture.repoName, path: fixture.repo };
  const otherRepo = { name: "Other schedule repo", path: other };
  command("git", ["init", "--initial-branch=main", other]);
  mkdirSync(dirname(registry), { recursive: true });
  writeFileSync(registry, JSON.stringify(registryEntries));
  writeFileSync(outside, "Prepared prompt outside the repository.\n");
  writeFileSync(apps, JSON.stringify([originalRepo, otherRepo]));

  const button = (label, within) =>
    within + '//button[normalize-space(.)="' + label + '"]';
  const mainButton = (label) => button(label, "//main");
  const modalButton = (label) => button(label, "//dialog[@open]");
  const action = (label) => 'main button[aria-label="' + label + '"]';
  async function click(selector) {
    const node = await browser.$(selector);
    await node.waitForClickable({ timeout: 15_000 });
    await node.click();
  }
  async function visible(selector) {
    await (await browser.$(selector)).waitForDisplayed({ timeout: 15_000 });
  }
  async function openSchedules(path) {
    const group = 'nav [data-sidebar-repo="' + path + '"]';
    const repo = await browser.$(group + " > button");
    await repo.waitForDisplayed({ timeout: 15_000 });
    if (await repo.getAttribute("aria-expanded") !== "true") await repo.click();
    const nav = await browser.$(group);
    const schedules = await nav.$('.//button[normalize-space(.)="Schedules"]');
    await schedules.waitForClickable({ timeout: 15_000 });
    await schedules.click();
  }

  await browser.refresh();
  await openSchedules(fixture.repo);
  await visible('//main//strong[.="Weekday review"]');
  await visible('//main//*[.="Timing unavailable"]');
  await visible('//main//code[.="0 17 1 * *"]');
  assert.equal(await browser.execute(() =>
    document.querySelector('nav button[aria-current="page"]')?.previousElementSibling?.textContent?.trim()
  ), "New run");
  assert.equal(await (await browser.$('//main//strong[.="Other repo only"]')).isExisting(), false);

  await browser.saveScreenshot(join(artifacts, "schedules.png"));
  await click(mainButton("New schedule"));
  await click(modalButton("Create schedule"));
  await visible('//dialog[@open]//*[@role="alert" and text()="Enter a schedule name."]');
  const name = await browser.$("dialog[open] input[type=text]");
  await name.setValue("Weekday review");
  await click(modalButton("Create schedule"));
  await visible('//dialog[@open]//*[@role="alert" and text()="A schedule with this name already exists."]');
  await name.setValue("Unsaved fixture schedule");
  await click(modalButton("Create schedule"));
  await visible('//dialog[@open]//*[@role="alert" and text()="Choose a prompt file."]');
  await click(modalButton("Browse"));

  const chooser = command("xdotool", ["search", "--sync", "--onlyvisible", "--name", "^Prompt file$"],
    { timeout: 10_000 }).trim();
  assert.match(chooser, /^\d+$/);
  command("xdotool", ["windowfocus", "--sync", chooser, "key", "--clearmodifiers", "ctrl+l", "ctrl+a"]);
  command("xdotool", ["type", "--clearmodifiers", outside + " "]);
  command("xdotool", ["key", "--clearmodifiers", "Return"]);
  await browser.waitUntil(async () => (await (await browser.$("dialog[open] output")).getText()) === outside,
    { timeout: 15_000 });
  const frequencySelector = 'dialog[open] select[name="scheduleFrequency"]';
  const frequency = await browser.$(frequencySelector);
  await frequency.selectByAttribute("value", "weekly");
  assert.equal((await browser.$$("dialog[open] select")).length, 4);
  const hourSelector = 'dialog[open] select[name="scheduleHour"]';
  const minuteSelector = 'dialog[open] select[name="scheduleMinute"]';
  const selectedTime = async () => [
    await (await browser.$(hourSelector)).getValue(),
    await (await browser.$(minuteSelector)).getValue(),
  ];
  assert.equal((await browser.$$(hourSelector + " option")).length, 24);
  assert.equal((await browser.$$(minuteSelector + " option")).length, 60);
  assert.deepEqual(await selectedTime(), ["09", "00"]);
  await (await browser.$(hourSelector)).selectByAttribute("value", "00");
  assert.deepEqual(await selectedTime(), ["00", "00"]);
  await (await browser.$(hourSelector)).selectByAttribute("value", "23");
  await (await browser.$(minuteSelector)).selectByAttribute("value", "59");
  assert.deepEqual(await selectedTime(), ["23", "59"]);
  await frequency.selectByAttribute("value", "custom");
  assert.equal(await (await browser.$(hourSelector)).isExisting(), false);
  assert.equal(await (await browser.$(minuteSelector)).isExisting(), false);
  await frequency.selectByAttribute("value", "weekly");
  assert.deepEqual(await selectedTime(), ["23", "59"]);
  await frequency.selectByAttribute("value", "custom");
  await (await browser.$("dialog[open] input.font-mono")).setValue("0 9 *");
  await click(modalButton("Create schedule"));
  await visible('//dialog[@open]//*[@role="alert" and text()="Enter five cron fields on one line."]');
  await click(modalButton("Cancel"));

  await click(action("Edit Weekday review"));
  assert.deepEqual(await selectedTime(), ["10", "05"]);
  await click(modalButton("Cancel"));

  await click(action("Edit Legacy prompt"));
  assert.equal(await (await browser.$(hourSelector)).isExisting(), false);
  assert.equal(await (await browser.$(frequencySelector)).getValue(), "keep");
  assert.equal(await (await browser.$("dialog[open] input[readonly]")).getValue(), "Legacy prompt");
  await click(modalButton("Cancel"));

  await click(action("Delete Weekday review"));
  await visible('//dialog[@open]//h2[.="Delete schedule"]');
  await click(modalButton("Cancel"));
  await browser.waitUntil(async () => await browser.execute(() =>
    document.activeElement?.getAttribute("aria-label") === "Delete Weekday review"), { timeout: 5_000 });
  await visible('//main//strong[.="Weekday review"]');

  await click(action("Delete Legacy prompt"));
  writeFileSync(registry, "{");
  await click(modalButton("Delete schedule"));
  await visible('//dialog[@open]//*[@role="alert" and starts-with(normalize-space(.),"Could not delete")]');
  await visible('//main//*[@role="alert" and starts-with(normalize-space(.),"Could not load schedules.")]');
  writeFileSync(registry, JSON.stringify(registryEntries));
  await click(modalButton("Delete schedule"));
  await visible('//dialog[@open]//*[@role="alert" and normalize-space(.)="Close this dialog and retry loading schedules before writing again."]');
  await click(modalButton("Cancel"));
  await click(mainButton("Retry"));
  await visible('//main//strong[.="Legacy prompt"]');

  await openSchedules(other);
  await visible('//main//strong[.="Other repo only"]');
  assert.equal(await (await browser.$('//main//strong[.="Weekday review"]')).isExisting(), false);

  writeFileSync(registry, "{");
  await openSchedules(fixture.repo);
  await visible("main [role=alert]");
  writeFileSync(registry, "{}");
  await click(mainButton("Retry"));
  await visible('//main//*[.="No schedules yet."]');

  writeFileSync(registry, JSON.stringify(registryEntries));
  await click('[data-testid="nav-settings"]');
  if (await browser.execute(() => document.documentElement.dataset.theme) === "dark") {
    await click('[data-testid="theme-toggle"]');
  }
  await (await browser.$("#language")).selectByAttribute("value", "es-AR");
  await click('[data-testid="settings-back"]');
  await visible('//main//h1[.="Programaciones"]');
  await visible('//main//*[.="Horario no disponible"]');
  await browser.setWindowSize(960, 680);
  await click(mainButton("Nueva programación"));
  await visible('//dialog[@open]//h2[.="Nueva programación"]');
  assert.equal(await browser.execute(() => {
    const dialog = document.querySelector("dialog[open]");
    const bounds = dialog?.getBoundingClientRect();
    return Boolean(bounds && bounds.left >= 0 && bounds.right <= window.innerWidth
      && bounds.top >= 0 && bounds.bottom <= window.innerHeight);
  }), true);
  await browser.saveScreenshot(join(artifacts, "new-schedule-es-AR.png"));
  const timeLabels = await browser.execute((selectors) =>
    selectors.map((selector) => document.querySelector(selector)?.labels[0]?.firstChild?.textContent?.trim()),
  [hourSelector, minuteSelector]);
  assert.deepEqual(timeLabels, ["Hora", "Minuto"]);
}
