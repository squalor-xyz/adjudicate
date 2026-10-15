# SPDX-License-Identifier: GPL-3.0-or-later
#
# Source this from bash or zsh, by path: . scripts/env.sh (from the repo root).
#
# Keeps Go's caches and temp files inside this checkout under .cache/ (C33).
# The repo root is derived from this script's own path, so each worktree uses
# its own .cache/. Nothing is exported or created unless that root holds go.mod
# and scripts/env.sh; otherwise the script prints an error and returns 1.
# Dash and other shells that are not bash or zsh cannot report the sourced
# path, so they fail that way.
# Go is told to use only the local toolchain and no module proxy.
# Never use `go env -w`: it writes outside the repo.

if [ -n "${BASH_VERSION-}" ]; then
  _adj_self=${BASH_SOURCE[0]}
elif [ -n "${ZSH_VERSION-}" ]; then
  eval '_adj_self=${(%):-%x}'
else
  _adj_self=
fi

if [ -z "$_adj_self" ]; then
  echo "env.sh: cannot tell where this script is; source it from bash or zsh" >&2
  unset _adj_self
  return 1
fi

_adj_root=$(CDPATH= cd -- "$(dirname -- "$_adj_self")/.." 2>/dev/null && pwd -P)

if [ -z "$_adj_root" ] || [ ! -f "$_adj_root/go.mod" ] || [ ! -f "$_adj_root/scripts/env.sh" ]; then
  echo "env.sh: repo root not found from '$_adj_self' (no go.mod there); nothing was set" >&2
  unset _adj_self _adj_root
  return 1
fi

export GOCACHE="$_adj_root/.cache/go-build"
export GOMODCACHE="$_adj_root/.cache/gomod"
export GOPATH="$_adj_root/.cache/gopath"
export GOTMPDIR="$_adj_root/.cache/tmp"
export TMPDIR="$_adj_root/.cache/tmp"
export GOTOOLCHAIN=local
export GOPROXY=off
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

if ! mkdir -p "$GOTMPDIR"; then
  echo "env.sh: cannot create $GOTMPDIR" >&2
  unset _adj_self _adj_root
  return 1
fi

unset _adj_self _adj_root
