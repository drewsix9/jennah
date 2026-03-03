package demo

import (
	"fmt"
	"os"
	"strconv"
)

// DistributedConfig holds configuration for the demo job
type DistributedConfig struct {
	InstanceID        int
	TotalInstances    int
	InputDataPath     string
	InputDataSize     int64
	OutputBasePath    string
	JobID             string
	DistributionMode  string
	EnableDistributed bool
}

// LoadConfig reads environment variables and returns configuration
func LoadConfig() (*DistributedConfig, error) {
	cfg := &DistributedConfig{}

	// Load BATCH_TASK_INDEX (optional for local testing)
	if val := os.Getenv("BATCH_TASK_INDEX"); val != "" {
		idx, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BATCH_TASK_INDEX: %w", err)
		}
		cfg.InstanceID = idx
	}

	// Load BATCH_TASK_COUNT (optional for local testing)
	if val := os.Getenv("BATCH_TASK_COUNT"); val != "" {
		count, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BATCH_TASK_COUNT: %w", err)
		}
		cfg.TotalInstances = count
	}

	// Load INPUT_DATA_PATH
	cfg.InputDataPath = os.Getenv("INPUT_DATA_PATH")
	if cfg.InputDataPath == "" {
		return nil, fmt.Errorf("INPUT_DATA_PATH not set")
	}

	// Load INPUT_DATA_SIZE
	if val := os.Getenv("INPUT_DATA_SIZE"); val != "" {
		size, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid INPUT_DATA_SIZE: %w", err)
		}
		cfg.InputDataSize = size
	}

	// Load OUTPUT_BASE_PATH
	cfg.OutputBasePath = os.Getenv("OUTPUT_BASE_PATH")
	if cfg.OutputBasePath == "" {
		cfg.OutputBasePath = "./output" // Default local output
	}

	// Load optional fields
	cfg.JobID = os.Getenv("JOB_ID")
	cfg.DistributionMode = os.Getenv("DISTRIBUTION_MODE")
	if cfg.DistributionMode == "" {
		cfg.DistributionMode = "BYTE_RANGE"
	}

	cfg.EnableDistributed = os.Getenv("ENABLE_DISTRIBUTED_MODE") == "true"

	// Default TotalInstances to 1 (single instance mode)
	if cfg.TotalInstances == 0 {
		cfg.TotalInstances = 1
	}

	return cfg, nil
}

// Validate checks that configuration is valid
func (c *DistributedConfig) Validate() error {
	if c.InstanceID < 0 {
		return fmt.Errorf("InstanceID must be >= 0, got %d", c.InstanceID)
	}
	if c.InstanceID >= c.TotalInstances {
		return fmt.Errorf("InstanceID %d >= TotalInstances %d", c.InstanceID, c.TotalInstances)
	}
	if c.TotalInstances < 1 {
		return fmt.Errorf("TotalInstances must be >= 1, got %d", c.TotalInstances)
	}
	if c.InputDataSize < 0 {
		return fmt.Errorf("InputDataSize cannot be negative, got %d", c.InputDataSize)
	}
	return nil
}
