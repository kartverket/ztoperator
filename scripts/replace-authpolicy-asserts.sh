#!/bin/bash

TEST_DIR="./test/chainsaw/authpolicy"
GET_ISTIO_SCRIPT="./scripts/get-generated-istio-resources.sh"
TARGET_NAMESPACE="lars"

if [ ! -x "$GET_ISTIO_SCRIPT" ]; then
  echo "❌ Script $GET_ISTIO_SCRIPT not found or not executable."
  exit 1
fi

for test_path in "$TEST_DIR"/*; do
  if [[ "$(basename "$test_path")" == "path_validation" ]]; then
    echo "⏭️ Skipping directory: $test_path"
    continue
  fi
  test_name=$(basename "$test_path")
  echo "🔍 Processing test: $test_name"

  AUTH_POLICY_FILES=$(find "$test_path" -maxdepth 1 -type f -name '*authpolicy*.yaml' ! -name '*assert*')

  for AUTH_POLICY_FILE in $AUTH_POLICY_FILES; do
    if [ ! -f "$AUTH_POLICY_FILE" ]; then
      echo "  ⚠️ File $AUTH_POLICY_FILE not found, skipping."
      continue
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
        continue
      fi
      ISTIO_RESOURCES="${ISTIO_RESOURCES}${RES}"$'\n---\n'
    done


    if [ $? -ne 0 ] || [ -z "$ISTIO_RESOURCES" ]; then
      echo "  ❌ Failed to retrieve Istio resources for $AUTH_POLICY_FILE"
      continue
    fi

    echo "  💾 Writing generated output to $ASSERT_FILE"
    echo "$ISTIO_RESOURCES" > "$ASSERT_FILE"
    echo "✅ Completed $AUTH_POLICY_FILE"

    for RESOURCE_NAME in $RESOURCE_NAMES; do
      echo "  🗑️ Deleting AuthPolicy $RESOURCE_NAME"
      kubectl delete authpolicy "$RESOURCE_NAME" -n "$TARGET_NAMESPACE"
    done

    echo "  ⏳ Waiting 1s for reconciliation..."
    sleep 1

    echo "-----------------------------"
  done
done

echo "🏁 All test cases processed."