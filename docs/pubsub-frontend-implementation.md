# Pub/Sub Frontend Implementation Guide

## Overview

The Jennah backend publishes terminal job events to **per-tenant Pub/Sub topics**. Each tenant (identified by a stable `TenantId`) gets a dedicated topic that is lazily created on the first terminal event. The frontend should implement a consumer service that subscribes to the tenant-scoped topics and translates these backend events into user-facing notifications (Slack, email, webhooks, in-app alerts).

## Architecture

```
┌──────────────────────────┐
│      Jennah Worker       │
│       (Backend)          │
└────────────┬─────────────┘
             │ publishes job.terminal event
             │ (topic resolved from TenantId)
             ▼
┌──────────────────────────────────────────────────┐
│         Per-Tenant Pub/Sub Topics                │
│                                                  │
│  jennah-events-<tenantA>   jennah-events-<tenantB>
│        │                          │              │
│        ├─► Sub (Tenant A)         ├─► Sub (B)   │
│        ├─► Sub (Analytics A)      ├─► Sub (…)   │
│        └─► Sub (Audit A)         └─► Sub (…)    │
└──────────────────────────────────────────────────┘
             │
             ▼
┌──────────────────────────┐
│    Frontend Consumer     │
│  (Cloud Function or      │
│   Cloud Run Service)     │
└────────────┬─────────────┘
             │
             ├─► Slack Integration
             ├─► Email Service
             ├─► Webhook Dispatcher
             └─► In-App Notification DB
             ▼
┌──────────────────────────┐
│    User Notification     │
│  (Slack, Email,          │
│   In-App, Webhooks)      │
└──────────────────────────┘
```

## Topic Naming Convention

Each tenant's topic name is deterministic:

```
jennah-events-<TenantId>
```

- `TenantId` is a UUID (e.g. `550e8400-e29b-41d4-a716-446655440000`).
- Full topic resource name: `projects/<project>/topics/jennah-events-<TenantId>`.
- The prefix (`jennah-events-`) is configurable via the `PUBSUB_TOPIC_PREFIX` environment variable.

### Topic Lifecycle

- **Created by**: The worker, lazily, when the first terminal event for a tenant is published.
- **Created how**: `CreateTopic` with `AlreadyExists` handled idempotently.
- **Deleted**: Not automatically deleted. Ops should clean up topics for deprovisioned tenants.

## Subscription Naming Convention

Each tenant gets dedicated subscriptions. Recommended naming:

```
jennah-events-<TenantId>-notifications   → Frontend consumer
jennah-events-<TenantId>-analytics       → BigQuery / analytics pipeline
jennah-events-<TenantId>-audit           → Firestore / Cloud Logging
```

Subscriptions are **not** created by the backend. Frontend or infrastructure tooling provisions them from the deterministic topic name.

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

Each tenant gets dedicated subscriptions on their own topic:

- **`jennah-events-<TenantId>-notifications`** → Cloud Function/Run for user notifications
- **`jennah-events-<TenantId>-analytics`** → BigQuery for analytics/reporting
- **`jennah-events-<TenantId>-audit`** → Firestore/Cloud Logging for compliance

Each subscription maintains independent message queues and can be consumed at different rates.

**Provisioning**: Subscriptions should be created by the frontend or infrastructure tooling when a new tenant topic appears (or when the first event arrives). The backend only creates the topic; subscription ownership belongs to the consumer layer.

**Discovery**: Since topic names are deterministic (`jennah-events-<TenantId>`), the consumer can compute the expected topic name from the tenant ID and create subscriptions on demand.

### 2. Consumer Service (Cloud Function or Cloud Run)

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

### 8. Per-Tenant Isolation

Isolation is enforced at the Pub/Sub topic level — each tenant's events are published to a dedicated topic:

- **Topic isolation**: Tenant A's events go to `jennah-events-<tenantA>`, tenant B's to `jennah-events-<tenantB>`. No cross-tenant event leakage at the infrastructure layer.
- **Subscription isolation**: Each tenant's consumers subscribe only to their own topic.
- Extract `tenant_id` from event payload for additional in-app validation.
- Look up tenant record to get owner's contact info.
- Validate tenant can receive notifications (not suspended, etc.).
- Log `tenant_id` in all audit records.

**Security considerations**:

- **IAM per topic**: Grant `pubsub.subscriber` only on the tenant's own topic to that tenant's consumer identity, preventing cross-tenant access.
- Never expose one tenant's events to another.
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

- `pubsub.editor` or `pubsub.admin` on the GCP project (to lazily create per-tenant topics)
- `pubsub.publisher` on all `jennah-events-*` topics

**Frontend consumer**:

- `pubsub.subscriber` on per-tenant subscriptions
- `pubsub.editor` if the consumer auto-creates subscriptions on tenant topics
- Write access to notification systems (Slack API, SendGrid, custom webhooks)
- Read access to user/tenant database for preferences
- Write access to audit log

### Scaling

- Pub/Sub automatically distributes load across subscribers per topic
- Cloud Functions/Run auto-scale based on load
- Per-tenant topics naturally partition load — high-volume tenants don't affect others
- Set appropriate concurrency limits to avoid overwhelming downstream services
- Monitor backlog per tenant and adjust as needed

### Backend Configuration

The worker uses environment variables to configure per-tenant Pub/Sub:

| Variable              | Default                            | Description                                   |
| --------------------- | ---------------------------------- | --------------------------------------------- |
| `PUBSUB_ENABLED`      | `false`                            | Set to `true` to enable Pub/Sub notifications |
| `PUBSUB_PROJECT_ID`   | (falls back to `BATCH_PROJECT_ID`) | GCP project owning the topics                 |
| `PUBSUB_TOPIC_PREFIX` | `jennah-events-`                   | Prefix prepended to TenantId for topic names  |

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
