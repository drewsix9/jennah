package demo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	SentimentProvider string
	SentimentModel    string
	SentimentLanguage string
	SentimentFields   []string
}

const (
	DistributionModeByteRange = "BYTE_RANGE"
	DistributionModeRecord    = "RECORD"
	SentimentProviderLexicon  = "lexicon"
	SentimentProviderGemini   = "gemini"
)

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
	cfg.EnableDistributed = strings.EqualFold(os.Getenv("ENABLE_DISTRIBUTED_MODE"), "true")
	cfg.DistributionMode = strings.ToUpper(strings.TrimSpace(os.Getenv("DISTRIBUTION_MODE")))
	if cfg.DistributionMode == "" {
		// Default to context-preserving record mode for DWP jobs.
		if cfg.EnableDistributed || cfg.TotalInstances > 1 {
			cfg.DistributionMode = DistributionModeRecord
		} else {
			cfg.DistributionMode = DistributionModeByteRange
		}
	}

	cfg.SentimentProvider = strings.ToLower(strings.TrimSpace(os.Getenv("SENTIMENT_PROVIDER")))
	if cfg.SentimentProvider == "" {
		cfg.SentimentProvider = SentimentProviderLexicon
	}
	cfg.SentimentModel = strings.TrimSpace(os.Getenv("SENTIMENT_MODEL"))
	if cfg.SentimentModel == "" {
		cfg.SentimentModel = "gemini-2.0-flash-001"
	}
	cfg.SentimentLanguage = strings.TrimSpace(os.Getenv("SENTIMENT_LANGUAGE"))
	if cfg.SentimentLanguage == "" {
		cfg.SentimentLanguage = "auto"
	}
	for _, raw := range strings.Split(os.Getenv("SENTIMENT_TEXT_FIELDS"), ",") {
		f := strings.ToLower(strings.TrimSpace(raw))
		if f != "" {
			cfg.SentimentFields = append(cfg.SentimentFields, f)
		}
	}

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
	switch c.DistributionMode {
	case DistributionModeByteRange, DistributionModeRecord:
	default:
		return fmt.Errorf("DistributionMode must be BYTE_RANGE or RECORD, got %q", c.DistributionMode)
	}
	switch c.SentimentProvider {
	case SentimentProviderLexicon, SentimentProviderGemini:
	default:
		return fmt.Errorf("SentimentProvider must be lexicon or gemini, got %q", c.SentimentProvider)
	}
	return nil
}
