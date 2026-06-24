#!/usr/bin/env bash
# shellcheck shell=bash
# Final provisioning layer (shared by all setup recipes): create a group and
# user mirroring the host identity, grant passwordless sudo, and hook the
# Overplane shell profile into the user's bash startup files.
#
# Arguments: $1=uid $2=gid $3=username $4=home
# Requires groupadd/useradd/usermod (debian: passwd; alpine: shadow package).
set -euo pipefail

OVERPLANE_UID="${1}"
OVERPLANE_GID="${2}"
OVERPLANE_USER="${3}"
OVERPLANE_HOME="${4}"

if ! getent group "${OVERPLANE_GID}" >/dev/null; then
  groupadd --gid "${OVERPLANE_GID}" "${OVERPLANE_USER}"
fi

if ! id -u "${OVERPLANE_USER}" >/dev/null 2>&1; then
  useradd \
    --uid "${OVERPLANE_UID}" \
    --gid "${OVERPLANE_GID}" \
    --create-home \
    --home-dir "${OVERPLANE_HOME}" \
    --shell /bin/bash \
    "${OVERPLANE_USER}"
fi

# Passwordless sudo via sudoers.d; only join the sudo group where it exists
# (debian has one, alpine does not).
if getent group sudo >/dev/null; then
  usermod -aG sudo "${OVERPLANE_USER}"
fi
echo "${OVERPLANE_USER} ALL=(ALL) NOPASSWD:ALL" >"/etc/sudoers.d/${OVERPLANE_USER}"
chmod 0440 "/etc/sudoers.d/${OVERPLANE_USER}"

mkdir -p "${OVERPLANE_HOME}"
chown -R "${OVERPLANE_UID}:${OVERPLANE_GID}" "${OVERPLANE_HOME}"

BASHRC="${OVERPLANE_HOME}/.bashrc"
BASHPROFILE="${OVERPLANE_HOME}/.bash_profile"
SOURCE_LINE='[ -f /etc/overplane/bash_profile_extra.sh ] && source /etc/overplane/bash_profile_extra.sh'

touch "${BASHRC}" "${BASHPROFILE}"
if ! grep -Fq "${SOURCE_LINE}" "${BASHRC}"; then
  echo "${SOURCE_LINE}" >>"${BASHRC}"
fi
if ! grep -Fq "${SOURCE_LINE}" "${BASHPROFILE}"; then
  echo "${SOURCE_LINE}" >>"${BASHPROFILE}"
fi

chown "${OVERPLANE_UID}:${OVERPLANE_GID}" "${BASHRC}" "${BASHPROFILE}"
