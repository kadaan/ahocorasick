#!/usr/bin/env -S dumb-init bash

export PACKAGE_VERSION="${PACKAGE_VERSION:-1.0}"
export PCRE2_VERSION="${PCRE2_VERSION:-10.47}"

function fatal() {
  echo "ERROR: $*" >&2
  exit 1
}

function run() {
  local root_dir="$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"
  if [[ "${DEVBOX_SHELL_ENABLED:-0}" != "1" || "$DEVBOX_PROJECT_ROOT" != "$root_dir" ]]; then
    devbox run --config "${root_dir}" build "$@"
    return $?
  fi

  just build || fatal "failed to build: $?"
}

run "$@"
