import { createHash } from "node:crypto";
import { readFile, readdir, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { pathToFileURL } from "node:url";

const versionPattern = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;

function versionParts(version) {
  if (typeof version !== "string" || !versionPattern.test(version)) {
    throw new Error(`Invalid stable desktop version: ${version}`);
  }
  return version.split(".").map(BigInt);
}

export function isNewer(candidate, previous) {
  const next = versionParts(candidate);
  const current = versionParts(previous);
  for (let index = 0; index < next.length; index += 1) {
    if (next[index] !== current[index]) return next[index] > current[index];
  }
  return false;
}

export async function buildManifest(version, repository, directory) {
  versionParts(version);
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)) {
    throw new Error("Expected GitHub owner/repository.");
  }
  const linux = `soft-factory_${version}_linux_amd64.AppImage`;
  const macInstaller = `soft-factory_${version}_darwin_arm64.dmg`;
  const macUpdate = `soft-factory_${version}_darwin_arm64.app.tar.gz`;
  const packages = [linux, macInstaller, macUpdate];
  const expected = packages.flatMap((name) => [name, `${name}.sha256`])
    .concat([`${linux}.sig`, `${macUpdate}.sig`]).sort();
  const files = (await readdir(directory)).sort();
  if (JSON.stringify(files) !== JSON.stringify(expected)) {
    throw new Error(`Unexpected release asset set: ${files.join(", ")}`);
  }
  for (const name of packages) {
    const contents = await readFile(join(directory, name));
    if (contents.length === 0) throw new Error(`Empty package: ${name}`);
    const checksum = createHash("sha256").update(contents).digest("hex");
    const sidecar = (await readFile(join(directory, `${name}.sha256`), "utf8")).trim();
    if (sidecar !== `${checksum}  ${name}`) {
      throw new Error(`Invalid checksum: ${name}`);
    }
  }
  const platforms = {};
  for (const [platform, name] of [["linux-x86_64", linux], ["darwin-aarch64", macUpdate]]) {
    const signature = (await readFile(join(directory, `${name}.sig`), "utf8")).trim();
    if (!/^[A-Za-z0-9+/]+={0,2}$/.test(signature)
        || !Buffer.from(signature, "base64").toString("utf8").startsWith("untrusted comment:")) {
      throw new Error(`Invalid signature encoding: ${name}`);
    }
    const comment = Buffer.from(signature, "base64").toString("utf8")
      .split("\n").find((line) => line.startsWith("trusted comment: "));
    if (!comment?.split("\t").includes(`version:${version}`)) {
      throw new Error(`Signature version does not match release: ${name}`);
    }
    platforms[platform] = {
      signature,
      url: `https://github.com/${repository}/releases/download/desktop-v${version}/${name}`,
    };
  }
  return { version, notes: `Soft Factory Desktop ${version}`, platforms };
}

async function main() {
  const [command, ...args] = process.argv.slice(2);
  if (command === "create" && args.length === 4) {
    const [version, repository, directory, output] = args;
    const manifest = await buildManifest(version, repository, directory);
    await writeFile(output, `${JSON.stringify(manifest, null, 2)}\n`);
  } else if (command === "newer" && args.length === 2) {
    const [candidate, previous] = await Promise.all(args.map(async (path) =>
      JSON.parse(await readFile(path, "utf8"))));
    process.stdout.write(`${isNewer(candidate.version, previous.version)}\n`);
  } else {
    throw new Error("Usage: desktop-updates.mjs create VERSION OWNER/REPO ASSET_DIR OUTPUT | newer CANDIDATE PREVIOUS");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  await main();
}
