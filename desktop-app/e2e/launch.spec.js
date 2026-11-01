import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import {
  closeSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createConnection, createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { remote } from "webdriverio";

const testDirectory = dirname(fileURLToPath(import.meta.url));
const appDirectory = resolve(testDirectory, "..");

function command(executable, args, options = {}) {
  const result = spawnSync(executable, args, {
    cwd: appDirectory,
    encoding: "utf8",
    timeout: 30_000,
    ...options,
  });
  if (result.error || result.status !== 0) {
    throw new Error(
      `${executable} ${args.join(" ")} failed: ` +
      `${result.error?.message ?? `exit ${result.status}, signal ${result.signal ?? "none"}`}\n` +
      `${result.stderr ?? ""}`,
    );
  }
  return result.stdout;
}

async function requireFreePort(port) {
  await new Promise((resolvePort, reject) => {
    const server = createServer();
    server.once("error", (error) => {
      reject(new Error(`Port ${port} is unavailable; stop the other test/driver.`, { cause: error }));
    });
    server.listen(port, "127.0.0.1", () => server.close(resolvePort));
  });
}

async function portIsOpen(port) {
  return new Promise((resolvePort) => {
    const socket = createConnection({ host: "127.0.0.1", port });
    const finish = (open) => {
      socket.destroy();
      resolvePort(open);
    };
    socket.setTimeout(200);
    socket.once("connect", () => finish(true));
    socket.once("error", () => finish(false));
    socket.once("timeout", () => finish(false));
  });
}

function signalGroup(child, signal) {
  if (!child?.pid) return;
  try {
    process.kill(-child.pid, signal);
  } catch (error) {
    if (error.code !== "ESRCH") throw error;
  }
}

test("launch shows Select a repository", { timeout: 900_000 }, async (t) => {
  let fixture;
  let driver;
  let browser;
  let artifacts;

  t.after(async () => {
    const failures = [];
    if (browser) {
      try {
        await browser.deleteSession();
      } catch (error) {
        failures.push(error);
      }
    }
    try {
      signalGroup(driver, "SIGTERM");
      if (driver?.pid) {
        await delay(500);
        signalGroup(driver, "SIGKILL");
      }
    } catch (error) {
      failures.push(error);
    }
    try {
      if (fixture) rmSync(fixture, { recursive: true, force: true });
    } catch (error) {
      failures.push(error);
    }
    if (failures.length) throw new AggregateError(failures, "Desktop test cleanup failed");
  });

  try {
    assert.equal(process.platform, "linux", "Desktop tests currently support Linux only.");
    const [major, minor] = process.versions.node.split(".").map(Number);
    assert.ok(major > 22 || (major === 22 && minor >= 19), "Use Node 22.19 or newer.");
    assert.ok(process.env.DISPLAY, "An X11 DISPLAY is required. Try: xvfb-run -a npm run test:desktop");

    for (const executable of ["git", "software-factory", "agent-sandbox", "tauri-driver", "WebKitWebDriver"]) {
      command(executable, ["--help"]);
    }
    await requireFreePort(4444);
    await requireFreePort(4445);

    command("npm", ["run", "tauri", "--", "build", "--debug", "--no-bundle"], {
      timeout: 600_000,
      stdio: "inherit",
      env: { ...process.env, CARGO_TARGET_DIR: join(appDirectory, "src-tauri", "target") },
    });

    fixture = mkdtempSync(join(tmpdir(), "soft-factory-desktop-e2e-"));
    t.diagnostic(`Temporary desktop test fixture: ${fixture}`);
    const repo = join(fixture, "launch-repository");
    mkdirSync(repo);
    command("git", ["init", "--initial-branch=main", repo]);
    writeFileSync(join(repo, "README.md"), "# Desktop launch test repository\n");
    command("git", ["-C", repo, "add", "README.md"]);
    command("git", [
      "-C", repo,
      "-c", "user.name=Desktop UI Test",
      "-c", "user.email=desktop-ui-test@example.invalid",
      "-c", "commit.gpgsign=false",
      "-c", "core.hooksPath=/dev/null",
      "commit", "-m", "Initial desktop UI fixture",
    ]);

    const dataHome = join(fixture, "data");
    const appData = join(dataHome, "com.ernesto.desktop-app");
    mkdirSync(appData, { recursive: true });
    writeFileSync(join(appData, "repos.json"), JSON.stringify([
      { name: "launch-repository", path: repo },
    ], null, 2));
    const environment = { ...process.env, XDG_DATA_HOME: dataHome, GDK_BACKEND: "x11" };
    const branches = JSON.parse(command(
      "agent-sandbox", ["worktree-list", "--json", "--repo", repo], { env: environment },
    ));
    assert.ok(branches === null || Array.isArray(branches), "agent-sandbox must return a worktree array or null.");

    mkdirSync(join(testDirectory, "artifacts"), { recursive: true });
    artifacts = mkdtempSync(join(testDirectory, "artifacts", "launch-"));
    t.diagnostic(`Desktop test artifacts: ${artifacts}`);
    const log = openSync(join(artifacts, "driver.log"), "a");
    let driverError;
    try {
      driver = spawn("tauri-driver", [], {
        cwd: appDirectory,
        env: environment,
        detached: true,
        stdio: ["ignore", log, log],
      });
      driver.on("error", (error) => { driverError = error; });
      t.diagnostic(`Desktop test driver process group: ${driver.pid}`);
    } finally {
      closeSync(log);
    }

    const deadline = Date.now() + 10_000;
    while (!(await portIsOpen(4444))) {
      if (driverError) throw driverError;
      if (driver.exitCode !== null || driver.signalCode !== null) {
        throw new Error(`tauri-driver exited before startup; inspect ${artifacts}/driver.log`);
      }
      if (Date.now() >= deadline) throw new Error(`tauri-driver startup timed out; inspect ${artifacts}/driver.log`);
      await delay(100);
    }

    browser = await remote({
      hostname: "127.0.0.1",
      port: 4444,
      path: "/",
      logLevel: "error",
      connectionRetryCount: 0,
      connectionRetryTimeout: 30_000,
      capabilities: {
        "tauri:options": {
          application: join(appDirectory, "src-tauri", "target", "debug", "desktop-app"),
        },
      },
    });
    const heading = await browser.$("main h1");
    await heading.waitForDisplayed({ timeout: 15_000 });
    assert.equal(await heading.getText(), "Select a repository");
  } catch (error) {
    if (browser && artifacts) {
      const results = await Promise.allSettled([
        browser.saveScreenshot(join(artifacts, "failure.png")),
        browser.getPageSource().then((source) => writeFileSync(join(artifacts, "failure.html"), source)),
      ]);
      for (const result of results) {
        if (result.status === "rejected") t.diagnostic(`Could not capture diagnostic: ${result.reason}`);
      }
    }
    throw error;
  }
});
