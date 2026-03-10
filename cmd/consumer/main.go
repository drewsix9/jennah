package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alphauslabs/jennah/internal/database"
	"github.com/alphauslabs/jennah/internal/notifier"
	"github.com/google/uuid"
)

// pushMessage is the envelope Pub/Sub delivers to push endpoints.
type pushMessage struct {
	Message struct {
		Data       string            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Database setup ---
	dbProject := os.Getenv("DB_PROJECT_ID")
	dbInstance := os.Getenv("DB_INSTANCE")
	dbDatabase := os.Getenv("DB_DATABASE")
	if dbProject == "" || dbInstance == "" || dbDatabase == "" {
		log.Fatal("DB_PROJECT_ID, DB_INSTANCE, and DB_DATABASE must be set")
	}

	dbClient, err := database.NewClient(ctx, dbProject, dbInstance, dbDatabase)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()
	log.Printf("Connected to Spanner: projects/%s/instances/%s/databases/%s", dbProject, dbInstance, dbDatabase)

	// --- HTTP handler ---
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/pubsub/push", makePushHandler(dbClient))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("Consumer service listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down consumer service...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// makePushHandler returns the HTTP handler that processes Pub/Sub push deliveries.
func makePushHandler(db *database.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg pushMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			log.Printf("Failed to decode push message: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Pub/Sub data is base64-encoded.
		rawData, err := base64.StdEncoding.DecodeString(msg.Message.Data)
		if err != nil {
			log.Printf("Failed to base64-decode message data: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var event notifier.JobTerminalEvent
		if err := json.Unmarshal(rawData, &event); err != nil {
			log.Printf("Failed to unmarshal JobTerminalEvent: %v", err)
			// Ack anyway — malformed messages would loop forever.
			w.WriteHeader(http.StatusOK)
			return
		}

		if err := saveNotification(r.Context(), db, msg.Message.MessageID, event); err != nil {
			log.Printf("Failed to save notification for job %s: %v", event.JobID, err)
			// Return 500 so Pub/Sub retries delivery.
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("Saved notification for job %s tenant %s status %s", event.JobID, event.TenantID, event.FinalStatus)
		w.WriteHeader(http.StatusOK)
	}
}

func saveNotification(ctx context.Context, db *database.Client, messageID string, event notifier.JobTerminalEvent) error {
	// Use EventID from the payload for deduplication. Fall back to messageID.
	notifID := event.EventID
	if notifID == "" {
		notifID = messageID
	}
	if notifID == "" {
		notifID = uuid.NewString()
	}

	occurredAt, err := time.Parse(time.RFC3339, event.OccurredAt)
	if err != nil {
		occurredAt = time.Now()
	}

	n := &database.Notification{
		TenantId:       event.TenantID,
		NotificationId: notifID,
		JobId:          event.JobID,
		FinalStatus:    event.FinalStatus,
		OccurredAt:     occurredAt,
	}
	if event.JobName != "" {
		n.JobName = &event.JobName
	}
	if event.ServiceTier != "" {
		n.ServiceTier = &event.ServiceTier
	}
	if event.AssignedService != "" {
		n.AssignedService = &event.AssignedService
	}
	if event.ErrorMessage != "" {
		n.ErrorMessage = &event.ErrorMessage
	}

	return db.InsertNotification(ctx, n)
}
