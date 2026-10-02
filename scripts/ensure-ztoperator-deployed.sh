#!/usr/bin/env bash
set -euo pipefail

# Allow overriding via env, but default to your Makefile defaults
KUBECONTEXT="${KUBECONTEXT:-kind-ztoperator}"
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

NAMESPACE="ztoperator-system"
DEPLOYMENT="ztoperator"

VALIDATING_CFG="ztoperator-validating-webhook-configuration"

EXPECTED_SVC="webhook-service"
EXPECTED_AUTHPOLICY_WEBHOOK_NAME="vauthpolicy-v1alpha1.kb.io"
EXPECTED_WEBHOOK_NAME="vpod-v1.kb.io"
EXPECTED_SELECTOR_KEY="skip.kartverket.no/skip-managed"
EXPECTED_SELECTOR_VALUE="true"

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

get_webhook_field() {
  local KIND="$1"
  local NAME="$2"
  local WEBHOOK_NAME="$3"
  local FIELD="$4"

  ${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o "jsonpath={.webhooks[?(@.name==\"${WEBHOOK_NAME}\")].${FIELD}}"
}

check_webhook() {
  local KIND="$1"   # mutatingwebhookconfiguration | validatingwebhookconfiguration
  local NAME="$2"
  local LABEL="$3"  # Mutating | Validating
  local WEBHOOK_NAME="$4"
  local CHECK_NAMESPACE_SELECTOR="$5"

  echo "🔎 Checking ${LABEL} webhook ${WEBHOOK_NAME} in ${NAME}..."

  ${KUBECTL_BIN} get "$KIND" "$NAME" >/dev/null

  FOUND_WEBHOOK_NAME=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'name')

  if [[ -z "$FOUND_WEBHOOK_NAME" ]]; then
    echo "❌ ${NAME} does not contain webhook ${WEBHOOK_NAME}"
    exit 1
  fi

  # clientConfig.service
  SVC_NAME=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'clientConfig.service.name')

  SVC_NS=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'clientConfig.service.namespace')

  if [[ "$SVC_NAME" != "$EXPECTED_SVC" || "$SVC_NS" != "$NAMESPACE" ]]; then
    echo "❌ ${NAME} webhook ${WEBHOOK_NAME}: clientConfig.service must be ${NAMESPACE}/${EXPECTED_SVC}"
    echo "   got ${SVC_NS}/${SVC_NAME}"
    exit 1
  fi

  # CA bundle is injected asynchronously by cert-manager.
  CA_BUNDLE=""
  for _ in {1..30}; do
    CA_BUNDLE=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'clientConfig.caBundle')
    if [[ -n "$CA_BUNDLE" ]]; then
      break
    fi
    sleep 2
  done

  if [[ -z "$CA_BUNDLE" ]]; then
    echo "❌ ${NAME} webhook ${WEBHOOK_NAME}: clientConfig.caBundle is empty"
    exit 1
  fi

  if [[ "$CHECK_NAMESPACE_SELECTOR" == "true" ]]; then
    # namespaceSelector is specific to the Pod webhook.
    SELECTOR_KEY=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'namespaceSelector.matchExpressions[0].key')

    SELECTOR_VALUE=$(get_webhook_field "$KIND" "$NAME" "$WEBHOOK_NAME" 'namespaceSelector.matchExpressions[0].values[0]')

    if [[ "$SELECTOR_KEY" != "$EXPECTED_SELECTOR_KEY" || "$SELECTOR_VALUE" != "$EXPECTED_SELECTOR_VALUE" ]]; then
      echo "❌ ${NAME} webhook ${WEBHOOK_NAME}: namespaceSelector must include ${EXPECTED_SELECTOR_KEY}=In(${EXPECTED_SELECTOR_VALUE})"
      echo "   Found: ${SELECTOR_KEY}=In(${SELECTOR_VALUE:-<empty>})"
      exit 1
    fi
  fi

  echo "✅ ${LABEL} webhook ${WEBHOOK_NAME} is valid."
}

# ---- execution ----

check_deployment

check_webhook \
  "validatingwebhookconfiguration" \
  "$VALIDATING_CFG" \
  "Validating" \
  "$EXPECTED_AUTHPOLICY_WEBHOOK_NAME" \
  "false"

check_webhook \
  "validatingwebhookconfiguration" \
  "$VALIDATING_CFG" \
  "Validating" \
  "$EXPECTED_WEBHOOK_NAME" \
  "true"

echo "🎉 Ztoperator is deployed and ready to reconcile."
