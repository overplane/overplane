#!/usr/bin/env node
// Thin launcher for the native overplane binary downloaded by install.js.
"use strict";

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const binName = process.platform === "win32" ? "overplane.exe" : "overplane";
const bin = path.join(__dirname, "..", "dist", binName);

if (!fs.existsSync(bin)) {
  console.error("overplane: native binary not found.");
  console.error("It is downloaded by the package's postinstall script.");
  console.error("Run: npm rebuild overplane (or reinstall without --ignore-scripts)");
  process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`overplane: failed to launch binary: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
