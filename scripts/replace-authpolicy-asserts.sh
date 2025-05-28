#!/bin/bash

TEST_DIR="./test/chainsaw/authpolicy"
GET_ISTIO_SCRIPT="./scripts/get-generated-istio-resources.sh"
TARGET_NAMESPACE="lars"

if [ ! -x "$GET_ISTIO_SCRIPT" ]; then
  echo "❌ Script $GET_ISTIO_SCRIPT not found or not executable."
  exit 1
fi

for test_path in "$TEST_DIR"/*/; do
  test_name=$(basename "$test_path")
  AUTH_POLICY_FILE="${test_path}authpolicy.yaml"
  ASSERT_FILE="${test_path}authpolicy-assert.yaml"

  echo "🔍 Processing test: $test_name"

  if [ ! -f "$AUTH_POLICY_FILE" ]; then
    echo "  ⚠️ Skipping $test_name — authpolicy.yaml not found"
    continue
  fi

  echo "  📥 Applying $AUTH_POLICY_FILE"
  kubectl apply -n "$TARGET_NAMESPACE" -f "$AUTH_POLICY_FILE"

  echo "  ⏳ Waiting 2s for reconciliation..."
  sleep 2

  echo "  📄 Generating Istio resources..."
  ISTIO_RESOURCES=$($GET_ISTIO_SCRIPT --name auth-policy --namespace "$TARGET_NAMESPACE")

  if [ $? -ne 0 ] || [ -z "$ISTIO_RESOURCES" ]; then
    echo "  ❌ Failed to get Istio resources for $test_name"
    continue
  fi

  echo "  💾 Writing generated output to $ASSERT_FILE"
  echo "$ISTIO_RESOURCES" > "$ASSERT_FILE"

  echo "✅ Completed $test_name"
  echo "-----------------------------"
done

echo "🏁 All test cases processed."