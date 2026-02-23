package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alphauslabs/jennah/internal/database"
)

// resumeActiveJobPollers finds all non-terminal jobs across all tenants and restarts their pollers.
func resumeActiveJobPollers(ctx context.Context, server *WorkerServer, dbClient *database.Client) error {
	log.Println("Scanning for active jobs to resume polling...")

	// Get all tenants
	tenants, err := dbClient.ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	if len(tenants) == 0 {
		log.Println("No tenants found")
		return nil
	}

	log.Printf("Found %d tenant(s)", len(tenants))

	resumedCount := 0

	// For each tenant, find active jobs and resume their pollers
	for _, tenant := range tenants {
		jobs, err := dbClient.ListJobs(ctx, tenant.TenantId)
		if err != nil {
			log.Printf("Error listing jobs for tenant %s: %v", tenant.TenantId, err)
			continue
		}

		for _, job := range jobs {
			// Skip terminal statuses - no need to poll
			if isTerminalStatus(job.Status) {
				continue
			}

			// Skip jobs without GCP Batch reference
			if job.GcpBatchJobName == nil {
				log.Printf("Skipping poller for job %s: no GCP Batch resource", job.JobId)
				continue
			}

			log.Printf("Resuming poller for job %s (status: %s)", job.JobId, job.Status)
			server.startJobPoller(ctx, tenant.TenantId, job.JobId, *job.GcpBatchJobName, job.Status)
			resumedCount++
		}
	}

	log.Printf("Job poller resume complete: %d poller(s) started", resumedCount)
	return nil
}
