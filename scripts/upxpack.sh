#!/usr/bin/env bash
# Compress built CLI binaries in place when UPX supports the target.
# Policy matches .goreleaser.yaml: linux/* and windows/amd64 only (no darwin).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  echo "usage: upxpack.sh [--goos OS --goarch ARCH] BINARY..." >&2
  exit 2
}

pack_supported() {
  case "$1/$2" in
    linux/amd64 | linux/arm64 | windows/amd64) return 0 ;;
    *) return 1 ;;
  esac
}

pack_one() {
  local goos="$1" goarch="$2" bin="$3"
  if [[ "${OVERPLANE_SKIP_UPX:-0}" == "1" ]]; then
    return 0
  fi
  if [[ ! -f "${bin}" ]]; then
    echo "upxpack: ${bin}: not found" >&2
    return 1
  fi
  if ! pack_supported "${goos}" "${goarch}"; then
    return 0
  fi
  if ! command -v upx >/dev/null 2>&1; then
    echo "upxpack: skipping ${bin} (upx not on PATH; install upx-ucl or set OVERPLANE_SKIP_UPX=1)" >&2
    return 0
  fi
  upx --best --lzma "${bin}" >&2
}

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
bins=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --goos)
      [[ $# -ge 2 ]] || usage
      goos="$2"
      shift 2
      ;;
    --goarch)
      [[ $# -ge 2 ]] || usage
      goarch="$2"
      shift 2
      ;;
    -h | --help)
      usage
      ;;
    --)
      shift
      bins+=("$@")
      break
      ;;
    -*)
      echo "upxpack: unknown option: $1" >&2
      usage
      ;;
    *)
      bins+=("$1")
      shift
      ;;
  esac
done

if [[ ${#bins[@]} -eq 0 ]]; then
  usage
fi

for bin in "${bins[@]}"; do
  pack_one "${goos}" "${goarch}" "${bin}"
done
