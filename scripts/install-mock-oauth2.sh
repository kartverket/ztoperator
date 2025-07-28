#!/bin/bash

# Parse --config flag
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --config) JSON_CONFIG_PATH="$2"; shift ;;
    *) echo "Unknown parameter passed: $1"; exit 1 ;;
  esac
  shift
done

if [[ -z "$JSON_CONFIG_PATH" ]]; then
  echo "Error: --config flag is required."
  exit 1
fi

KUBECONTEXT=${KUBECONTEXT:-"kind-kind"}
MOCK_OAUTH2_SERVER_VERSION=${MOCK_OAUTH2_SERVER_VERSION:-"2.2.1"}
JSON_CONTENT=$(<"$JSON_CONFIG_PATH")

DEPLOYMENT="$(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: auth
---
# ConfigMap containing JSON config for mock-oauth2-server
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-oauth2-config
  namespace: auth
data:
  JSON_CONFIG: |
$(echo "$JSON_CONTENT" | sed 's/^/      /')
---
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: mock-oauth2
  namespace: auth
spec:
  image: ghcr.io/navikt/mock-oauth2-server:${MOCK_OAUTH2_SERVER_VERSION}
  port: 8080
  replicas: 1
  ingresses:
      - fake.auth
  env:
    - name: "LOG_LEVEL"
      value: "TRACE"
  envFrom:
    - configMap: mock-oauth2-config
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow
  namespace: auth
spec:
  ingress:
  - ports:
    - port: 8080
      protocol: TCP
  podSelector:
    matchLabels:
      app: mock-oauth2
  policyTypes:
  - Ingress
EOF
)"

kubectl apply -f <(echo "$DEPLOYMENT") --context "$KUBECONTEXT"
