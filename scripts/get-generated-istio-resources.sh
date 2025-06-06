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

OUTPUT=$(kubectl get requestauthentication "$NAME" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml)
if [[ -n "$OUTPUT" ]]; then
  echo "# RequestAuthentication"
  echo "$OUTPUT" | yq eval '
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
fi

OUTPUT=$(kubectl get authorizationpolicy "${NAME}-deny-auth-rules" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml)
if [[ -n "$OUTPUT" ]]; then
  echo -e "---\n# AuthorizationPolicy (deny-auth-rules)"
  echo "$OUTPUT" | yq eval '
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
fi

OUTPUT=$(kubectl get authorizationpolicy "${NAME}-ignore-auth" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml)
if [[ -n "$OUTPUT" ]]; then
  echo -e "---\n# AuthorizationPolicy (ignore-auth)"
  echo "$OUTPUT" | yq eval '
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
fi

OUTPUT=$(kubectl get authorizationpolicy "${NAME}-require-auth" -n "$NAMESPACE" --context "$KUBECONTEXT" -o yaml)
if [[ -n "$OUTPUT" ]]; then
  echo -e "---\n# AuthorizationPolicy (require-auth)"
  echo "$OUTPUT" | yq eval '
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
fi