#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
caller_dir="$(pwd)"
binary="${script_dir}/dist/overplane"

needs_build=0
if [[ ! -x "${binary}" ]]; then
  needs_build=1
elif [[ "${OVERPLANE_FORCE_BUILD:-0}" == "1" ]]; then
  needs_build=1
elif find "${script_dir}" "${script_dir}/../config/data" "${script_dir}/../config/schema" \
  -path "${script_dir}/dist" -prune -o \
  \( -name '*.go' -o -name go.mod -o -name go.sum -o -name VERSION -o -path "${script_dir}/static/*" -o -name '*.yaml' -o -name '*.json' \) \
  -newer "${binary}" -print -quit | grep -q .; then
  needs_build=1
fi

if [[ "${needs_build}" == "1" ]]; then
  mkdir -p "${script_dir}/dist"
  version="$(tr -d '\n' < "${script_dir}/VERSION")"
  commit="$(git -C "${script_dir}" rev-parse --short=12 HEAD 2>/dev/null || echo dev)"
  if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
    date="$(date -u -d "@${SOURCE_DATE_EPOCH}" +%Y-%m-%dT%H:%M:%SZ)"
  else
    date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  fi
  ldflags="-s -w -buildid= -X github.com/overplane/overplane/internal/platform/version.Version=${version} -X github.com/overplane/overplane/internal/platform/version.Commit=${commit} -X github.com/overplane/overplane/internal/platform/version.Date=${date}"
  (cd "${script_dir}" && CGO_ENABLED=0 go build -trimpath -ldflags "${ldflags}" -buildvcs=false -o dist/overplane ./cmd/overplane)
fi

cd "${caller_dir}"
exec "${binary}" "$@"
