#!/usr/bin/env bash
# shellcheck shell=bash
# Agent layer: OpenAI Codex CLI, installed globally from the official
# @openai/codex npm package (the native Rust CLI). $1 is an npm dist-tag or
# exact version. Ends with a --version smoke check so a broken install fails
# the image build.
set -euo pipefail

CODEX_VERSION="${1:?usage: agent-codex.sh <npm dist-tag or version>}"

npm install -g "@openai/codex@${CODEX_VERSION}"
codex --version
