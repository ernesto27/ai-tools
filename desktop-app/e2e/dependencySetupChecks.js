import assert from "node:assert/strict";

export async function runDependencySetupChecks(browser) {
  const navigation = await browser.$('nav[data-testid="main-nav"]');
  await navigation.waitForDisplayed({ timeout: 30_000 });
  const setup = await browser.$('[data-testid="dependency-setup"]');
  assert.equal(await setup.isExisting(), false, "Setup must unmount when both real tools are ready");
}
