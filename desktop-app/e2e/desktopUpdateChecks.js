import assert from "node:assert/strict";

export async function runDesktopUpdateChecks(browser) {
  // Use the actual native IPC bridge. The debug binary must never offer or
  // install production updates, even if the feed has a newer release.
  const result = await browser.execute(async () => {
    const update = await window.__TAURI_INTERNALS__.invoke("check_desktop_update");
    try {
      await window.__TAURI_INTERNALS__.invoke("install_desktop_update");
      return { update, installError: null };
    } catch (error) {
      return { update, installError: String(error) };
    }
  });
  assert.equal(result.update, null);
  assert.equal(result.installError, "No desktop update is available.");
}
