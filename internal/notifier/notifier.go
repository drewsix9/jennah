package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
)

// JobTerminalEvent is the payload published to Pub/Sub when a job reaches
// a terminal state (COMPLETED, FAILED, or CANCELLED).
type JobTerminalEvent struct {
	EventID            string  `json:"event_id"`
	EventType          string  `json:"event_type"`
	TenantID           string  `json:"tenant_id"`
	JobID              string  `json:"job_id"`
	FinalStatus        string  `json:"final_status"`
	PreviousStatus     string  `json:"previous_status"`
	OccurredAt         string  `json:"occurred_at"`
	UserEmail          string  `json:"user_email,omitempty"`
	ServiceTier        string  `json:"service_tier,omitempty"`
	AssignedService    string  `json:"assigned_service,omitempty"`
	CloudResourcePath  string  `json:"cloud_resource_path,omitempty"`
	ErrorMessage       string  `json:"error_message,omitempty"`
	JobName            string  `json:"job_name,omitempty"`
}

// Notifier publishes job terminal events. Implementations must be safe
// for concurrent use.
type Notifier interface {
	// PublishJobTerminalEvent publishes an event for a job that reached a terminal state.
	// Implementations should not block the caller on delivery confirmation unless necessary.
	PublishJobTerminalEvent(ctx context.Context, event JobTerminalEvent) error

	// Close releases any resources held by the notifier.
	Close() error
}

// PubSubNotifier publishes terminal events to a Google Cloud Pub/Sub topic.
type PubSubNotifier struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// NewPubSubNotifier creates a notifier that publishes to the given Pub/Sub topic.
// The caller must call Close when the notifier is no longer needed.
func NewPubSubNotifier(client *pubsub.Client, topicID string) *PubSubNotifier {
	topic := client.Topic(topicID)
	return &PubSubNotifier{
		client: client,
		topic:  topic,
	}
}

// PublishJobTerminalEvent publishes the event to Pub/Sub. It blocks until
// the publish is acknowledged or the context is cancelled.
func (n *PubSubNotifier) PublishJobTerminalEvent(ctx context.Context, event JobTerminalEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	result := n.topic.Publish(ctx, &pubsub.Message{
		Data: data,
		Attributes: map[string]string{
			"event_type": event.EventType,
			"tenant_id":  event.TenantID,
			"job_id":     event.JobID,
			"status":     event.FinalStatus,
		},
	})

	// Block until the publish result is available so we can log failures.
	serverID, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("publish event for job %s: %w", event.JobID, err)
	}
	log.Printf("Published terminal event for job %s (msg id: %s, status: %s)", event.JobID, serverID, event.FinalStatus)
	return nil
}

// Close stops the topic (flushing pending publishes) and closes the client.
func (n *PubSubNotifier) Close() error {
	n.topic.Stop()
	return n.client.Close()
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
