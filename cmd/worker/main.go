package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	batch "cloud.google.com/go/batch/apiv1"
	"cloud.google.com/go/batch/apiv1/batchpb"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	jennahv1 "github.com/alphauslabs/jennah/gen/proto"
	"github.com/alphauslabs/jennah/gen/proto/jennahv1connect"
	"github.com/alphauslabs/jennah/internal/database"
)

const (
	projectID       = "labs-169405"
	region          = "asia-northeast1"
	spannerInstance = "alphaus-dev"
	spannerDB       = "main"
	workerPort      = "8081"
)

type WorkerServer struct {
	dbClient    *database.Client
	batchClient *batch.Client
	projectID   string
	region      string
}

func (s *WorkerServer) SubmitJob(
	ctx context.Context,
	req *connect.Request[jennahv1.SubmitJobRequest],
) (*connect.Response[jennahv1.SubmitJobResponse], error) {
	log.Printf("Received SubmitJob request for tenant: %s", req.Msg.TenantId)

	if req.Msg.TenantId == "" {
		log.Printf("Error: tenant_id is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id is required"))
	}

	if req.Msg.ImageUri == "" {
		log.Printf("Error: image_uri is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image_uri is required"))
	}

	jobId := uuid.New().String()
	log.Printf("Generated job ID: %s", jobId)

	err := s.dbClient.InsertJob(ctx, req.Msg.TenantId, jobId, req.Msg.ImageUri, []string{})
	if err != nil {
		log.Printf("Error inserting job to database: %v", err)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to create job record: %w", err),
		)
	}
	log.Printf("Job %s saved to database with PENDING status", jobId)

	batchJob, err := s.createGCPBatchJob(ctx, jobId, req.Msg.ImageUri, req.Msg.EnvVars)
	if err != nil {
		log.Printf("Error creating GCP Batch job: %v", err)
		failErr := s.dbClient.FailJob(ctx, req.Msg.TenantId, jobId, err.Error())
		if failErr != nil {
			log.Printf("Error updating job status to FAILED: %v", failErr)
		}
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to create GCP Batch job: %w", err),
		)
	}
	log.Printf("GCP Batch job created: %s", batchJob.Name)

	err = s.dbClient.UpdateJobStatus(ctx, req.Msg.TenantId, jobId, database.JobStatusRunning)
	if err != nil {
		log.Printf("Error updating job status to RUNNING: %v", err)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to update job status: %w", err),
		)
	}
	log.Printf("Job %s status updated to RUNNING", jobId)

	response := connect.NewResponse(&jennahv1.SubmitJobResponse{
		JobId:  jobId,
		Status: database.JobStatusRunning,
	})

	log.Printf("Successfully submitted job %s for tenant %s", jobId, req.Msg.TenantId)
	return response, nil
}

func (s *WorkerServer) ListJobs(
	ctx context.Context,
	req *connect.Request[jennahv1.ListJobsRequest],
) (*connect.Response[jennahv1.ListJobsResponse], error) {
	log.Printf("Received ListJobs request for tenant: %s", req.Msg.TenantId)
	if req.Msg.TenantId == "" {
		log.Printf("Error: tenant_id is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id is required"))
	}

	jobs, err := s.dbClient.ListJobs(ctx, req.Msg.TenantId)
	if err != nil {
		log.Printf("Error listing jobs from database: %v", err)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to list jobs: %w", err),
		)
	}
	log.Printf("Retrieved %d jobs for tenant %s", len(jobs), req.Msg.TenantId)

	protoJobs := make([]*jennahv1.Job, 0, len(jobs))
	for _, job := range jobs {
		protoJob := &jennahv1.Job{
			JobId:     job.JobId,
			TenantId:  job.TenantId,
			ImageUri:  job.ImageUri,
			Status:    job.Status,
			CreatedAt: job.CreatedAt.Format(time.RFC3339),
		}
		protoJobs = append(protoJobs, protoJob)
	}

	response := connect.NewResponse(&jennahv1.ListJobsResponse{
		Jobs: protoJobs,
	})

	log.Printf("Successfully listed %d jobs for tenant %s", len(protoJobs), req.Msg.TenantId)
	return response, nil
}

func (s *WorkerServer) createGCPBatchJob(
	ctx context.Context,
	jobID string,
	imageURI string,
	envVars map[string]string,
) (*batchpb.Job, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", s.projectID, s.region)

	runnable := &batchpb.Runnable{
		Executable: &batchpb.Runnable_Container_{
			Container: &batchpb.Runnable_Container{
				ImageUri: imageURI,
			},
		},
	}

	if len(envVars) > 0 {
		runnable.Environment = &batchpb.Environment{
			Variables: envVars,
		}
	}

	job := &batchpb.Job{
		TaskGroups: []*batchpb.TaskGroup{
			{
				TaskSpec: &batchpb.TaskSpec{
					Runnables: []*batchpb.Runnable{runnable},
				},
				TaskCount: 1,
			},
		},
	}

	req := &batchpb.CreateJobRequest{
		Parent: parent,
		JobId:  jobID,
		Job:    job,
	}

	return s.batchClient.CreateJob(ctx, req)
}

func main() {
	log.Println("Starting worker...")

	ctx := context.Background()

	dbClient, err := database.NewClient(ctx, projectID, spannerInstance, spannerDB)
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}
	defer dbClient.Close()
	log.Printf("Connected to Spanner: %s/%s/%s", projectID, spannerInstance, spannerDB)

	batchClient, err := batch.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCP Batch client: %v", err)
	}
	defer batchClient.Close()
	log.Printf("Connected to GCP Batch API in region: %s", region)

	workerServer := &WorkerServer{
		dbClient:    dbClient,
		batchClient: batchClient,
		projectID:   projectID,
		region:      region,
	}

	mux := http.NewServeMux()
	path, handler := jennahv1connect.NewDeploymentServiceHandler(workerServer)
	mux.Handle(path, handler)
	log.Printf("ConnectRPC handler registered at path: %s", path)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Println("Health check endpoint: /health")

	addr := fmt.Sprintf("0.0.0.0:%s", workerPort)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Worker listening on %s", addr)
		log.Println("Available endpoints:")
		log.Printf("  • POST %sSubmitJob", path)
		log.Printf("  • POST %sListJobs", path)
		log.Printf("  • GET  /health")
		log.Printf("Worker configured for project: %s, region: %s", projectID, region)
		log.Println("")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-sigCtx.Done()
	log.Println("Shutdown signal received, gracefully shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Worker stopped")
}
