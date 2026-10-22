#!/bin/bash
set -euo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
SKIPERATOR_VERSION=${SKIPERATOR_VERSION:-"v2.19.0"}
# Versions below are the ones skiperator v2.19.0 builds against (see its go.mod)
PROMETHEUS_VERSION=${PROMETHEUS_VERSION:-"v0.94.0"}
GATEWAY_API_VERSION=${GATEWAY_API_VERSION:-"v1.6.2"}
CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-"v1.21.2"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

KUBECTL=("${KUBECTL_BIN}" --context "${KUBECONTEXT}")

RAW="https://raw.githubusercontent.com/kartverket/skiperator/${SKIPERATOR_VERSION}"

SKIPERATOR_RESOURCES=(
  "${RAW}/config/crd/skiperator.kartverket.no_applications.yaml"
  "${RAW}/config/crd/skiperator.kartverket.no_routings.yaml"
  "${RAW}/config/crd/skiperator.kartverket.no_skipjobs.yaml"
  "${RAW}/config/static/priorities.yaml"
  "${RAW}/config/rbac/role.yaml"
  # Gateway API >= 1.5 (needs ListenerSet, standard channel). Checked at
  # startup and skiperator exits if absent - even with routingProvider=Legacy.
  "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml"
  # ServiceMonitor/PodMonitor: the Application reconciler panics without
  # servicemonitors.monitoring.coreos.com
  "https://github.com/prometheus-operator/prometheus-operator/releases/download/${PROMETHEUS_VERSION}/stripped-down-crds.yaml"
  # cert-manager Certificate CRD is watched by the Application controller.
  # CRDs alone are enough to boot; install full cert-manager if you want
  # working ingress TLS.
  "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.crds.yaml"
  "https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_idportenclients.yaml"
  "https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_maskinportenclients.yaml"
)

echo "🤞  Creating namespace: skiperator-system"

output=$("${KUBECTL[@]}" create namespace "skiperator-system" 2>&1) && exit_code=0 || exit_code=$?

if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'skiperator-system' created successfully"
elif echo "$output" | grep -q "already exists"; then
    echo "✅  Namespace 'skiperator-system' already exists, continuing..."
else
    echo -e "❌  Error creating 'skiperator-system' namespace:\n$output"
    exit 1
fi

# Install required skiperator resources
for resource in "${SKIPERATOR_RESOURCES[@]}"; do
  "${KUBECTL[@]}" apply --server-side --force-conflicts -f "$resource"
done

"${KUBECTL[@]}" wait --for=condition=Established --timeout=60s \
  crd/applications.skiperator.kartverket.no \
  crd/routings.skiperator.kartverket.no \
  crd/skipjobs.skiperator.kartverket.no \
  crd/gateways.gateway.networking.k8s.io \
  crd/httproutes.gateway.networking.k8s.io \
  crd/listenersets.gateway.networking.k8s.io

# The SKIPJob CRD ships a v1alpha1 -> v1beta1 conversion webhook served by
# skiperator itself. We run with webhooks disabled, so drop the webhook and
# serve the storage version directly.
"${KUBECTL[@]}" patch crd skipjobs.skiperator.kartverket.no --type=merge \
  -p '{"spec":{"conversion":{"strategy":"None","webhook":null}}}'

# Install skiperator
SKIPERATOR_MANIFESTS="$(cat <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: "skiperator-system"
  name: "skiperator"
automountServiceAccountToken: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: skiperator
roleRef:
  apiGroup: "rbac.authorization.k8s.io"
  kind: "ClusterRole"
  name: "skiperator"
subjects:
  - kind: "ServiceAccount"
    namespace: "skiperator-system"
    name: "skiperator"
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: "namespace-exclusions"
  namespace: skiperator-system
data:
  auth: "true"
  istio-system: "true"
  istio-gateways: "true"
  cert-manager: "true"
  kube-node-lease: "true"
  kube-public: "true"
  kube-system: "true"
  default: "true"
  skiperator-system: "true"
  kube-state-metrics: "true"
  ztoperator-system: "true"
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: "skiperator-config"
  namespace: skiperator-system
data:
  config.json: |
    {
      "topologyKeys": ["kubernetes.io/hostname"],
      "leaderElection": false,
      "leaderElectionNamespace": "skiperator-system",
      "concurrentReconciles": 1,
      "isDeployment": true,
      "logLevel": "debug",
      "registrySecretRefs": [],
      "clusterCIDRExclusionEnabled": false,
      "enableWebhooks": false
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "skiperator"
  namespace: skiperator-system
  labels:
    app: "skiperator"
spec:
  selector:
    matchLabels:
      app: "skiperator"
  replicas: 1
  template:
    metadata:
      labels:
        app: "skiperator"
    spec:
      serviceAccountName: "skiperator"
      automountServiceAccountToken: true
      containers:
        - name: "skiperator"
          image: "ghcr.io/kartverket/skiperator:${SKIPERATOR_VERSION}"
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            runAsUser: 65532
            runAsGroup: 65532
            runAsNonRoot: true
            privileged: false
            seccompProfile:
              type: "RuntimeDefault"
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
            limits:
              memory: 256Mi
          ports:
            - name: metrics
              containerPort: 8181
            - name: "probes"
              containerPort: 8081
          livenessProbe:
            httpGet:
              path: "/healthz"
              port: "probes"
          readinessProbe:
            httpGet:
              path: "/readyz"
              port: "probes"
EOF
)"

"${KUBECTL[@]}" apply -f <(echo "$SKIPERATOR_MANIFESTS")

"${KUBECTL[@]}" rollout status deployment/skiperator -n skiperator-system --timeout=120s