package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alphauslabs/jennah/internal/demo"
)

// TestLoadFromLocal_Success tests loading metrics from local directory
func TestLoadFromLocal_Success(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create sample metrics files
	metrics := []*demo.ProcessMetrics{
		{
			InstanceID:             0,
			BytesProcessed:         1000,
			LinesCount:             100,
			WordsCount:             500,
			CharactersCount:        1000,
			ProcessingTimeSeconds:  1.5,
			ThroughputMBPerSecond:  0.67,
			Timestamp:              time.Now().Format(time.RFC3339),
		},
		{
			InstanceID:             1,
			BytesProcessed:         1000,
			LinesCount:             100,
			WordsCount:             500,
			CharactersCount:        1000,
			ProcessingTimeSeconds:  1.6,
			ThroughputMBPerSecond:  0.625,
			Timestamp:              time.Now().Format(time.RFC3339),
		},
	}

	// Write metrics to files
	for _, m := range metrics {
		data, _ := json.Marshal(m)
		filename := filepath.Join(tmpDir, "instance-"+string(rune('0'+m.InstanceID))+".json")
		os.WriteFile(filename, data, 0644)
	}

	// Load metrics
	ctx := context.Background()
	loaded, err := loadFromLocal(ctx, tmpDir)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Expected 2 metrics, got %d", len(loaded))
	}

	// Verify metrics
	if loaded[0].InstanceID != 0 || loaded[1].InstanceID != 1 {
		t.Errorf("Instance IDs not in order: %d, %d", loaded[0].InstanceID, loaded[1].InstanceID)
	}
}

// TestLoadFromLocal_NoFiles tests loading from empty directory
func TestLoadFromLocal_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	loaded, err := loadFromLocal(ctx, tmpDir)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(loaded) != 0 {
		t.Fatalf("Expected 0 metrics, got %d", len(loaded))
	}
}

// TestLoadFromLocal_InvalidJSON tests handling of malformed JSON
func TestLoadFromLocal_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with invalid JSON
	filename := filepath.Join(tmpDir, "instance-0.json")
	os.WriteFile(filename, []byte("{invalid json}"), 0644)

	// Create one valid file
	validMetrics := &demo.ProcessMetrics{
		InstanceID:    1,
		BytesProcessed: 1000,
	}
	data, _ := json.Marshal(validMetrics)
	validFilename := filepath.Join(tmpDir, "instance-1.json")
	os.WriteFile(validFilename, data, 0644)

	ctx := context.Background()
	loaded, err := loadFromLocal(ctx, tmpDir)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should skip invalid file and load valid one
	if len(loaded) != 1 {
		t.Fatalf("Expected 1 valid metric (skipping invalid), got %d", len(loaded))
	}
}

// TestAggregateMetrics tests the aggregation calculation
func TestAggregateMetrics(t *testing.T) {
	metrics := []*demo.ProcessMetrics{
		{
			InstanceID:             0,
			BytesProcessed:         1000,
			LinesCount:             100,
			WordsCount:             500,
			CharactersCount:        1000,
			ProcessingTimeSeconds:  2.0,
			ThroughputMBPerSecond:  0.5,
		},
		{
			InstanceID:             1,
			BytesProcessed:         2000,
			LinesCount:             200,
			WordsCount:             1000,
			CharactersCount:        2000,
			ProcessingTimeSeconds:  3.0,
			ThroughputMBPerSecond:  0.67,
		},
		{
			InstanceID:             2,
			BytesProcessed:         1500,
			LinesCount:             150,
			WordsCount:             750,
			CharactersCount:        1500,
			ProcessingTimeSeconds:  2.5,
			ThroughputMBPerSecond:  0.6,
		},
	}

	agg := &demo.AggregatedMetrics{
		Instances: []demo.ProcessMetrics{
			*metrics[0],
			*metrics[1],
			*metrics[2],
		},
	}
	agg.Calculate()

	// Verify totals
	if agg.TotalLines != 450 {
		t.Errorf("Expected 450 lines, got %d", agg.TotalLines)
	}
	if agg.TotalWords != 2250 {
		t.Errorf("Expected 2250 words, got %d", agg.TotalWords)
	}
	if agg.TotalBytesProcessed != 4500 {
		t.Errorf("Expected 4500 bytes, got %d", agg.TotalBytesProcessed)
	}
	if agg.TotalCharacters != 4500 {
		t.Errorf("Expected 4500 characters, got %d", agg.TotalCharacters)
	}

	// Verify timing
	if agg.MinProcessingTime != 2.0 {
		t.Errorf("Expected min time 2.0, got %v", agg.MinProcessingTime)
	}
	if agg.MaxProcessingTime != 3.0 {
		t.Errorf("Expected max time 3.0, got %v", agg.MaxProcessingTime)
	}
	expectedAvg := (2.0 + 3.0 + 2.5) / 3.0
	if agg.AvgProcessingTime != expectedAvg {
		t.Errorf("Expected avg time %.3f, got %.3f", expectedAvg, agg.AvgProcessingTime)
	}
}

// TestSpeedupCalculation tests speedup and efficiency calculation
func TestSpeedupCalculation(t *testing.T) {
	instances := []demo.ProcessMetrics{
		{ProcessingTimeSeconds: 2.5},
		{ProcessingTimeSeconds: 2.4},
		{ProcessingTimeSeconds: 2.3},
		{ProcessingTimeSeconds: 2.4},
	}
	metrics := &demo.AggregatedMetrics{
		Instances: instances,
	}
	metrics.Calculate()

	// Add speedup calculation
	baselineTime := 10.0
	metrics.Speedup = baselineTime / metrics.MaxProcessingTime
	metrics.Efficiency = metrics.Speedup / float64(len(metrics.Instances))

	// Expected: speedup = 10.0 / 2.5 = 4.0x, efficiency = 4.0 / 4 = 100%
	if metrics.Speedup != 4.0 {
		t.Errorf("Expected speedup 4.0, got %.2f", metrics.Speedup)
	}
	if metrics.Efficiency != 1.0 {
		t.Errorf("Expected efficiency 1.0, got %.3f", metrics.Efficiency)
	}
}

// TestMetricsJSONMarshal tests JSON serialization
func TestMetricsJSONMarshal(t *testing.T) {
	instances := []demo.ProcessMetrics{
		{
			InstanceID:            0,
			BytesProcessed:        1000,
			ProcessingTimeSeconds: 2.0,
		},
	}
	metrics := &demo.AggregatedMetrics{
		Instances: instances,
	}
	metrics.Calculate()

	// Should be JSON serializable (no error from encoding/json)
	_, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Failed to marshal metrics to JSON: %v", err)
	}
}
