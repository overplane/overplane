#!/usr/bin/env bash
# shellcheck shell=bash
# Agent layer: Anthropic Claude Code, installed globally from the official
# @anthropic-ai/claude-code npm package, which delivers the same native
# binary as Anthropic's install script via per-platform optional dependencies
# (including musl builds for alpine). $1 is an npm dist-tag or exact version.
# Ends with a --version smoke check so a broken install fails the image build.
set -euo pipefail

CLAUDE_CODE_VERSION="${1:?usage: agent-claude-code.sh <npm dist-tag or version>}"

npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"
claude --version
