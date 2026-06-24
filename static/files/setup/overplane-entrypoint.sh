#!/usr/bin/env bash
# shellcheck shell=bash
# Container entrypoint (run under tini).
#
# Workspace contract for agent runs:
#   /workspace-ro : read-only bind mount of the host project root
#   /workspace    : writable scratch the project is rsynced into
#   $@            : command + args
#
# When /workspace-ro is not mounted (e.g. `overplane agent shell` runs a bare
# container with no mounts) the workspace bootstrap is skipped entirely and
# the command execs directly.
#
# Why rsync (not cp -a): handles large node_modules trees and symlink-heavy
# projects more reliably, preserving metadata while keeping output quiet.
set -euo pipefail

if [[ $# -eq 0 ]]; then
  echo "overplane-entrypoint: no command provided" >&2
  exit 67
fi

if [[ ! -d /workspace-ro ]]; then
  exec "$@"
fi

if ! touch /workspace/.overplane-probe 2>/dev/null; then
  echo "overplane-entrypoint: /workspace not writable" >&2
  exit 65
fi
rm -f /workspace/.overplane-probe

if ! rsync -aHAX --numeric-ids --info=stats0,progress0 /workspace-ro/ /workspace/; then
  code=$?
  echo "overplane-entrypoint: rsync failed with exit $code" >&2
  exit 66
fi

cd /workspace
exec "$@"
