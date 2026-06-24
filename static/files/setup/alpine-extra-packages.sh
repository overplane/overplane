#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 3 (alpine): project-specific extra apk packages from
# overplane.yaml's extra_packages.alpine, passed space-joined as $1. Lives in
# its own layer so edits never invalidate the cached toolchain layer; a no-op
# (and therefore cache-stable) when the list is empty.
set -euo pipefail

EXTRA_OS_PACKAGES="${1:-}"

if [[ -z "${EXTRA_OS_PACKAGES// /}" ]]; then
  echo "no extra alpine packages requested"
  exit 0
fi

# shellcheck disable=SC2206
PACKAGES=(${EXTRA_OS_PACKAGES})

apk add --no-cache "${PACKAGES[@]}"
