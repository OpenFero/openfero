#!/bin/bash

# Quick test of OpenFero deduplication functionality
# This is a minimal example that demonstrates the core feature
# Note: OpenFero runs locally, jobs are created in the Kubernetes cluster

set -e

OPENFERO_URL="${OPENFERO_URL:-http://localhost:8080}"
NAMESPACE="${OPENFERO_NAMESPACE:-openfero}"
TEST_GROUP_KEY="test-group-123"

# Function to calculate the hash of a groupKey using OpenFero's hash function
hash_group_key() {
	local groupKey="$1"
	go run test-hash-util.go "$groupKey"
}

echo "🧪 Testing OpenFero Deduplication"
echo "Server: $OPENFERO_URL (running locally)"
echo "Namespace: $NAMESPACE"
echo "GroupKey: $TEST_GROUP_KEY"

# Calculate hashed groupKey
HASHED_GROUP_KEY=$(hash_group_key "$TEST_GROUP_KEY")
echo "Hashed GroupKey: $HASHED_GROUP_KEY"
echo

# Check if OpenFero is accessible
echo "🔍 Checking OpenFero availability..."
if ! curl -s -f "$OPENFERO_URL/health" >/dev/null 2>&1; then
	echo "❌ Error: OpenFero is not accessible at $OPENFERO_URL"
	echo "Make sure OpenFero is running locally"
	exit 1
fi
echo "✅ OpenFero is running locally"

# Check if kubectl works
echo "🔍 Checking Kubernetes cluster access..."
if ! kubectl cluster-info >/dev/null 2>&1; then
	echo "❌ Error: Cannot access Kubernetes cluster"
	echo "Make sure kubectl is configured properly"
	exit 1
fi
echo "✅ Kubernetes cluster is accessible"

# Test 1: Send first alert
echo "📤 Sending first alert..."
curl -s -X POST "$OPENFERO_URL/alerts" \
	-H "Content-Type: application/json" \
	-d "{
    \"version\": \"4\",
    \"groupKey\": \"$TEST_GROUP_KEY\",
    \"status\": \"firing\",
    \"receiver\": \"openfero\",
    \"groupLabels\": {
      \"alertname\": \"TestAlert\",
      \"severity\": \"warning\"
    },
    \"commonLabels\": {
      \"alertname\": \"TestAlert\",
      \"severity\": \"warning\"
    },
    \"alerts\": [
      {
        \"status\": \"firing\",
        \"labels\": {
          \"alertname\": \"TestAlert\",
          \"severity\": \"warning\",
          \"instance\": \"test-node\"
        },
        \"annotations\": {
          \"summary\": \"Test alert for deduplication\"
        },
        \"startsAt\": \"2024-12-17T10:00:00Z\"
      }
    ]
  }" && echo " ✅ First alert sent"

sleep 2

# Check job count
echo "🔍 Checking jobs created..."
job_count=$(kubectl get jobs -n "$NAMESPACE" -l "openfero.io/group-key=$HASHED_GROUP_KEY" --no-headers 2>/dev/null | wc -l)
echo "Jobs found: $job_count (expected: 1)"

# Test 2: Send duplicate alert
echo "📤 Sending duplicate alert..."
curl -s -X POST "$OPENFERO_URL/alerts" \
	-H "Content-Type: application/json" \
	-d "{
    \"version\": \"4\",
    \"groupKey\": \"$TEST_GROUP_KEY\",
    \"status\": \"firing\",
    \"receiver\": \"openfero\",
    \"groupLabels\": {
      \"alertname\": \"TestAlert\", 
      \"severity\": \"warning\"
    },
    \"commonLabels\": {
      \"alertname\": \"TestAlert\",
      \"severity\": \"warning\"
    },
    \"alerts\": [
      {
        \"status\": \"firing\",
        \"labels\": {
          \"alertname\": \"TestAlert\",
          \"severity\": \"warning\",
          \"instance\": \"test-node-2\"
        },
        \"annotations\": {
          \"summary\": \"Duplicate test alert\"
        },
        \"startsAt\": \"2024-12-17T10:01:00Z\"
      }
    ]
  }" && echo " ✅ Duplicate alert sent"

sleep 2

# Check job count again
echo "🔍 Checking jobs after duplicate..."
job_count_after=$(kubectl get jobs -n "$NAMESPACE" -l "openfero.io/group-key=$HASHED_GROUP_KEY" --no-headers 2>/dev/null | wc -l)
echo "Jobs found: $job_count_after (expected: 1)"

# Results
echo
if [ "$job_count_after" -eq 1 ]; then
	echo "🎉 SUCCESS: Deduplication is working! Only 1 job created from 2 identical alerts"
else
	echo "❌ FAILURE: Expected 1 job, found $job_count_after"
fi

echo
echo "🧹 To clean up: kubectl delete jobs -n $NAMESPACE -l \"openfero.io/group-key=$HASHED_GROUP_KEY\""
