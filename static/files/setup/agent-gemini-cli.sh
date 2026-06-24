#!/usr/bin/env bash
# shellcheck shell=bash
# Agent layer: Google Gemini CLI, installed globally from the official
# @google/gemini-cli npm package. $1 is an npm dist-tag or exact version
# (stable channel: latest; preview/nightly dist-tags also work). Ends with a
# --version smoke check so a broken install fails the image build.
set -euo pipefail

GEMINI_CLI_VERSION="${1:?usage: agent-gemini-cli.sh <npm dist-tag or version>}"

npm install -g "@google/gemini-cli@${GEMINI_CLI_VERSION}"
gemini --version
