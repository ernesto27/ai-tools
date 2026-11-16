import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { buildManifest, isNewer } from "./desktop-updates.mjs";

test("feed advances by semantic version and never downgrades", () => {
  assert.equal(isNewer("0.1.10", "0.1.9"), true);
  assert.equal(isNewer("1.0.0", "0.99.99"), true);
  assert.equal(isNewer("0.1.9", "0.1.10"), false);
  assert.equal(isNewer("0.1.10", "0.1.10"), false);
  for (const invalid of ["desktop-v1.2.3", "1.2.3-beta", "01.2.3", undefined]) {
    assert.throws(() => isNewer(invalid, "0.1.0"));
    assert.throws(() => isNewer("1.0.0", invalid));
  }
});

async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), "desktop-update-feed-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const names = [
    "soft-factory_0.1.7_linux_amd64.AppImage",
    "soft-factory_0.1.7_darwin_arm64.dmg",
    "soft-factory_0.1.7_darwin_arm64.app.tar.gz",
  ];
  const signature = Buffer.from("untrusted comment: fixture signature\nfixture\ntrusted comment: timestamp:1\tfile:fixture\tversion:0.1.7\nfixture\n").toString("base64");
  for (const name of names) {
    await writeFile(join(directory, name), `package ${name}`);
    const hash = createHash("sha256").update(`package ${name}`).digest("hex");
    await writeFile(join(directory, `${name}.sha256`), `${hash}  ${name}\n`);
    if (!name.endsWith(".dmg")) await writeFile(join(directory, `${name}.sig`), signature);
  }
  return { directory, names };
}

test("manifest selects the macOS updater archive and desktop-only artifact URLs", async (t) => {
  const { directory } = await fixture(t);
  const manifest = await buildManifest("0.1.7", "ernesto27/ai-tools", directory);
  assert.equal(manifest.version, "0.1.7");
  assert.deepEqual(Object.keys(manifest.platforms), ["linux-x86_64", "darwin-aarch64"]);
  assert.match(manifest.platforms["darwin-aarch64"].url, /desktop-v0\.1\.7\/.*\.app\.tar\.gz$/);
  assert.match(manifest.platforms["linux-x86_64"].url, /desktop-v0\.1\.7\/.*\.AppImage$/);
});

test("missing platform signatures prevent feed publication", async (t) => {
  const { directory, names } = await fixture(t);
  await rm(join(directory, `${names[2]}.sig`));
  await assert.rejects(buildManifest("0.1.7", "ernesto27/ai-tools", directory), /asset set/);
});

test("corrupt packages prevent feed publication", async (t) => {
  const { directory, names } = await fixture(t);
  await writeFile(join(directory, names[0]), "corrupt");
  await assert.rejects(buildManifest("0.1.7", "ernesto27/ai-tools", directory), /checksum/);
});

test("malformed signatures prevent feed publication", async (t) => {
  const { directory, names } = await fixture(t);
  await writeFile(join(directory, `${names[0]}.sig`), "placeholder");
  await assert.rejects(buildManifest("0.1.7", "ernesto27/ai-tools", directory), /signature encoding/);
});

test("an artifact signed for another version cannot advance the feed", async (t) => {
  const { directory, names } = await fixture(t);
  const signature = Buffer.from("untrusted comment: fixture\nfixture\ntrusted comment: timestamp:1\tversion:0.1.6\nfixture\n").toString("base64");
  await writeFile(join(directory, `${names[0]}.sig`), signature);
  await assert.rejects(buildManifest("0.1.7", "ernesto27/ai-tools", directory), /Signature version/);
});
