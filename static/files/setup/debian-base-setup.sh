#!/usr/bin/env bash
# shellcheck shell=bash
# Layer 1 (debian): UTC timezone, full apt upgrade, TLS roots. Slowest-changing
# layer; apt cache is cleaned in the same layer to keep it small.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

ln -snf /usr/share/zoneinfo/Etc/UTC /etc/localtime
echo "Etc/UTC" >/etc/timezone

apt-get update
apt-get -y upgrade
apt-get -y install --no-install-recommends ca-certificates tzdata

apt-get clean
rm -rf /var/lib/apt/lists/*
