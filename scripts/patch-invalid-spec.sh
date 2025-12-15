#!/bin/bash

set -u

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <patch-file> <expected-error>" >&2
  exit 1
fi

patch_file="$1"
expected_error="$2"

if output=$(kubectl patch authpolicy auth-policy -n chainsaw-manifest-validation --type=merge \
  --patch-file "${patch_file}" 2>&1); then
  echo "expected kubectl patch ${patch_file} to fail" >&2
  exit 1
fi

if ! grep -q -- "${expected_error}" <<<"${output}"; then
  echo "kubectl patch did not fail with expected validation error" >&2
  echo "${output}" >&2
  exit 1
fi
