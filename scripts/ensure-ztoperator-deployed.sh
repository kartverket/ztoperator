#!/usr/bin/env bash
set -euo pipefail

# Allow overriding via env, but default to your Makefile defaults
KUBECONTEXT="${KUBECONTEXT:-kind-ztoperator}}"
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

NAMESPACE="ztoperator-system"
DEPLOYMENT="ztoperator"

check_deployment() {
  echo "🔎 Checking Deployment ${NAMESPACE}/${DEPLOYMENT}..."

  ${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" >/dev/null

  READY=$(${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.status.readyReplicas}')

  DESIRED=$(${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.spec.replicas}')

  if [[ "${READY:-0}" != "$DESIRED" ]]; then
    echo "❌ Deployment not ready (ready=${READY:-0}, desired=${DESIRED})"
    exit 1
  fi

  echo "✅ Deployment is healthy (replicas=${DESIRED})."
}

# ---- execution ----

check_deployment

echo "🎉 Ztoperator is deployed and ready to reconcile."