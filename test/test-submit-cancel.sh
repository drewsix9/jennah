#!/bin/bash

# Use consistent OAuth headers so gateway assigns same tenant
OAUTH_EMAIL="test.user@example.com"
OAUTH_USER_ID="test-user-$(date +%s)"
OAUTH_PROVIDER="test"

# 1. SUBMIT JOB
echo "=== Submitting Job ==="
JOB_RESPONSE=$(curl -s -X POST https://jennah-gateway-382915581671.asia-northeast1.run.app/jennah.v1.DeploymentService/SubmitJob \
  -H "Content-Type: application/json" \
  -H "X-OAuth-Email: $OAUTH_EMAIL" \
  -H "X-OAuth-UserId: $OAUTH_USER_ID" \
  -H "X-OAuth-Provider: $OAUTH_PROVIDER" \
  -d '{
    "name": "test-job-'$(date +%s)'",
    "image_uri": "busybox:latest",
    "command": ["sh", "-c", "echo Running job && sleep 30"]
  }')

echo "$JOB_RESPONSE" | jq .

# Extract job_id (camelCase from response)
JOB_ID=$(echo "$JOB_RESPONSE" | jq -r '.jobId')

if [ -z "$JOB_ID" ] || [ "$JOB_ID" = "null" ]; then
  echo "Error: Failed to extract jobId from response"
  exit 1
fi

echo -e "\n✓ Job ID: $JOB_ID"
echo "✓ OAuth User ID: $OAUTH_USER_ID (determines tenant)"

# Wait before cancel
echo -e "\nWaiting 5 seconds before cancelling..."
sleep 5

# 2. CANCEL JOB (use same OAuth headers!)
echo -e "\n=== Cancelling Job ==="
curl -s -X POST https://jennah-gateway-382915581671.asia-northeast1.run.app/jennah.v1.DeploymentService/CancelJob \
  -H "Content-Type: application/json" \
  -H "X-OAuth-Email: $OAUTH_EMAIL" \
  -H "X-OAuth-UserId: $OAUTH_USER_ID" \
  -H "X-OAuth-Provider: $OAUTH_PROVIDER" \
  -d '{
    "job_id": "'$JOB_ID'"
  }' | jq .

echo -e "\n✓ Done"