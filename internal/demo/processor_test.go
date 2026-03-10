package demo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcess_ResolvesLocalFileSizeWhenUnset(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.txt")
	content := strings.Repeat("hello world\n", 64)

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	cfg := &DistributedConfig{
		InstanceID:     0,
		TotalInstances: 4,
		InputDataPath:  inputFile,
		// Critical regression case: size is unset, must be resolved automatically.
		InputDataSize:  0,
		OutputBasePath: tmpDir,
		JobID:          "test-job",
	}

	metrics, err := NewProcessor(cfg).Process(context.Background())
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	if cfg.InputDataSize == 0 {
		t.Fatalf("expected InputDataSize to be resolved from local file")
	}
	if metrics.EndByte < metrics.StartByte {
		t.Fatalf("expected non-empty byte range, got start=%d end=%d", metrics.StartByte, metrics.EndByte)
	}
	if metrics.BytesProcessed == 0 {
		t.Fatalf("expected instance to process bytes, got 0")
	}
}

func TestProcess_RecordMode_DoesNotSplitSingleJSONRecord(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.json")
	content := `{"amountMax":139.99,"merchant":"Amazon.com","shipping":"FREE Shipping."}`

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	// Single record with 4 instances:
	// instance 0 should get the full record, others get empty work.
	cfg0 := &DistributedConfig{
		InstanceID:        0,
		TotalInstances:    4,
		InputDataPath:     inputFile,
		OutputBasePath:    tmpDir,
		DistributionMode:  DistributionModeRecord,
		EnableDistributed: true,
	}
	m0, err := NewProcessor(cfg0).Process(context.Background())
	if err != nil {
		t.Fatalf("instance 0 Process() error: %v", err)
	}
	if m0.RecordsProcessed != 1 {
		t.Fatalf("instance 0 expected 1 record, got %d", m0.RecordsProcessed)
	}
	if m0.BytesProcessed == 0 {
		t.Fatalf("instance 0 expected bytes > 0, got 0")
	}

	cfg1 := &DistributedConfig{
		InstanceID:        1,
		TotalInstances:    4,
		InputDataPath:     inputFile,
		OutputBasePath:    tmpDir,
		DistributionMode:  DistributionModeRecord,
		EnableDistributed: true,
	}
	m1, err := NewProcessor(cfg1).Process(context.Background())
	if err != nil {
		t.Fatalf("instance 1 Process() error: %v", err)
	}
	if m1.RecordsProcessed != 0 {
		t.Fatalf("instance 1 expected 0 records, got %d", m1.RecordsProcessed)
	}
}

func TestProcess_ByteRange_EmitsSentimentMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "reviews.txt")
	content := strings.Join([]string{
		"amazing fast delivery",
		"awful packaging and terrible support",
		"works as expected",
	}, "\n") + "\n"

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	cfg := &DistributedConfig{
		InstanceID:       0,
		TotalInstances:   1,
		InputDataPath:    inputFile,
		InputDataSize:    0,
		OutputBasePath:   tmpDir,
		DistributionMode: DistributionModeByteRange,
	}

	metrics, err := NewProcessor(cfg).Process(context.Background())
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	if metrics.RecordsProcessed != 3 {
		t.Fatalf("expected 3 processed records, got %d", metrics.RecordsProcessed)
	}
	if metrics.Sentiment == nil {
		t.Fatal("expected sentiment summary for byte-range processing")
	}
	if metrics.Sentiment.TotalRecords() != metrics.RecordsProcessed {
		t.Fatalf("sentiment total records: got %d, want %d", metrics.Sentiment.TotalRecords(), metrics.RecordsProcessed)
	}
}
