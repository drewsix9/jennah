package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JobTerminalEvent is the payload published to Pub/Sub when a job reaches
// a terminal state (COMPLETED, FAILED, or CANCELLED).
type JobTerminalEvent struct {
	EventID           string `json:"event_id"`
	EventType         string `json:"event_type"`
	TenantID          string `json:"tenant_id"`
	JobID             string `json:"job_id"`
	FinalStatus       string `json:"final_status"`
	PreviousStatus    string `json:"previous_status"`
	OccurredAt        string `json:"occurred_at"`
	UserEmail         string `json:"user_email,omitempty"`
	ServiceTier       string `json:"service_tier,omitempty"`
	AssignedService   string `json:"assigned_service,omitempty"`
	CloudResourcePath string `json:"cloud_resource_path,omitempty"`
	ErrorMessage      string `json:"error_message,omitempty"`
	JobName           string `json:"job_name,omitempty"`
}

// Notifier publishes job terminal events. Implementations must be safe
// for concurrent use.
type Notifier interface {
	// PublishJobTerminalEvent publishes an event for a job that reached a terminal state.
	// The implementation resolves the correct tenant-scoped topic from the event's TenantID.
	PublishJobTerminalEvent(ctx context.Context, event JobTerminalEvent) error

	// Close releases any resources held by the notifier.
	Close() error
}

// TenantTopicID returns the deterministic Pub/Sub topic ID for a tenant.
// Format: jennah-events-<tenantId>
func TenantTopicID(topicPrefix, tenantID string) string {
	return topicPrefix + tenantID
}

// PubSubNotifier publishes terminal events to per-tenant Pub/Sub topics.
// Topics are created lazily on the first publish for each tenant and cached
// for the lifetime of the notifier.
// If ConsumerPushURL is set, a push subscription pointing to the consumer
// service is also ensured whenever a new topic is created.
type PubSubNotifier struct {
	client         *pubsub.Client
	topicPrefix    string
	ConsumerPushURL string // e.g. "https://jennah-consumer-xxx.run.app/pubsub/push"

	mu     sync.RWMutex
	topics map[string]*pubsub.Topic // tenantID → *pubsub.Topic
}

// NewPubSubNotifier creates a notifier that publishes to per-tenant Pub/Sub topics.
// topicPrefix is prepended to each tenant ID to form the topic name
// (e.g. "jennah-events-" → topic "jennah-events-<tenantId>").
// The caller must call Close when the notifier is no longer needed.
func NewPubSubNotifier(client *pubsub.Client, topicPrefix string) *PubSubNotifier {
	return &PubSubNotifier{
		client:      client,
		topicPrefix: topicPrefix,
		topics:      make(map[string]*pubsub.Topic),
	}
}

// topicFor returns a cached topic handle for the tenant, creating the Pub/Sub
// topic on demand if it does not yet exist.
func (n *PubSubNotifier) topicFor(ctx context.Context, tenantID string) (*pubsub.Topic, error) {
	// Fast path: topic already cached.
	n.mu.RLock()
	t, ok := n.topics[tenantID]
	n.mu.RUnlock()
	if ok {
		return t, nil
	}

	// Slow path: ensure topic exists, then cache it.
	n.mu.Lock()
	defer n.mu.Unlock()

	// Double-check after acquiring write lock.
	if t, ok = n.topics[tenantID]; ok {
		return t, nil
	}

	topicID := TenantTopicID(n.topicPrefix, tenantID)
	t = n.client.Topic(topicID)

	exists, err := t.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check topic %s: %w", topicID, err)
	}
	if !exists {
		t, err = n.client.CreateTopic(ctx, topicID)
		if err != nil {
			// Another process may have created it concurrently.
			if status.Code(err) == codes.AlreadyExists {
				t = n.client.Topic(topicID)
			} else {
				return nil, fmt.Errorf("create topic %s: %w", topicID, err)
			}
		}
		log.Printf("Created Pub/Sub topic %s for tenant %s", topicID, tenantID)

		// If a consumer push URL is configured, ensure a push subscription
		// exists so new events are delivered to the consumer service.
		if n.ConsumerPushURL != "" {
			if err := n.ensurePushSubscription(ctx, t, topicID, tenantID); err != nil {
				// Non-fatal: log and continue; messages can be consumed later.
				log.Printf("WARNING: could not create push subscription for topic %s: %v", topicID, err)
			}
		}
	}

	n.topics[tenantID] = t
	return t, nil
}

// PublishJobTerminalEvent resolves the tenant-scoped topic, lazily creates it
// if needed, and publishes the event. It blocks until the publish is
// acknowledged or the context is cancelled.
func (n *PubSubNotifier) PublishJobTerminalEvent(ctx context.Context, event JobTerminalEvent) error {
	topic, err := n.topicFor(ctx, event.TenantID)
	if err != nil {
		return fmt.Errorf("resolve topic for tenant %s: %w", event.TenantID, err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	result := topic.Publish(ctx, &pubsub.Message{
		Data: data,
		Attributes: map[string]string{
			"event_type": event.EventType,
			"tenant_id":  event.TenantID,
			"job_id":     event.JobID,
			"status":     event.FinalStatus,
		},
	})

	serverID, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("publish event for job %s: %w", event.JobID, err)
	}
	log.Printf("Published terminal event for job %s to topic %s (msg id: %s, status: %s)",
		event.JobID, TenantTopicID(n.topicPrefix, event.TenantID), serverID, event.FinalStatus)
	return nil
}

// Close stops all cached topics (flushing pending publishes) and closes the client.
func (n *PubSubNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, t := range n.topics {
		t.Stop()
	}
	n.topics = nil
	return n.client.Close()
}

// ensurePushSubscription creates a push subscription for topicID → ConsumerPushURL
// if one does not already exist. Subscription ID: "<topicID>-consumer-push".
func (n *PubSubNotifier) ensurePushSubscription(ctx context.Context, t *pubsub.Topic, topicID, tenantID string) error {
	subID := topicID + "-consumer-push"
	sub := n.client.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return fmt.Errorf("check subscription %s: %w", subID, err)
	}
	if exists {
		return nil
	}
	_, err = n.client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
		Topic: t,
		PushConfig: pubsub.PushConfig{
			Endpoint: n.ConsumerPushURL,
		},
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("create push subscription %s: %w", subID, err)
	}
	log.Printf("Created push subscription %s → %s for tenant %s", subID, n.ConsumerPushURL, tenantID)
	return nil
}

// NoopNotifier silently discards all events. Used when Pub/Sub is disabled.
type NoopNotifier struct{}

// PublishJobTerminalEvent is a no-op.
func (n *NoopNotifier) PublishJobTerminalEvent(_ context.Context, event JobTerminalEvent) error {
	log.Printf("Notification disabled: would publish terminal event for job %s (status: %s)", event.JobID, event.FinalStatus)
	return nil
}

// Close is a no-op.
func (n *NoopNotifier) Close() error { return nil }

// BuildEvent constructs a JobTerminalEvent from the provided fields.
// eventID should be a unique identifier such as the transition UUID.
func BuildEvent(eventID, tenantID, jobID, finalStatus, previousStatus string) JobTerminalEvent {
	return JobTerminalEvent{
		EventID:        eventID,
		EventType:      "job.terminal",
		TenantID:       tenantID,
		JobID:          jobID,
		FinalStatus:    finalStatus,
		PreviousStatus: previousStatus,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339),
	}
}
