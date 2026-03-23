#!/bin/bash
set -eo pipefail


LOCAL_WEBHOOK_CERTS_DIR="${LOCAL_WEBHOOK_CERTS_DIR:-webhook-certs}"

OUT="$(pwd)/${LOCAL_WEBHOOK_CERTS_DIR}"
mkdir -p "$OUT"

NAMESPACE="${NAMESPACE:-ztoperator-system}"
CERT_NAME="${CERT_NAME:-webhook-cert}"
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "ERROR: missing required command: $1" >&2
    exit 1
  }
}

require base64

# Find the Secret created by cert-manager for this Certificate
SECRET_NAME="$("${KUBECTL_BIN}" -n "$NAMESPACE" get certificate "$CERT_NAME" -o jsonpath='{.spec.secretName}')"
if [[ -z "${SECRET_NAME}" ]]; then
  echo "ERROR: could not determine .spec.secretName for Certificate ${CERT_NAME} in namespace ${NAMESPACE}" >&2
  exit 1
fi

echo "Using Certificate: ${NAMESPACE}/${CERT_NAME}"
echo "Using Secret:      ${NAMESPACE}/${SECRET_NAME}"

# tls.crt and tls.key are always present in a cert-manager TLS secret
${KUBECTL_BIN} -n "$NAMESPACE" get secret "$SECRET_NAME" -o jsonpath='{.data.tls\.crt}' | base64 --decode > "$OUT/tls.crt"
${KUBECTL_BIN} -n "$NAMESPACE" get secret "$SECRET_NAME" -o jsonpath='{.data.tls\.key}' | base64 --decode > "$OUT/tls.key"

# cert-manager often includes the issuer CA at .data["ca.crt"].
# If it does not exist (selfSigned/CA-less setups), leave ca.crt/caBundle absent.
CA_B64="$("${KUBECTL_BIN}" -n "$NAMESPACE" get secret "$SECRET_NAME" -o jsonpath='{.data.ca\.crt}' 2>/dev/null || true)"
if [[ -n "$CA_B64" ]]; then
  printf '%s' "$CA_B64" | base64 --decode > "$OUT/ca.crt"
  base64 < "$OUT/ca.crt" | tr -d '\n' > "$OUT/caBundle"
  echo "Wrote: $OUT/ca.crt, $OUT/caBundle, $OUT/tls.crt, $OUT/tls.key"
else
  echo "NOTE: Secret does not contain ca.crt; writing only tls.crt and tls.key" >&2
  echo "Wrote: $OUT/tls.crt, $OUT/tls.key"
fi

# Helpful permissions for local use
chmod 600 "$OUT/tls.key" || true
chmod 644 "$OUT/tls.crt" || true
[[ -f "$OUT/ca.crt" ]] && chmod 644 "$OUT/ca.crt" || true
