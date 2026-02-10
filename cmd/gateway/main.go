package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	jennahv1 "github.com/alphauslabs/jennah/gen/proto"
	"github.com/alphauslabs/jennah/gen/proto/jennahv1connect"
	"github.com/alphauslabs/jennah/internal/hashing"
)

type GatewayServer struct {
	router        *hashing.Router
	workerClients map[string]jennahv1connect.DeploymentServiceClient
}

func (s *GatewayServer) SubmitJob(
	ctx context.Context,
	req *connect.Request[jennahv1.SubmitJobRequest],
) (*connect.Response[jennahv1.SubmitJobResponse], error) {
	log.Printf("recieved jog")
	if req.Msg.TenantId == "" {
		log.Printf("tenant id is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id is required"))
	}

	if req.Msg.ImageUri == "" {

		log.Printf("image uri is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image_uri is required"))
	}
	log.Printf("request is valid")

	workerIP := s.router.GetWorkerIP(req.Msg.TenantId)

	if workerIP == "" {
		log.Printf("no worker found for client: %s", workerIP)
		return nil, connect.NewError(connect.CodeInternal, errors.New("no worker found for tenant_id"))
	}
	log.Printf("selected worker: %s for tenant: %s", workerIP, req.Msg.TenantId)

	workerClient, exists := s.workerClients[workerIP]
	if !exists {
		log.Printf("no worker found for tenant id: %s", req.Msg.TenantId)
		return nil, connect.NewError(connect.CodeInternal, errors.New("no worker client found for tenant_id"))
	}
	log.Printf("forwarding request to worker: %s", workerIP)

	response, err := workerClient.SubmitJob(ctx, req)
	if err != nil {
		log.Printf("ERROR: Worker %s failed: %v", workerIP, err)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("worker failed to process job: %w", err),
		)
	}
	response.Msg.WorkerAssigned = workerIP
	log.Printf("submitted successfully: job_id=%s, worker=%s, status=%s",
		response.Msg.JobId, workerIP, response.Msg.Status)
	return response, nil
	//mock data
	//response := connect.NewResponse(&jennahv1.SubmitJobResponse{
	//	JobId:          "mock job",
	//	Status:         "mock status",
	//	WorkerAssigned: workerIP,
	//})
	//return response, nil
}

func (s *GatewayServer) ListJobs(
	ctx context.Context,
	req *connect.Request[jennahv1.ListJobsRequest],
) (*connect.Response[jennahv1.ListJobsResponse], error) {
	log.Printf("recieved list jobs request")
	if req.Msg.TenantId == "" {
		log.Printf("tenant id is empty")
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id is required"))
	}
	log.Printf("request is valid")

	workerIP := s.router.GetWorkerIP(req.Msg.TenantId)
	if workerIP == "" {
		log.Printf("no worker found for tenant id: %s", req.Msg.TenantId)
		return nil, connect.NewError(connect.CodeInternal, errors.New("no worker found for tenant_id"))
	}
	log.Printf("selected worker: %s for tenant: %s", workerIP, req.Msg.TenantId)

	workerClient, exists := s.workerClients[workerIP]
	if !exists {
		log.Printf("ERROR: No client found for worker: %s", workerIP)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("worker client not found for IP: %s", workerIP),
		)
	}

	log.Printf("forwarding request to worker: %s", workerIP)

	response, err := workerClient.ListJobs(ctx, req)
	if err != nil {
		log.Printf("ERROR: Worker %s failed: %v", workerIP, err)
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("worker failed to list jobs: %w", err),
		)
	}

	log.Printf("✓ Listed %d jobs for tenant %s from worker %s",
		len(response.Msg.Jobs), req.Msg.TenantId, workerIP)

	return response, nil
	//mock data
	//response := connect.NewResponse(&jennahv1.ListJobsResponse{
	//	Jobs: []*jennahv1.Job{
	//		{
	//			JobId:     "job-12345",
	//			TenantId:  req.Msg.TenantId,
	//			ImageUri:  "nginx:latest",
	//			Status:    "running",
	//			CreatedAt: "2025-02-10T10:30:00Z",
	//		},
	//		{
	//			JobId:     "job-67890",
	//			TenantId:  req.Msg.TenantId,
	//			ImageUri:  "redis:alpine",
	//			Status:    "completed",
	//			CreatedAt: "2025-02-10T09:15:00Z",
	//		},
	//	},
	//})
	//return response, nil
}

func main() {
	log.Println("Starting gateway...")
	//hardcoded for testing
	workerIPs := []string{
		"10.128.0.1",
		"10.128.0.2",
		"10.128.0.3",
	}
	log.Printf("Configured %d workers: %v", len(workerIPs), workerIPs)

	router := hashing.NewRouter(workerIPs)
	log.Printf("Consistent hashing ring initialized with %d workers", len(workerIPs))
	workerClients := make(map[string]jennahv1connect.DeploymentServiceClient)
	httpClient := &http.Client{}

	for _, ip := range workerIPs {
		// assuming workers listen on port 8081
		workerURL := fmt.Sprintf("http://%s:8081", ip)
		workerClients[ip] = jennahv1connect.NewDeploymentServiceClient(
			httpClient,
			workerURL,
		)
		log.Printf("Created client for worker: %s", workerURL)
	}
	gatewayServer := &GatewayServer{router: router, workerClients: workerClients}

	mux := http.NewServeMux()
	path, handler := jennahv1connect.NewDeploymentServiceHandler(gatewayServer)
	mux.Handle(path, handler)
	log.Printf("handler registered at path: %s", path)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Health check endpoint: /health")

	addr := "0.0.0.0:8080"

	log.Printf("Gateway listening on %s", addr)
	log.Println("Available endpoints:")
	log.Printf("  • POST %sSubmitJob", path)
	log.Printf("  • POST %sListJobs", path)
	log.Printf("  • GET  /health")

	log.Println("workers must be running on port 8081 for requests to succeed")
	log.Println("")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
