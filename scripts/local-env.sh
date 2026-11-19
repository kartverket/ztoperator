#!/usr/bin/env bash
# Prints the environment for running Ztoperator locally (terminal or IDE) as KEY=value lines.
# config/manager/base/.env and config/manager/base/oauth-providers.json are the single source of truth,
# shared with `make deploy` (kustomize).
#
#   Terminal:  set -a; eval "$(scripts/local-env.sh --export)"; set +a
#   IDE:       use this script as an executable env file (see .run/Ztoperator.run.xml)
set -euo pipefail

base_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../config/manager/base" && pwd)"

# Newlines are stripped so the multiline JSON fits on one KEY=value line.
providers="$(tr -d '\n' < "${base_dir}/oauth-providers.json")"

if [[ "${1:-}" == "--export" ]]; then
  grep -v '^\s*#' "${base_dir}/.env" | grep -v '^\s*$' | sed 's/^/export /'
  printf "export ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS='%s'\n" "${providers}"
else
  grep -v '^\s*#' "${base_dir}/.env" | grep -v '^\s*$'
  printf 'ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS=%s\n' "${providers}"
fi
