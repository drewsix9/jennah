# Pub/Sub Frontend Implementation Guide

## Overview

The Jennah backend publishes terminal job events to a **single shared Pub/Sub topic** (`jennah-job-events`). All tenants' events are published to this one topic with `tenant_id` included in both the message payload and Pub/Sub attributes. The frontend consumer service (deployed as a Cloud Run service) subscribes to this topic and translates events into user-facing notifications (Slack, email, webhooks, in-app alerts). Tenant isolation is enforced at the application and data layer, not at the Pub/Sub topic level.

## Architecture

```
┌──────────────────────────┐
│      Jennah Worker       │
│       (Backend)          │
└────────────┬─────────────┘
             │ publishes job.terminal event
             │ (tenant_id in payload + attributes)
             ▼
┌──────────────────────────────────────────────────┐
│       Shared Pub/Sub Topic                       │
│       jennah-job-events                          │
│                                                  │
│    └─► jennah-job-events-consumer-push           │
│        (push subscription → consumer service)    │
└──────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────┐
│    Jennah Consumer       │
│   (Cloud Run Service)    │
└────────────┬─────────────┘
             │
             ├─► In-App Notification DB (Spanner)
             ├─► Slack Integration
             ├─► Email Service
             └─► Webhook Dispatcher
             ▼
┌──────────────────────────┐
│    User Notification     │
│  (Slack, Email,          │
│   In-App, Webhooks)      │
└──────────────────────────┘
```

## Topic

All job terminal events are published to a single shared topic:

```
jennah-job-events
```

- Full topic resource name: `projects/<project>/topics/jennah-job-events`.
- Configurable via the `PUBSUB_TOPIC_ID` environment variable (default: `jennah-job-events`).

### Topic Lifecycle

- **Created by**: The worker, lazily, on the first publish.
- **Created how**: `CreateTopic` with `AlreadyExists` handled idempotently.
- **Deleted**: Not automatically deleted.

## Subscription

The worker auto-creates a single push subscription when `CONSUMER_PUSH_URL` is set:

```
jennah-job-events-consumer-push   → Consumer Cloud Run service
```

Additional subscriptions can be added by ops or infrastructure tooling for analytics, audit, or other downstream consumers:

```
jennah-job-events-analytics       → BigQuery / analytics pipeline
jennah-job-events-audit           → Firestore / Cloud Logging
```

## Event Payload Structure

The frontend receives events with this structure:

```json
{
  "event_id": "uuid-for-idempotency",
  "event_type": "job.terminal",
  "tenant_id": "tenant-uuid",
  "job_id": "job-uuid",
  "final_status": "COMPLETED|FAILED|CANCELLED",
  "previous_status": "RUNNING|PENDING|...",
  "occurred_at": "2026-03-09T12:00:00Z",
  "user_email": "submitter@example.com",
  "service_tier": "COMPLEX|SIMPLE",
  "assigned_service": "CLOUD_BATCH|CLOUD_RUN_JOB",
  "cloud_resource_path": "projects/.../jobs/...",
  "error_message": "optional error details",
  "job_name": "optional user-provided job name"
}
```

Pub/Sub **attributes** (for efficient filtering):

```
event_type: job.terminal
tenant_id: tenant-uuid
job_id: job-uuid
status: COMPLETED|FAILED|CANCELLED
```

## Recommended Frontend Architecture

### 1. Subscription Strategy

A single push subscription (`jennah-job-events-consumer-push`) delivers all events to the consumer Cloud Run service. Additional pull or push subscriptions can be created on the same topic for analytics, audit, or other pipelines.

Each subscription maintains an independent message queue and can be consumed at different rates.

**Provisioning**: The worker auto-creates the consumer push subscription when `CONSUMER_PUSH_URL` is set. Additional subscriptions are created by ops or infrastructure tooling.

### 2. Consumer Service (Cloud Run)

**Purpose**: Receive Pub/Sub events and route to notification channels

**Responsibilities**:

