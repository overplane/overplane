#!/usr/bin/env node
// postinstall: download the overplane binary for this platform from the
// GitHub release matching package.json's version, verify its sha256 against
// the release's checksums.txt, and extract it into ./dist/.
"use strict";

const fs = require("fs");
const path = require("path");
const https = require("https");
const crypto = require("crypto");
const { execFileSync } = require("child_process");

const pkg = require("./package.json");
const REPO = "overplane/overplane";

const OS_MAP = { linux: "linux", darwin: "darwin", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

function fail(msg) {
  console.error(`overplane install error: ${msg}`);
  process.exit(1);
}

function get(url, redirects = 5) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "user-agent": "overplane-npm-installer" } }, (res) => {
        if ([301, 302, 303, 307, 308].includes(res.statusCode)) {
          res.resume();
          if (!redirects) return reject(new Error(`too many redirects: ${url}`));
          return resolve(get(res.headers.location, redirects - 1));
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`GET ${url}: HTTP ${res.statusCode}`));
        }
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

async function main() {
  if (pkg.version === "0.0.0-dev") {
    console.log("overplane: development placeholder version; skipping binary download.");
    return;
  }

  const osName = OS_MAP[process.platform];
  const archName = ARCH_MAP[process.arch];
  if (!osName || !archName) {
    fail(`unsupported platform: ${process.platform}/${process.arch}`);
  }

  const ext = osName === "windows" ? "zip" : "tar.gz";
  const archive = `overplane_${pkg.version}_${osName}_${archName}.${ext}`;
  const base = `https://github.com/${REPO}/releases/download/v${pkg.version}`;

  const checksums = (await get(`${base}/checksums.txt`)).toString("utf8");
  const entry = checksums
    .split("\n")
    .map((l) => l.trim().split(/\s+/))
    .find((f) => f[1] === archive);
  if (!entry) fail(`no entry for ${archive} in checksums.txt`);

  const data = await get(`${base}/${archive}`);
  const digest = crypto.createHash("sha256").update(data).digest("hex");
  if (digest !== entry[0]) {
    fail(`sha256 mismatch for ${archive}: expected ${entry[0]}, got ${digest}`);
  }

  const distDir = path.join(__dirname, "dist");
  fs.mkdirSync(distDir, { recursive: true });
  const archivePath = path.join(distDir, archive);
  fs.writeFileSync(archivePath, data);

  // bsdtar (shipped with Windows 10+) extracts zip as well as tar.gz
  const binName = osName === "windows" ? "overplane.exe" : "overplane";
  execFileSync("tar", ["-xf", archivePath, "-C", distDir, binName]);
  fs.rmSync(archivePath);
  if (osName !== "windows") {
    fs.chmodSync(path.join(distDir, binName), 0o755);
  }
  console.log(`overplane ${pkg.version} installed (${osName}/${archName})`);
}

main().catch((err) => fail(err.message));
