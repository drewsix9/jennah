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
