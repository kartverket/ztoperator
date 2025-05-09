#!/bin/bash


KUBECONTEXT=${KUBECONTEXT:-"kind-kind"}
SKIPERATOR_VERSION=${SKIPERATOR_VERSION:-"latest"}
CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-"v1.17.1"}
PROMETHEUS_VERSION=${PROMETHEUS_VERSION:-"v0.81.0"}

SKIPERATOR_RESOURCES=(
  https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/crd/skiperator.kartverket.no_applications.yaml
  https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/crd/skiperator.kartverket.no_routings.yaml
  https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/crd/skiperator.kartverket.no_skipjobs.yaml
  https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/static/priorities.yaml
  https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/rbac/role.yaml
  https://github.com/cert-manager/cert-manager/releases/download/"${CERT_MANAGER_VERSION}"/cert-manager.yaml
  https://github.com/prometheus-operator/prometheus-operator/releases/download/"${PROMETHEUS_VERSION}"/stripped-down-crds.yaml
  https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_idportenclients.yaml
  https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_maskinportenclients.yaml
)

echo "Installing skiperator in cluster $KUBECONTEXT"
# Install required skiperator resources
for resource in "${SKIPERATOR_RESOURCES[@]}"; do
  kubectl apply --context "$KUBECONTEXT" -f "$resource"
done

echo "Waiting for cert-manager to be ready..."
sleep 20

# Configure cert-manager clusterissuer
kubectl apply --context "$KUBECONTEXT" -f <(cat <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: cluster-issuer
spec:
  selfSigned: {}
EOF
)

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
          args: ["-l", "-d"]
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

kubectl apply -f <(echo "$SKIPERATOR_MANIFESTS") --context "$KUBECONTEXT"
