#!/bin/bash

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --name)
      NAME="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    *)
      echo "Unknown parameter: $1"
      echo "Usage: $0 --name <authpolicy-name> --namespace <namespace>"
      exit 1
      ;;
  esac
done

if [[ -z "$NAME" || -z "$NAMESPACE" ]]; then
  echo "Error: --name and --namespace are required."
  echo "Usage: $0 --name <authpolicy-name> --namespace <namespace>"
  exit 1
fi

KUBECONTEXT="kind-ztoperator"

echo "# RequestAuthentication"
kubectl get requestauthentication "$NAME" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml | \
yq eval '
  del(
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.labels,
    .metadata.namespace,
    .metadata.ownerReferences,
    .metadata.resourceVersion,
    .metadata.uid
  )
' -

echo -e "---\n# AuthorizationPolicy (deny-auth-rules)"
kubectl get authorizationpolicy "${NAME}-deny-auth-rules" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml | \
yq eval '
  del(
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.labels,
    .metadata.namespace,
    .metadata.ownerReferences,
    .metadata.resourceVersion,
    .metadata.uid
  )
' -

echo -e "---\n# AuthorizationPolicy (ignore-auth)"
kubectl get authorizationpolicy "${NAME}-ignore-auth" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml | \
yq eval '
  del(
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.labels,
    .metadata.namespace,
    .metadata.ownerReferences,
    .metadata.resourceVersion,
    .metadata.uid
  )
' -

echo -e "---\n# AuthorizationPolicy (require-auth)"
kubectl get authorizationpolicy "${NAME}-require-auth" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml | \
yq eval '
  del(
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.labels,
    .metadata.namespace,
    .metadata.ownerReferences,
    .metadata.resourceVersion,
    .metadata.uid
  )
' -