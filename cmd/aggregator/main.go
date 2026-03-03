package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alphauslabs/jennah/internal/demo"
)

func main() {
	// Parse command-line flags
	metricsPath := flag.String("metrics-path", "", "Path to metrics directory (local or GCS: gs://bucket/path)")
	baselineSeconds := flag.Float64("baseline-seconds", 0, "Single-instance baseline time for speedup calculation")
	outputFormat := flag.String("format", "detailed", "Output format: detailed or summary")
	flag.Parse()

	if *metricsPath == "" {
		fmt.Fprintf(os.Stderr, "ERROR: --metrics-path is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load all instance metrics
	instanceMetrics, err := loadInstanceMetrics(ctx, *metricsPath)
	if err != nil {
		log.Fatalf("Failed to load metrics: %v", err)
	}

	if len(instanceMetrics) == 0 {
		log.Fatalf("No instance metrics found in %s", *metricsPath)
	}

	// Sort by instance ID for consistent output
	sort.Slice(instanceMetrics, func(i, j int) bool {
		return instanceMetrics[i].InstanceID < instanceMetrics[j].InstanceID
	})

	// Create aggregated metrics
	instances := make([]demo.ProcessMetrics, len(instanceMetrics))
	for i, m := range instanceMetrics {
		instances[i] = *m
	}
	agg := &demo.AggregatedMetrics{
		Instances: instances,
	}
	agg.Calculate()

	// Add speedup calculation if baseline provided
	if *baselineSeconds > 0 && agg.MaxProcessingTime > 0 {
		agg.Speedup = *baselineSeconds / agg.MaxProcessingTime
		agg.Efficiency = agg.Speedup / float64(len(agg.Instances))
	}

	agg.TotalInstances = len(agg.Instances)
	agg.Timestamp = time.Now().Format(time.RFC3339)

	// Output results
	outputResults(agg, *outputFormat)
}

// loadInstanceMetrics loads all instance-*.json files from directory
func loadInstanceMetrics(ctx context.Context, path string) ([]*demo.ProcessMetrics, error) {
	// Determine if GCS or local path
	if strings.HasPrefix(path, "gs://") {
		return loadFromGCS(ctx, path)
	}
	return loadFromLocal(ctx, path)
}

// loadFromLocal loads metrics from local filesystem
func loadFromLocal(ctx context.Context, dirPath string) ([]*demo.ProcessMetrics, error) {
	var metrics []*demo.ProcessMetrics

	// List all files matching instance-*.json
	pattern := filepath.Join(dirPath, "instance-*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern %s: %w", pattern, err)
	}

	for _, filePath := range files {
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to read %s: %v\n", filePath, err)
			continue
		}

		var m demo.ProcessMetrics
		if err := json.Unmarshal(data, &m); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to parse %s: %v\n", filePath, err)
			continue
		}

		metrics = append(metrics, &m)
	}

	return metrics, nil
}

// loadFromGCS loads metrics from GCS bucket
func loadFromGCS(ctx context.Context, basePath string) ([]*demo.ProcessMetrics, error) {
	var resultMetrics []*demo.ProcessMetrics

	// Parse bucket from basePath
	bucket, _, err := demo.ParseGCSPath(basePath)
	if err != nil {
		return nil, fmt.Errorf("parse GCS path: %w", err)
	}

	// List all objects under path
	objects, err := demo.ListGCSObjects(ctx, basePath)
	if err != nil {
		return nil, fmt.Errorf("list GCS objects: %w", err)
	}

	// Filter for instance-*.json files
	for _, objName := range objects {
		if !strings.HasSuffix(objName, ".json") || !strings.Contains(objName, "instance-") {
			continue
		}

		// Construct full GCS path
		gcspath := fmt.Sprintf("gs://%s/%s", bucket, objName)

		// Download and parse file
		data, err := demo.ReadGCSFile(ctx, gcspath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to read %s: %v\n", gcspath, err)
			continue
		}

		var m demo.ProcessMetrics
		if err := json.Unmarshal(data, &m); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to parse %s: %v\n", gcspath, err)
			continue
		}

		resultMetrics = append(resultMetrics, &m)
	}

	return resultMetrics, nil
}

// outputResults formats and prints aggregated metrics
func outputResults(agg *demo.AggregatedMetrics, format string) {
	if format == "summary" {
		outputSummary(agg)
	} else {
		outputDetailed(agg)
	}
}

// outputDetailed prints detailed metrics for all instances
func outputDetailed(agg *demo.AggregatedMetrics) {
	fmt.Println("=== AGGREGATED METRICS ===")
	fmt.Printf("\nInstances processed: %d\n", len(agg.Instances))
	fmt.Printf("\n--- Totals ---\n")
	fmt.Printf("Total lines:      %d\n", agg.TotalLines)
	fmt.Printf("Total words:      %d\n", agg.TotalWords)
	fmt.Printf("Total characters: %d\n", agg.TotalCharacters)
	fmt.Printf("Total bytes:      %d\n", agg.TotalBytesProcessed)

	fmt.Printf("\n--- Timing ---\n")
	fmt.Printf("Min processing time:  %.3f seconds\n", agg.MinProcessingTime)
	fmt.Printf("Max processing time:  %.3f seconds\n", agg.MaxProcessingTime)
	fmt.Printf("Avg processing time:  %.3f seconds\n", agg.AvgProcessingTime)

	if agg.Speedup > 0 {
		fmt.Printf("\n--- Performance ---\n")
		fmt.Printf("Speedup:   %.2fx\n", agg.Speedup)
		fmt.Printf("Efficiency: %.1f%%\n", agg.Efficiency*100)
	}

	// Per-instance breakdown
	fmt.Printf("\n--- Instance Breakdown ---\n")
	for i := range agg.Instances {
		m := &agg.Instances[i]
		fmt.Printf("\nInstance %d:\n", m.InstanceID)
		fmt.Printf("  Lines:            %d\n", m.LinesCount)
		fmt.Printf("  Words:            %d\n", m.WordsCount)
		fmt.Printf("  Characters:       %d\n", m.CharactersCount)
		fmt.Printf("  Bytes processed:  %d (%.3f MB)\n", m.BytesProcessed, float64(m.BytesProcessed)/1024/1024)
		fmt.Printf("  Processing time:  %.3f seconds\n", m.ProcessingTimeSeconds)
		fmt.Printf("  Throughput:       %.2f MB/s\n", m.ThroughputMBPerSecond)
	}

	// JSON output for logging/analysis
	fmt.Printf("\n--- JSON Output ---\n")
	jsonData, _ := json.MarshalIndent(agg, "", "  ")
	fmt.Println(string(jsonData))
}

// outputSummary prints a concise summary
func outputSummary(agg *demo.AggregatedMetrics) {
	fmt.Printf("Instances: %d\n", len(agg.Instances))
	fmt.Printf("Total lines: %d\n", agg.TotalLines)
	fmt.Printf("Total bytes: %d (%.3f MB)\n", agg.TotalBytesProcessed, float64(agg.TotalBytesProcessed)/1024/1024)
	fmt.Printf("Processing time: %.3f seconds (avg: %.3f)\n", agg.MaxProcessingTime, agg.AvgProcessingTime)
	if agg.Speedup > 0 {
		fmt.Printf("Speedup: %.2fx | Efficiency: %.1f%%\n", agg.Speedup, agg.Efficiency*100)
	}
}
