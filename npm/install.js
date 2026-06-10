#!/usr/bin/env node
// postinstall: download the bare overplane executable for this platform from
// the GitHub release matching package.json's version, verify its sha256
// against the release's checksums.txt, and place it in ./dist/.
//
// Self-contained: Node stdlib only, no archive extraction, no system tools.
"use strict";

const fs = require("fs");
const path = require("path");
const http = require("http");
const https = require("https");
const crypto = require("crypto");

const pkg = require("./package.json");
const DEFAULT_BASE = `https://github.com/overplane/overplane/releases/download/v${pkg.version}`;
// Test hook: point at any static server hosting the artifacts.
const BASE = process.env.OVERPLANE_DOWNLOAD_BASE || DEFAULT_BASE;

const OS_MAP = { linux: "linux", darwin: "darwin", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

function fail(msg) {
  console.error(`overplane install error: ${msg}`);
  process.exit(1);
}

function get(url, redirects = 5) {
  const proto = url.startsWith("http://") ? http : https;
  return new Promise((resolve, reject) => {
    proto
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

  const ext = osName === "windows" ? ".exe" : "";
  const asset = `overplane_${pkg.version}_${osName}_${archName}${ext}`;

  const checksums = (await get(`${BASE}/checksums.txt`)).toString("utf8");
  const entry = checksums
    .split("\n")
    .map((l) => l.trim().split(/\s+/))
    .find((f) => f[1] === asset);
  if (!entry) fail(`no entry for ${asset} in checksums.txt`);

  const data = await get(`${BASE}/${asset}`);
  const digest = crypto.createHash("sha256").update(data).digest("hex");
  if (digest !== entry[0]) {
    fail(`sha256 mismatch for ${asset}: expected ${entry[0]}, got ${digest}`);
  }

  const distDir = path.join(__dirname, "dist");
  fs.mkdirSync(distDir, { recursive: true });
  const binPath = path.join(distDir, `overplane${ext}`);
  fs.writeFileSync(binPath, data, { mode: 0o755 });
  console.log(`overplane ${pkg.version} installed (${osName}/${archName})`);
}

main().catch((err) => fail(err.message));
