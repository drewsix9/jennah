package demo

import "testing"

func TestLoadConfig_DefaultsToRecordModeWhenDWPEnabled(t *testing.T) {
	t.Setenv("INPUT_DATA_PATH", "/tmp/input.txt")
	t.Setenv("ENABLE_DISTRIBUTED_MODE", "true")
	t.Setenv("DISTRIBUTION_MODE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.DistributionMode != DistributionModeRecord {
		t.Fatalf("expected distribution mode %q, got %q", DistributionModeRecord, cfg.DistributionMode)
	}
}

func TestLoadConfig_DefaultsToByteRangeWhenDWPDisabled(t *testing.T) {
	t.Setenv("INPUT_DATA_PATH", "/tmp/input.txt")
	t.Setenv("ENABLE_DISTRIBUTED_MODE", "false")
	t.Setenv("DISTRIBUTION_MODE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.DistributionMode != DistributionModeByteRange {
		t.Fatalf("expected distribution mode %q, got %q", DistributionModeByteRange, cfg.DistributionMode)
	}
}

func TestLoadConfig_DefaultsToRecordModeWhenMultipleInstances(t *testing.T) {
	t.Setenv("INPUT_DATA_PATH", "/tmp/input.txt")
	t.Setenv("ENABLE_DISTRIBUTED_MODE", "false")
	t.Setenv("DISTRIBUTION_MODE", "")
	t.Setenv("BATCH_TASK_COUNT", "4")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.DistributionMode != DistributionModeRecord {
		t.Fatalf("expected distribution mode %q, got %q", DistributionModeRecord, cfg.DistributionMode)
	}
}

func TestLoadConfig_SentimentDefaults(t *testing.T) {
	t.Setenv("INPUT_DATA_PATH", "/tmp/input.txt")
	t.Setenv("SENTIMENT_PROVIDER", "")
	t.Setenv("SENTIMENT_MODEL", "")
	t.Setenv("SENTIMENT_LANGUAGE", "")
	t.Setenv("SENTIMENT_TEXT_FIELDS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.SentimentProvider != SentimentProviderLexicon {
		t.Fatalf("expected sentiment provider %q, got %q", SentimentProviderLexicon, cfg.SentimentProvider)
	}
	if cfg.SentimentModel == "" {
		t.Fatalf("expected non-empty default sentiment model")
	}
	if cfg.SentimentLanguage != "auto" {
		t.Fatalf("expected sentiment language auto, got %q", cfg.SentimentLanguage)
	}
}

func TestLoadConfig_ParsesSentimentFields(t *testing.T) {
	t.Setenv("INPUT_DATA_PATH", "/tmp/input.txt")
	t.Setenv("SENTIMENT_TEXT_FIELDS", "title, review.text , summary")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if len(cfg.SentimentFields) != 3 {
		t.Fatalf("expected 3 sentiment fields, got %d (%v)", len(cfg.SentimentFields), cfg.SentimentFields)
	}
}
