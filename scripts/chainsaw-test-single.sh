#!/bin/bash
set -eo pipefail

# Default values
TEST_DIR=""

# Process flags
while [[ $# -gt 0 ]]; do
  case $1 in
    -d|--dir)
      TEST_DIR="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

if [[ -z "$TEST_DIR" ]]; then
  echo "Error: The -d (--dir) flag is required."
  echo "Usage: $0 -d /path/to/directory"
  exit 1
fi

KUBECONTEXT=${KUBECONTEXT:-"kind-ztoperator"}
CHAINSAW_BIN="${CHAINSAW_BIN:-./bin/chainsaw}"

"${CHAINSAW_BIN}" test --kube-context "$KUBECONTEXT" --config test/chainsaw/config.yaml --test-dir "$TEST_DIR" && echo "✅ Test succeeded" || (echo "❌ Test failed" && exit 1)