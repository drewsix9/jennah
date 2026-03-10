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

// PubSubNotifier publishes terminal events to a shared Pub/Sub topic.
// The topic is created lazily on the first publish and cached for the
// lifetime of the notifier. Tenant isolation is maintained via the
// tenant_id attribute and payload field on each message.
// If ConsumerPushURL is set, a push subscription pointing to the consumer
// service is also ensured when the topic is first created.
type PubSubNotifier struct {
	client          *pubsub.Client
	topicID         string
	ConsumerPushURL string // e.g. "https://jennah-consumer-xxx.run.app/pubsub/push"

	once  sync.Once
	topic *pubsub.Topic
	err   error
}

// NewPubSubNotifier creates a notifier that publishes to a single shared
// Pub/Sub topic identified by topicID (e.g. "jennah-job-events").
// The caller must call Close when the notifier is no longer needed.
func NewPubSubNotifier(client *pubsub.Client, topicID string) *PubSubNotifier {
	return &PubSubNotifier{
		client:  client,
		topicID: topicID,
	}
}

// ensureTopic returns the cached topic handle, creating the Pub/Sub topic
// on demand if it does not yet exist. It is safe for concurrent use.
func (n *PubSubNotifier) ensureTopic(ctx context.Context) (*pubsub.Topic, error) {
	n.once.Do(func() {
		t := n.client.Topic(n.topicID)

		exists, err := t.Exists(ctx)
		if err != nil {
			n.err = fmt.Errorf("check topic %s: %w", n.topicID, err)
			return
		}
		if !exists {
			t, err = n.client.CreateTopic(ctx, n.topicID)
			if err != nil {
				if status.Code(err) == codes.AlreadyExists {
					t = n.client.Topic(n.topicID)
				} else {
					n.err = fmt.Errorf("create topic %s: %w", n.topicID, err)
					return
				}
			}
			log.Printf("Created Pub/Sub topic %s", n.topicID)
		}

		// If a consumer push URL is configured, ensure a push subscription
		// exists so events are delivered to the consumer service.
		if n.ConsumerPushURL != "" {
			if err := n.ensurePushSubscription(ctx, t); err != nil {
				log.Printf("WARNING: could not create push subscription for topic %s: %v", n.topicID, err)
			}
		}

		n.topic = t
	})
	return n.topic, n.err
}

// PublishJobTerminalEvent publishes the event to the shared topic. It blocks
// until the publish is acknowledged or the context is cancelled.
func (n *PubSubNotifier) PublishJobTerminalEvent(ctx context.Context, event JobTerminalEvent) error {
	topic, err := n.ensureTopic(ctx)
	if err != nil {
		return fmt.Errorf("resolve topic %s: %w", n.topicID, err)
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
		event.JobID, n.topicID, serverID, event.FinalStatus)
	return nil
}

// Close stops the cached topic (flushing pending publishes) and closes the client.
func (n *PubSubNotifier) Close() error {
	if n.topic != nil {
		n.topic.Stop()
	}
	return n.client.Close()
}

// ensurePushSubscription creates a push subscription for the shared topic →
// ConsumerPushURL if one does not already exist.
// Subscription ID: "<topicID>-consumer-push".
func (n *PubSubNotifier) ensurePushSubscription(ctx context.Context, t *pubsub.Topic) error {
	subID := n.topicID + "-consumer-push"
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
	log.Printf("Created push subscription %s → %s", subID, n.ConsumerPushURL)
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
