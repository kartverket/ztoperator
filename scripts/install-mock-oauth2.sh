#!/bin/bash

MOCK_OAUTH2_SERVER_VERSION=${MOCK_OAUTH2_SERVER_VERSION:-"2.2.1"}

echo -e "🤞 Retrieving content from mock-oauth2-config.json..."
JSON_CONTENT=$(cat "$MOCK_OAUTH2_CONFIG")
if [[ -z "$JSON_CONTENT" ]]; then
  echo "❌ Error: mock-oauth2-config.json is empty or not found at path: $MOCK_OAUTH2_CONFIG"
  exit 1
fi
echo -e "✅  Successfully retrieved content from mock-oauth2-config.json"

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

while true; do
  SUMMARY_STATUS=$(kubectl get application.skiperator.kartverket.no/mock-oauth2 -n auth -o jsonpath='{.status.summary.status}')

  if [[ "$SUMMARY_STATUS" == "Synced" ]]; then
    echo "✅ Application summary status is Synced."
    break
  fi

  sleep 1
  ELAPSED=$((ELAPSED + 1))
  if [[ "$ELAPSED" -ge 30 ]]; then
    echo "❌ Timeout: Application did not reach 'Synced' status in time."
    exit 1
  fi
done

kubectl wait --for=condition=InternalRulesValid=True application.skiperator.kartverket.no/mock-oauth2 -n auth --timeout=30s || (echo -e "❌  Error: accessPolicies for 'mock-oauth2' remain in InvalidConfig state." && exit 1)

kubectl wait pod --for=create --timeout=30s -n auth -l app=mock-oauth2 || (echo -e "❌  Error deploying 'mock-oauth2'." && exit 1)
kubectl wait pod --for=condition=Ready --timeout=30s -n auth -l app=mock-oauth2 || (echo -e "❌  Error deploying 'mock-oauth2'." && exit 1)
