#!/bin/bash

# Deploy jennah-worker to all worker VMs
# This script automates the process of:
# 1. Stopping and removing old container
# 2. Pulling latest image
# 3. Running new container with proper configuration

set -e  # Exit on error

# Worker configuration: array of "vm-name:zone" pairs
WORKERS=(
  "jennah-dp:asia-northeast1-a"
  "jennah-dp-2:asia-northeast1-a"
  "jennah-dp-3:asia-northeast1-b"
)

# Common configuration
PROJECT_ID="labs-169405"
BATCH_REGION="asia-northeast1"
DB_INSTANCE="alphaus-dev"
DB_DATABASE="main"
WORKER_PORT="8081"
JOB_CONFIG_PATH="/config/job-config.json"
WORKER_LEASE_TTL_SECONDS="30"
WORKER_CLAIM_INTERVAL_SECONDS="5"
CLOUD_TASKS_QUEUE_ID="jennah-simple"
CLOUD_RUN_IMAGE_REGISTRY="gcr.io/$PROJECT_ID"
SERVICE_ACCOUNT="gcp-sa-dev-interns@$PROJECT_ID.iam.gserviceaccount.com"
IMAGE="asia.gcr.io/$PROJECT_ID/jennah-worker:latest"

# Docker run command (will be executed remotely)
DOCKER_COMMAND='docker run -d \
  --name jennah-worker \
  --restart always \
  -p 8081:8081 \
  -e BATCH_PROVIDER=gcp \
  -e BATCH_PROJECT_ID='"$PROJECT_ID"' \
  -e BATCH_REGION='"$BATCH_REGION"' \
  -e DB_PROVIDER=spanner \
  -e DB_PROJECT_ID='"$PROJECT_ID"' \
  -e DB_INSTANCE='"$DB_INSTANCE"' \
  -e DB_DATABASE='"$DB_DATABASE"' \
  -e WORKER_PORT='"$WORKER_PORT"' \
  -e JOB_CONFIG_PATH='"$JOB_CONFIG_PATH"' \
  -e WORKER_ID=WORKER_ID_PLACEHOLDER \
  -e WORKER_LEASE_TTL_SECONDS='"$WORKER_LEASE_TTL_SECONDS"' \
  -e WORKER_CLAIM_INTERVAL_SECONDS='"$WORKER_CLAIM_INTERVAL_SECONDS"' \
  -e CLOUD_TASKS_TARGET_URL=http://localhost:'"$WORKER_PORT"'/jennah.v1.DeploymentService/SubmitJob \
  -e CLOUD_TASKS_QUEUE_ID='"$CLOUD_TASKS_QUEUE_ID"' \
  -e CLOUD_RUN_ENABLED=true \
  -e CLOUD_RUN_IMAGE_REGISTRY='"$CLOUD_RUN_IMAGE_REGISTRY"' \
  -e CLOUD_TASKS_SERVICE_ACCOUNT='"$SERVICE_ACCOUNT"' \
  -e PUBSUB_ENABLED=true \
  -e PUBSUB_TOPIC_PREFIX=jennah-events- \
  -e PUBSUB_PROJECT_ID='"$PROJECT_ID"' \
  '"$IMAGE"' \
  serve'

# Function to deploy to a single worker
deploy_worker() {
  local worker_config="$1"
  local vm_name="${worker_config%:*}"
  local zone="${worker_config#*:}"
  
  echo "========================================="
  echo "Deploying to $vm_name in zone $zone"
  echo "========================================="
  
  # Create commands to run on remote VM
  REMOTE_COMMANDS=$(cat <<EOF
#!/bin/bash
set -e

echo "Stopping jennah-worker container..."
docker stop jennah-worker 2>/dev/null || echo "Container not running"

echo "Removing jennah-worker container..."
docker rm jennah-worker 2>/dev/null || echo "Container not found"

echo "Pulling latest image..."
docker pull $IMAGE

echo "Starting jennah-worker container..."
${DOCKER_COMMAND/WORKER_ID_PLACEHOLDER/$vm_name}

echo "Deployment complete for $vm_name"
EOF
)
  
  # Execute commands on remote VM
  gcloud compute ssh "$vm_name" \
    --zone="$zone" \
    --command="$REMOTE_COMMANDS"
  
  echo "✓ Successfully deployed to $vm_name"
  echo ""
}

# Main deployment loop
echo "Starting deployment to all workers..."
echo ""

for worker_config in "${WORKERS[@]}"; do
  deploy_worker "$worker_config"
done

echo "========================================="
echo "✓ All workers deployed successfully!"
echo "========================================="
