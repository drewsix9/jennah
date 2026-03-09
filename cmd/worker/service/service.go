package service

import (
	"context"
	"log"
	"sync"
	"time"

	gcpbatch "cloud.google.com/go/batch/apiv1"

	"github.com/alphauslabs/jennah/gen/proto/jennahv1connect"
	batch "github.com/alphauslabs/jennah/internal/cloudexec"
	"github.com/alphauslabs/jennah/internal/config"
	"github.com/alphauslabs/jennah/internal/database"
	"github.com/alphauslabs/jennah/internal/dispatcher"
	"github.com/alphauslabs/jennah/internal/notifier"
)

// WorkerService implements the DeploymentService RPC handlers for the worker.
type WorkerService struct {
	jennahv1connect.UnimplementedDeploymentServiceHandler
	dbClient       *database.Client
	batchProvider  batch.Provider
	dispatcher     *dispatcher.Dispatcher
	jobConfig      *config.JobConfigFile
	workerID       string
	leaseTTL       time.Duration
	claimInterval  time.Duration
	pollers        map[string]*JobPoller // Key: "tenantID/jobID"
	pollersMutex   sync.Mutex
	gcpBatchClient *gcpbatch.Client
	notifier       notifier.Notifier
}

// NewWorkerService creates a new WorkerService with the given dependencies.
func NewWorkerService(
	dbClient *database.Client,
	batchProvider batch.Provider,
	d *dispatcher.Dispatcher,
	jobConfig *config.JobConfigFile,
	gcpBatchClient *gcpbatch.Client,
	workerID string,
	leaseTTL time.Duration,
	claimInterval time.Duration,
	n notifier.Notifier,
) *WorkerService {
	return &WorkerService{
		dbClient:       dbClient,
		batchProvider:  batchProvider,
		dispatcher:     d,
		jobConfig:      jobConfig,
		workerID:       workerID,
		leaseTTL:       leaseTTL,
		claimInterval:  claimInterval,
		pollers:        make(map[string]*JobPoller),
		gcpBatchClient: gcpBatchClient,
		notifier:       n,
	}
}

// publishTerminalEvent enriches the event with tenant metadata and publishes it.
// Publish failures are logged but do not propagate — database state remains the source of truth.
func (s *WorkerService) publishTerminalEvent(ctx context.Context, event notifier.JobTerminalEvent, tenantID string) {
	// Enrich event with submitter email from tenant record.
	tenant, err := s.dbClient.GetTenant(ctx, tenantID)
	if err != nil {
		log.Printf("Warning: could not look up tenant %s for event enrichment: %v", tenantID, err)
	} else {
		event.UserEmail = tenant.UserEmail
	}

	if err := s.notifier.PublishJobTerminalEvent(ctx, event); err != nil {
		log.Printf("Error publishing terminal event for job %s: %v", event.JobID, err)
	}
}
