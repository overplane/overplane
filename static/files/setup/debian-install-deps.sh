#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 2 (debian): the broad developer toolchain. Go comes from the official
# go.dev tarball, Rust via rustup (stable, minimal profile), Node.js from
# NodeSource, and uv via pip; everything else is apt. Versions are overridable
# through the GO_VERSION / NODE_MAJOR / RUST_MIN_VERSION build args.
# Project-specific extras belong in debian-extra-packages.sh so they never
# invalidate this (expensive, cache-friendly) layer.
set -euo pipefail

GO_VERSION="${GO_VERSION:-1.26.4}"
RUST_MIN_VERSION="${RUST_MIN_VERSION:-1.96.0}"
NODE_MAJOR="${NODE_MAJOR:-24}"

BASE_PACKAGES=(
  curl
  jq
  ca-certificates
  htop
  tini
  make
  build-essential
  pkg-config
  libssl-dev
  git
  ripgrep
  rsync
  z3
  python3
  python3-pip
  python-is-python3
  sudo
)

version_ge() {
  test "$(printf '%s\n' "$1" "$2" | sort -V | head -n1)" = "$1"
}

install_go() {
  local arch
  case "$(dpkg --print-architecture)" in
    amd64) arch=amd64 ;;
    arm64) arch=arm64 ;;
    *)
      echo "unsupported architecture for Go install: $(dpkg --print-architecture)" >&2
      exit 1
      ;;
  esac
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" | tar -C /usr/local -xz
  cat >/etc/profile.d/go-path.sh <<'EOF'
export PATH=/usr/local/go/bin:$PATH
EOF
  chmod 0644 /etc/profile.d/go-path.sh
  /usr/local/go/bin/go version
}

install_rust() {
  export RUSTUP_HOME=/usr/local/rustup
  export CARGO_HOME=/usr/local/cargo
  curl -fsSL https://sh.rustup.rs | sh -s -- -y --default-toolchain stable --profile minimal --no-modify-path
  for tool in cargo rustc rustup; do
    ln -sf "/usr/local/cargo/bin/${tool}" "/usr/local/bin/${tool}"
  done
  cat >/etc/profile.d/rust-path.sh <<'EOF'
export RUSTUP_HOME=/usr/local/rustup
export CARGO_HOME=/usr/local/cargo
export PATH=/usr/local/cargo/bin:$PATH
EOF
  chmod 0644 /etc/profile.d/rust-path.sh
  local installed
  installed="$(rustc --version | awk '{print $2}')"
  if ! version_ge "${RUST_MIN_VERSION}" "${installed}"; then
    echo "rustc ${installed} is older than required ${RUST_MIN_VERSION}" >&2
    exit 1
  fi
}

install_node() {
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get update
  apt-get -y install --no-install-recommends nodejs
  local node_major
  node_major="$(node --version | sed 's/^v//' | cut -d. -f1)"
  if [[ "${node_major}" -lt 22 ]]; then
    echo "node v${node_major} is not current LTS (22+)" >&2
    exit 1
  fi
}

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get -y install --no-install-recommends "${BASE_PACKAGES[@]}"

install_go
install_rust
install_node

# Install the Python uv package/tool manager.
python3 -m pip install --break-system-packages --upgrade uv

apt-get clean
rm -rf /var/lib/apt/lists/*
