#!/bin/bash
set -eo pipefail

# Allow overriding via env, but default to your Makefile defaults
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-ztoperator}"
KUBECONTEXT="${KUBECONTEXT:-kind-${KIND_CLUSTER_NAME}}"
KIND_BIN="${KIND_BIN:-./bin/kind}"
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

# tools
command -v "${KIND_BIN}" >/dev/null 2>&1 || {
  echo "❌  kind is not installed (needed to verify the cluster)." >&2
  exit 1
}
command -v "${KUBECTL_BIN}" >/dev/null 2>&1 || {
  echo "❌  kubectl is not installed." >&2
}

# current context
ctx="$("${KUBECTL_BIN}" config current-context 2>/dev/null || true)"
if [[ "${ctx}" != "${KUBECONTEXT}" ]]; then
  echo "❌  Current kubecontext is '${ctx}' (expected '${KUBECONTEXT}')." >&2
  echo "    Switch with: kubectx ${KUBECONTEXT}" >&2
  exit 1
fi

required_namespaces=(skiperator-system cert-manager istio-system auth istio-gateways)

for ns in "${required_namespaces[@]}"; do
  if ! "${KUBECTL_BIN}" get namespace "${ns}" --context "${KUBECONTEXT}" >/dev/null 2>&1; then
    echo "❌  Namespace '${ns}' does not exist in context ${KUBECONTEXT}." >&2
    exit 1
  fi

  # Ensure namespace has pods
  pods=$("${KUBECTL_BIN}" get pods -n "${ns}" --context "${KUBECONTEXT}" -o jsonpath='{.items[*].metadata.name}')

  if [[ -z "${pods}" ]]; then
    echo "❌  No pods found in namespace '${ns}'." >&2
    exit 1
  fi

  # Ensure all pods are Ready
  for pod in ${pods}; do
    [[ -z "${pod}" ]] && continue
    echo "🔎  Checking pod ${ns}/${pod}..." >&2

    "${KUBECTL_BIN}" wait -n "${ns}" --context "${KUBECONTEXT}" \
      --for=condition=Ready "pod/${pod}" --timeout=120s >/dev/null || {
      echo "❌  Pod ${ns}/${pod} is not Ready." >&2
      exit 1
    }
  done
done

echo "✅  Local cluster looks healthy (cluster=${KIND_CLUSTER_NAME}, context=${KUBECONTEXT})."