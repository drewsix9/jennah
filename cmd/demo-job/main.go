package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/alphauslabs/jennah/internal/demo"
)

func main() {
	// Parse command-line flags (for local testing)
	instanceID := flag.Int("instance-id", -1, "Override BATCH_TASK_INDEX")
	totalInstances := flag.Int("total-instances", -1, "Override BATCH_TASK_COUNT")
	flag.Parse()

	// Load configuration from environment
	cfg, err := demo.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override with CLI flags if provided
	if *instanceID >= 0 {
		cfg.InstanceID = *instanceID
	}
	if *totalInstances > 0 {
		cfg.TotalInstances = *totalInstances
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Log configuration
	log.Printf("Demo Job Started")
	log.Printf("  Instance ID: %d / %d", cfg.InstanceID, cfg.TotalInstances)
	log.Printf("  Input Path: %s", cfg.InputDataPath)
	log.Printf("  Input Size: %d bytes", cfg.InputDataSize)
	log.Printf("  Output Path: %s", cfg.OutputBasePath)
	log.Printf("  Distribution Mode: %s", cfg.DistributionMode)

	// Create processor
	processor := demo.NewProcessor(cfg)

	// Run with timeout (for safety)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	startTime := time.Now()

	// Process assigned byte range
	metrics, err := processor.Process(ctx)
	if err != nil {
		log.Fatalf("Processing failed: %v", err)
	}

	metrics.ProcessingTimeSeconds = time.Since(startTime).Seconds()
	metrics.CalculateThroughput()

	// Write metrics to local file or GCS
	if err := processor.WriteMetrics(ctx, metrics); err != nil {
		log.Fatalf("Failed to write metrics: %v", err)
	}

	log.Printf("Demo Job Completed")
	log.Printf("  Bytes Processed: %d", metrics.BytesProcessed)
	log.Printf("  Lines Counted: %d", metrics.LinesCount)
	log.Printf("  Time Elapsed: %.2f seconds", metrics.ProcessingTimeSeconds)
	log.Printf("  Throughput: %.2f MB/s", metrics.ThroughputMBPerSecond)
}
