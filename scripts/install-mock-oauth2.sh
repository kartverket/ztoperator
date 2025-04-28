#!/bin/bash

KUBECONTEXT=${KUBECONTEXT:-"kind-kind"}
MOCK_OAUTH2_SERVER_VERSION=${MOCK_OAUTH2_SERVER_VERSION:-"2.1.10"}


DEPLOYMENT="$(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: auth
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
