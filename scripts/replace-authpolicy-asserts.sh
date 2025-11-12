#!/bin/bash

TEST_DIR="./test/chainsaw/authpolicy"
GET_ISTIO_SCRIPT="./scripts/get-generated-istio-resources.sh"
TARGET_NAMESPACE="replace-asserts"

if [ ! -x "$GET_ISTIO_SCRIPT" ]; then
  echo "❌ Script $GET_ISTIO_SCRIPT not found or not executable."
  exit 1
fi

# Ensure target namespace exists
if ! kubectl get namespace "$TARGET_NAMESPACE" > /dev/null 2>&1; then
  echo "📦 Namespace '$TARGET_NAMESPACE' not found. Creating..."
  if ! kubectl create namespace "$TARGET_NAMESPACE" > /dev/null 2>&1; then
    echo "❌ Failed to create namespace '$TARGET_NAMESPACE'."
    exit 1
  fi
  kubectl label namespace "$TARGET_NAMESPACE" \
      name="$TARGET_NAMESPACE" \
      istio.io/rev=default \
      --overwrite
  echo "✅ Namespace '$TARGET_NAMESPACE' created and labeled successfully"
else
  echo "⛔️ Namespace '$TARGET_NAMESPACE' already exists, recreating it to ensure correctness"
  if ! kubectl delete namespace "$TARGET_NAMESPACE" > /dev/null 2>&1; then
    echo "❌ Failed to delete namespace '$TARGET_NAMESPACE'."
    exit 1
  fi
  if ! kubectl create namespace "$TARGET_NAMESPACE" > /dev/null 2>&1; then
    echo "❌ Failed to create namespace '$TARGET_NAMESPACE'."
    exit 1
  fi
  kubectl label namespace "$TARGET_NAMESPACE" \
      name="$TARGET_NAMESPACE" \
      istio.io/rev=default \
      --overwrite
  echo "✅ Namespace '$TARGET_NAMESPACE' recreated and labeled successfully"
fi


for test_path in "$TEST_DIR"/*; do
  test_name=$(basename "$test_path")
  echo "🔍 Processing test: $test_name"

  # If an oauth2-credentials.yaml sits next to the authpolicy file, apply it first
  OAUTH_CREDENTIALS_FILE="$test_path/oauth2-credentials.yaml"
  OAUTH_SECRETS=""
  if [ -f "$OAUTH_CREDENTIALS_FILE" ]; then
    echo "  🔑 Found oauth2-credentials.yaml — applying it to namespace '$TARGET_NAMESPACE'..."
    if kubectl apply -n "$TARGET_NAMESPACE" -f "$OAUTH_CREDENTIALS_FILE"; then
      # Capture any Secret names from the oauth2-credentials file
      OAUTH_SECRETS=$(yq e 'select(.kind == "Secret") | .metadata.name' "$OAUTH_CREDENTIALS_FILE" | grep -Ev '^(null|---)$' | tr '\n' ' ')
      if [ -n "$OAUTH_SECRETS" ]; then
        echo "  🧾 Will delete these Secrets during cleanup: $OAUTH_SECRETS"
      fi
      else
        echo "  ❌ Failed to apply oauth2-credentials.yaml"
        exit 1
      fi
  else
    echo "  ℹ️ No oauth2-credentials.yaml found next to $AUTH_POLICY_FILE, skipping."
  fi

  AUTH_POLICY_FILES=$(find "$test_path" -maxdepth 1 -type f -name '*authpolicy*.yaml' ! -name '*assert*')

  for AUTH_POLICY_FILE in $AUTH_POLICY_FILES; do
    if [ ! -f "$AUTH_POLICY_FILE" ]; then
      echo " ❌ File $AUTH_POLICY_FILE not found."
      exit 1
    fi

    ASSERT_FILE="${AUTH_POLICY_FILE%.yaml}-assert.yaml"
    echo "  📥 Applying $AUTH_POLICY_FILE"
    kubectl apply -n "$TARGET_NAMESPACE" -f "$AUTH_POLICY_FILE"

    echo "  ⏳ Waiting 2s for reconciliation..."
    sleep 2

    echo "  📄 Retrieving Istio resources..."
    RESOURCE_NAMES=$(yq e 'select(.kind == "AuthPolicy") | .metadata.name' "$AUTH_POLICY_FILE" | grep -Ev '^(null|---)$')
    ISTIO_RESOURCES=""

    for RESOURCE_NAME in $RESOURCE_NAMES; do
      echo "  🔄 Retrieving Istio resources for: $RESOURCE_NAME"
      RES=$($GET_ISTIO_SCRIPT --name "$RESOURCE_NAME" --namespace "$TARGET_NAMESPACE")
      if [ $? -ne 0 ] || [ -z "$RES" ]; then
        echo "  ❌ Failed to retrieve Istio resources for $RESOURCE_NAME"
        exit 1
      fi
      ISTIO_RESOURCES="${ISTIO_RESOURCES}${RES}"$'\n---\n'
    done


    if [ $? -ne 0 ] || [ -z "$ISTIO_RESOURCES" ]; then
      echo "  ❌ Failed to retrieve Istio resources for $AUTH_POLICY_FILE"
      exit 1
    fi

    echo "  💾 Writing generated output to $ASSERT_FILE"
    echo "$ISTIO_RESOURCES" > "$ASSERT_FILE"
    echo "✅ Completed $AUTH_POLICY_FILE"

    for RESOURCE_NAME in $RESOURCE_NAMES; do
      echo "  🗑️ Deleting AuthPolicy $RESOURCE_NAME"
      kubectl delete authpolicy "$RESOURCE_NAME" -n "$TARGET_NAMESPACE"
    done

    # Delete secrets that were applied from oauth2-credentials.yaml (if any)
    if [ -n "$OAUTH_SECRETS" ]; then
      for S in $OAUTH_SECRETS; do
        if kubectl get secret "$S" -n "$TARGET_NAMESPACE" > /dev/null 2>&1; then
          echo "  🗑️ Deleting Secret $S"
          kubectl delete secret "$S" -n "$TARGET_NAMESPACE"
        else
          echo "  ℹ️ No Secret $S found to delete"
        fi
      done
    fi


    echo "  ⏳ Waiting 1s for reconciliation..."
    sleep 1

    echo "-----------------------------"
  done
done

echo "🏁 All test cases processed."