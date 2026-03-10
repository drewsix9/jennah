#!/bin/bash

# Deploy jennah-worker to all worker VMs
# This script automates the process of:
# 1. Stopping and removing old container
# 2. Pulling latest image
# 3. Running new container with proper configuration

set -e  # Exit on error

GCLOUD_BIN="${GCLOUD_BIN:-gcloud}"
case "${OSTYPE:-}" in
  msys*|cygwin*|win32*)
    if command -v gcloud.cmd >/dev/null 2>&1; then
      GCLOUD_BIN="$(command -v gcloud.cmd)"
    fi
    ;;
esac

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
WORKER_LEASE_TTL_SECONDS="10"
WORKER_CLAIM_INTERVAL_SECONDS="3"
CLOUD_TASKS_QUEUE_ID="jennah-simple"
CLOUD_RUN_IMAGE_REGISTRY="gcr.io/$PROJECT_ID"
SERVICE_ACCOUNT="gcp-sa-dev-interns@$PROJECT_ID.iam.gserviceaccount.com"
IMAGE_TAG="${IMAGE_TAG:-latest}"
IMAGE="asia-docker.pkg.dev/$PROJECT_ID/asia.gcr.io/jennah-worker:$IMAGE_TAG"

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
  -e PUBSUB_TOPIC_ID=jennah-job-events \
  -e PUBSUB_PROJECT_ID='"$PROJECT_ID"' \
  -e CONSUMER_PUSH_URL='"$CONSUMER_PUSH_URL"' \
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
  
  # Keep the remote command on one line so Git Bash + gcloud.cmd on Windows
  # do not mangle multiline --command payloads during argument translation.
  REMOTE_COMMANDS="set -e; \
echo 'Stopping jennah-worker container...'; \
docker stop jennah-worker 2>/dev/null || echo 'Container not running'; \
echo 'Removing jennah-worker container...'; \
docker rm jennah-worker 2>/dev/null || echo 'Container not found'; \
echo 'Configuring Docker auth for Artifact Registry...'; \
gcloud auth configure-docker asia-docker.pkg.dev --quiet >/dev/null 2>&1; \
echo 'Pulling latest image...'; \
docker pull $IMAGE; \
echo 'Starting jennah-worker container...'; \
${DOCKER_COMMAND/WORKER_ID_PLACEHOLDER/$vm_name}; \
docker ps --filter name=jennah-worker --format '{{.Names}} {{.Image}} {{.Status}}' | grep '^jennah-worker '; \
echo 'Deployment complete for $vm_name'"
  
  # Execute commands on remote VM
  "$GCLOUD_BIN" compute ssh "$vm_name" \
    --zone="$zone" \
    --command="$REMOTE_COMMANDS"
  
  echo "[OK] Successfully deployed to $vm_name"
  echo ""
}

# Main deployment loop
echo "Starting deployment to all workers..."
echo "Worker image: $IMAGE"
echo ""

for worker_config in "${WORKERS[@]}"; do
  deploy_worker "$worker_config"
done

echo "========================================="
echo "[OK] All workers deployed successfully!"
echo "========================================="
