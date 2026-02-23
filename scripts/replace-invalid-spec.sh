#!/bin/bash

set -u

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <resource-file> <expected-error>" >&2
  exit 1
fi

resource_file="$1"
expected_error="$2"

if output=$(kubectl replace -f "${resource_file}" -n chainsaw-manifest-validation 2>&1); then
  echo "expected kubectl replace ${resource_file} to fail" >&2
  exit 1
fi

if ! grep -q -- "${expected_error}" <<<"${output}"; then
  echo "kubectl patch did not fail with expected validation error" >&2
  echo "${output}" >&2
  exit 1
fi

