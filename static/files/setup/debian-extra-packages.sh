#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 3 (debian): project-specific extra apt packages from
# overplane.yaml's extra_packages.debian, passed space-joined as $1. Lives in
# its own layer so edits never invalidate the cached toolchain layer; a no-op
# (and therefore cache-stable) when the list is empty.
set -euo pipefail

EXTRA_OS_PACKAGES="${1:-}"

if [[ -z "${EXTRA_OS_PACKAGES// /}" ]]; then
  echo "no extra debian packages requested"
  exit 0
fi

# shellcheck disable=SC2206
PACKAGES=(${EXTRA_OS_PACKAGES})

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get -y install --no-install-recommends "${PACKAGES[@]}"
apt-get clean
rm -rf /var/lib/apt/lists/*