- Parse the incoming Pub/Sub message
- Extract tenant/user identity from event
- Look up user preferences (notification channels, frequency, quiet hours)
- Route to appropriate notification handler (Slack, email, webhook, etc.)
- Handle failures gracefully (retry, dead-letter queue)
- Track delivery status and audit trail

**Idempotency**: Use `event_id` to detect and deduplicate duplicate messages (Pub/Sub at-least-once guarantee).

### 3. Notification Channels

#### Slack Integration

- Parse event → format Slack message
- Thread together job submission + completion notification
- Include status indicator (✅ COMPLETED, ❌ FAILED, ⏸ CANCELLED)
- Link to job details in UI
- Optionally include GCP resource path for debugging

#### Email Notifications

- Template-based email generation
- Different templates for COMPLETED, FAILED, CANCELLED
- Include error details for failures
- Link to job logs/details in UI
- Batch notifications (send digest periodically vs immediately based on user preference)

#### Webhooks

- Allow tenants to register webhook endpoints
- POST the full event payload to their webhook
- Include HMAC signature for security
- Retry logic for failed deliveries
- Dead-letter queue for consistently failing webhooks

#### In-App Notifications

- Store event in database (Firestore, Cloud Datastore, or app's own DB)
- Show as unread notification to user
- Mark as read when user views details
- Archive/delete after N days
- Real-time push to frontend via WebSocket or polling

### 4. User Preference Management

Store preferences in database for each user/tenant:

```
UserNotificationPreferences:
  - tenant_id
  - user_email
  - channels: [slack, email, webhook, in-app]
  - quiet_hours: (optional start/end time)
  - notification_frequency: [immediate, digest, disabled]
  - slack_webhook_url: (if using Slack)
  - custom_webhook_url: (if custom webhook)
  - email_address: (override default)
```

Logic:

- If `quiet_hours` active → delay notification until hours resume
- If `notification_frequency: digest` → batch notifications and send once daily/weekly
- If `notification_frequency: disabled` → skip notification but log event
- If channel not in preferences → don't notify via that channel

### 5. Failure Handling

**Best Practices**:

- Don't fail silently — log all notification failures
- Use exponential backoff for retries (up to 3-5 attempts)
- Implement a dead-letter queue for events that fail repeatedly
- Monitor failure metrics in Cloud Logging
- Alert ops if notification success rate drops below threshold

**Error Cases**:

- Slack webhook is invalid → store in dead-letter, alert
- Email recipient bounced → mark user as inactive, alert
- Webhook endpoint unreachable → notify tenant to check integration
- Database error → requeue message (Pub/Sub will retry)

### 6. Audit & Observability

**Logging**:

- Log each notification event (which channel, to whom, timestamp)
- Include event_id for tracing
- Track success/failure per channel
- Store reasons for failures

**Metrics** (Cloud Monitoring):

- Notification latency (event published → notification sent)
- Success rate by channel
- Retry count distribution
- Dead-letter queue depth

**Alerting**:

- Alert if latency > 30 seconds
- Alert if success rate < 95%
- Alert if dead-letter queue grows

### 7. Handling Duplicate Messages

Pub/Sub guarantees at-least-once delivery, so duplicates can occur:

1. **Store seen event IDs**: Keep a cache/database of recently processed `event_id` values
2. **Deduplication window**: Remember events for ~24 hours (configurable)
3. **Idempotent operations**: If a duplicate is detected, return success without re-notifying

Example:

```
if (cache.exists(event.event_id)) {
  log.info("Duplicate event, skipping", event.event_id)
  return 200  // Ack to Pub/Sub
}
cache.set(event.event_id, true)
// Process notification
```

### 8. Tenant Isolation

All events flow through a single shared topic. Tenant isolation is enforced at the application and data layer:

- **Payload isolation**: Every message carries `tenant_id` in both the JSON payload and the Pub/Sub attributes. The consumer uses this to store notifications per tenant in Spanner.
- Extract `tenant_id` from event payload for validation.
- Look up tenant record to get owner's contact info.
- Validate tenant can receive notifications (not suspended, etc.).
- Log `tenant_id` in all audit records.

**Security considerations**:

- The consumer service is internal (Cloud Run with IAM authentication); external tenants do not subscribe directly to the topic.
- Webhook endpoints should be verified (HMAC signature).
- Email addresses validated (no cross-tenant leaks).
- Slack integrations per-tenant (don't share workspace tokens).

## Implementation Phases

### Phase 1: Basic Email Notifications

- Consume Pub/Sub events
- Parse job details
- Send email to submitter on COMPLETED/FAILED
- Simple logging and error handling

### Phase 2: Slack Integration

- Add Slack channel configuration
- Format prettier Slack messages
- Link to UI job details
- Thread-aware notifications

### Phase 3: User Preferences UI

- Dashboard for users to configure:
  - Which channels to use
  - Quiet hours
  - Batch vs immediate
- API to store/retrieve preferences

### Phase 4: Advanced Features

- Webhooks for custom integrations
- In-app notification center
- Digest notifications (daily/weekly summary)
- Metric export for monitoring

### Phase 5: Observability & Ops

- Comprehensive logging and metrics
- Dead-letter queue mgmt UI
- Notification testing tool
- Admin dashboard for troubleshooting

## Deployment Considerations

### Where to Run

- **Cloud Function**: Simplest, serverless, auto-scales, low cost (event-driven)
- **Cloud Run**: More control, stateful if needed, can handle heavier processing
- **App Engine**: If already using in your stack

### Permissions Required

**Worker (backend)**:

- `pubsub.editor` on the GCP project (to lazily create the topic and subscription)
- `pubsub.publisher` on `jennah-job-events`

**Consumer service**:

- No Pub/Sub permissions needed when using push delivery (Pub/Sub pushes to the consumer endpoint)
- Write access to notification systems (Slack API, SendGrid, custom webhooks)
- Read access to user/tenant database for preferences
- Write access to audit log

### Scaling

- Pub/Sub automatically distributes load across push endpoints
- Cloud Run auto-scales based on incoming request load
- Use Pub/Sub message attributes (`tenant_id`, `status`) for downstream filtering
- Set appropriate concurrency limits to avoid overwhelming downstream services
- Monitor subscription backlog and adjust as needed

### Backend Configuration

The worker uses environment variables to configure Pub/Sub:

| Variable            | Default                            | Description                                   |
| ------------------- | ---------------------------------- | --------------------------------------------- |
| `PUBSUB_ENABLED`    | `false`                            | Set to `true` to enable Pub/Sub notifications |
| `PUBSUB_PROJECT_ID` | (falls back to `BATCH_PROJECT_ID`) | GCP project owning the topic                  |
| `PUBSUB_TOPIC_ID`   | `jennah-job-events`                | Shared topic for all job terminal events      |

## Testing Strategy

### Unit Tests

- Event parsing and validation
- Notification formatting (email templates, Slack messages)
- User preference lookup and precedence logic

### Integration Tests

- Send test event through Pub/Sub
- Verify notification received in Slack/email test account
- Verify deduplication works
- Verify preferences are respected

### Staging Environment

- Deploy consumer to staging subscription
- Submit jobs on staging worker
- Verify end-to-end notifications work

### Production Rollout

- Start with email only (safest)
- Monitor success rate, latency, errors
- Add Slack integration when confident
- Add webhooks/advanced features increments

## Monitoring Checklist

- [ ] Error rate by notification channel
- [ ] Latency percentiles (p50, p95, p99)
- [ ] Dead-letter queue depth and growth
- [ ] Duplicate detection rate
- [ ] User preference coverage (% with preferences set)
- [ ] Webhook delivery success rate
- [ ] Email bounce rate
- [ ] Slack API rate limit usage

## Future Enhancements

- **Job log streaming**: Attach truncated job logs to notifications
- **Cost attribution**: Include job cost in completion notifications
- **Performance insights**: Add job performance summary (duration, resource usage)
- **Retry suggestions**: Smart suggestions if job failed (increase timeout, memory, etc.)
- **Batch operations**: Notify on bulk job submissions
- **Team notifications**: Notify team leads in addition to submitter
- **Custom templates**: Allow tenants to customize notification wording
