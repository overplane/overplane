#!/usr/bin/env bash
# shellcheck shell=bash
# Sourced by ~/.bashrc and ~/.bash_profile inside the Overplane agent
# container (no set -e: this runs in interactive shells).

export OVERPLANE_SHELL=1
export OVERPLANE_CONTAINER=1
export LS_COLORS="${LS_COLORS:-}"

alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CFlha'

# Overplane prompt: autumn-palette xterm-256 colors (amber 214, rust 130,
# rose 166) rendering "⌬ overplane:<project>" before the path. Neither the host
# user nor the hostname is shown since the shell is ephemeral.
if [[ $- == *i* ]] && [[ -z ${OVERPLANE_PS1_SET:-} ]]; then
  OVERPLANE_PS1_SET=1
  _op_amber='\[\033[38;5;214m\]'  # brand amber
  _op_rust='\[\033[38;5;130m\]'   # brand rust
  _op_rose='\[\033[38;5;166m\]'   # brand rose
  _op_sand='\[\033[38;5;180m\]'   # sand path
  _op_dim='\[\033[38;5;244m\]'    # dim separators
  _op_reset='\[\033[0m\]'
  _op_bold='\[\033[1m\]'

  PS1="${_op_amber}⌬ ${_op_bold}overplane${_op_reset}${_op_dim}:${_op_rose}${OVERPLANE_PROJECT:-project}${_op_reset} ${_op_rust}▸${_op_reset} ${_op_sand}\w${_op_reset}${_op_bold}\$${_op_reset} "
  unset -v _op_amber _op_rust _op_rose _op_sand _op_dim _op_reset _op_bold
fi
