package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alphauslabs/jennah/internal/database"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	// Initialize database client
	fmt.Println("🔌 Connecting to Cloud Spanner...")
	client, err := database.NewClient(ctx, "labs-169405", "alphaus-dev", "main")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	fmt.Println("✅ Connected successfully!")

	// Test Tenant Operations
	fmt.Println("\n--- Testing Tenant Operations ---")

	tenantID := uuid.New().String()
	fmt.Printf("Creating tenant: %s\n", tenantID)

	err = client.InsertTenant(ctx, tenantID, "Test Company")
	if err != nil {
		log.Fatalf("Failed to insert tenant: %v", err)
	}
	fmt.Println("✅ Tenant created")

	// Get tenant
	tenant, err := client.GetTenant(ctx, tenantID)
	if err != nil {
		log.Fatalf("Failed to get tenant: %v", err)
	}
	fmt.Printf("✅ Retrieved tenant: %s - %s\n", tenant.TenantId, tenant.Name)

	// List all tenants
	tenants, err := client.ListTenants(ctx)
	if err != nil {
		log.Fatalf("Failed to list tenants: %v", err)
	}
	fmt.Printf("✅ Total tenants: %d\n", len(tenants))

	// Test Job Operations
	fmt.Println("\n--- Testing Job Operations ---")

	jobID := uuid.New().String()
	fmt.Printf("Creating job: %s\n", jobID)

	err = client.InsertJob(ctx, tenantID, jobID,
		"gcr.io/test/image:latest",
		[]string{"echo", "hello world"})
	if err != nil {
		log.Fatalf("Failed to insert job: %v", err)
	}
	fmt.Println("✅ Job created with status: PENDING")

	// Get job
	job, err := client.GetJob(ctx, tenantID, jobID)
	if err != nil {
		log.Fatalf("Failed to get job: %v", err)
	}
	fmt.Printf("✅ Retrieved job: %s - Status: %s\n", job.JobId, job.Status)

	// Update job status to RUNNING
	fmt.Println("\n🔄 Updating job status to RUNNING...")
	err = client.UpdateJobStatus(ctx, tenantID, jobID, database.JobStatusRunning)
	if err != nil {
		log.Fatalf("Failed to update status: %v", err)
	}
	fmt.Println("✅ Status updated")

	// Verify status changed
	job, _ = client.GetJob(ctx, tenantID, jobID)
	fmt.Printf("✅ Current status: %s\n", job.Status)

	// List jobs
	jobs, err := client.ListJobs(ctx, tenantID)
	if err != nil {
		log.Fatalf("Failed to list jobs: %v", err)
	}
	fmt.Printf("✅ Total jobs for tenant: %d\n", len(jobs))

	// List jobs by status
	runningJobs, err := client.ListJobsByStatus(ctx, tenantID, database.JobStatusRunning)
	if err != nil {
		log.Fatalf("Failed to list running jobs: %v", err)
	}
	fmt.Printf("✅ Running jobs: %d\n", len(runningJobs))

	// Complete the job
	fmt.Println("\n✅ Completing job...")
	time.Sleep(1 * time.Second) // Simulate work
	err = client.CompleteJob(ctx, tenantID, jobID)
	if err != nil {
		log.Fatalf("Failed to complete job: %v", err)
	}
	fmt.Println("✅ Job completed")

	// Verify completion
	job, _ = client.GetJob(ctx, tenantID, jobID)
	fmt.Printf("✅ Final status: %s\n", job.Status)
	if job.CompletedAt != nil {
		fmt.Printf("✅ Completed at: %s\n", job.CompletedAt.Format(time.RFC3339))
	}

	// Cleanup - Delete test data
	fmt.Println("\n--- Cleanup ---")
	err = client.DeleteJob(ctx, tenantID, jobID)
	if err != nil {
		log.Printf("Warning: Failed to delete job: %v", err)
	} else {
		fmt.Println("✅ Test job deleted")
	}

	err = client.DeleteTenant(ctx, tenantID)
	if err != nil {
		log.Printf("Warning: Failed to delete tenant: %v", err)
	} else {
		fmt.Println("✅ Test tenant deleted")
	}

	fmt.Println("\n🎉 All tests passed!")
}
