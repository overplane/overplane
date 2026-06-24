#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 2 (alpine): best-effort toolchain parity with the debian recipe using
# apk. Known gaps (documented in the recipe registry): Go, Node.js, and Rust
# install from the Alpine package repositories (musl builds), so their
# versions track the Alpine release rather than the GO_VERSION / NODE_MAJOR /
# RUST_MIN_VERSION arguments honored by the debian recipe. uv installs via
# pip (musl wheels). The shadow package provides groupadd/useradd/usermod for
# the shared linux-setup-user fragment.
set -euo pipefail

BASE_PACKAGES=(
  bash
  curl
  jq
  ca-certificates
  htop
  tini
  make
  build-base
  pkgconf
  openssl-dev
  git
  ripgrep
  rsync
  z3
  python3
  py3-pip
  sudo
  shadow
  go
  nodejs
  npm
  rust
  cargo
)

apk add --no-cache "${BASE_PACKAGES[@]}"

# Parity with debian's python-is-python3.
if [[ ! -e /usr/bin/python ]]; then
  ln -s /usr/bin/python3 /usr/bin/python
fi

# Install the Python uv package/tool manager.
python3 -m pip install --break-system-packages --upgrade uv

go version
rustc --version
node --version
