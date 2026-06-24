#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 1 (alpine): UTC timezone, full apk upgrade, TLS roots. Slowest-changing
# layer. Note: this fragment runs under bash, which alpine-base images lack,
# so the Dockerfile bootstraps bash via /bin/sh before any fragment runs.
set -euo pipefail

apk update
apk upgrade
apk add --no-cache ca-certificates tzdata

ln -snf /usr/share/zoneinfo/Etc/UTC /etc/localtime
echo "Etc/UTC" >/etc/timezone
