# Jennah Consumer Service

Receives Pub/Sub push deliveries for terminal job events and persists them as in-app notifications in Cloud Spanner. Deployed as a standalone Cloud Run service, separate from the worker and gateway.

## Endpoints

| Method | Path           | Purpose                        |
| ------ | -------------- | ------------------------------ |
| GET    | `/health`      | Health check (returns `ok`)    |
| POST   | `/pubsub/push` | Pub/Sub push delivery endpoint |

## Required Environment Variables

| Variable        | Example       | Description                   |
| --------------- | ------------- | ----------------------------- |
| `DB_PROJECT_ID` | `labs-169405` | GCP project for Cloud Spanner |
| `DB_INSTANCE`   | `alphaus-dev` | Spanner instance name         |
| `DB_DATABASE`   | `main`        | Spanner database name         |

Optional: `PORT` (default `8080`).

## Prerequisites

1. Run the notifications migration once in Spanner:

```bash
# Apply database/migrate-notifications.sql to your Spanner database.
```

2. Authenticate with GCP:

```bash
gcloud auth application-default login
```

## Local Development

```bash
# Run directly
DB_PROJECT_ID=labs-169405 DB_INSTANCE=alphaus-dev DB_DATABASE=main \
  go run ./cmd/consumer/

# Build binary
make consumer-build
```

## Docker

```bash
# Build image
make consumer-docker-build

# Run locally
make consumer-docker-run

# Push to Artifact Registry
make consumer-docker-push
```

## Cloud Run Deployment

```bash
# Deploy (sets DB env vars automatically)
make consumer-deploy

# Get deployed URL
make consumer-url

# Test health
make consumer-test-health
```

## Worker Integration

After deploying the consumer, set `CONSUMER_PUSH_URL` on the worker so it auto-creates Pub/Sub push subscriptions pointing to this service:

```bash
CONSUMER_PUSH_URL=https://<consumer-cloud-run-url>/pubsub/push
```

The worker reads this variable at startup and configures push subscriptions per tenant topic. See `cmd/worker/cmd/serve.go` for details.
