package demo

import (
	"encoding/json"
	"time"
)

// ProcessMetrics contains statistics about processing
type ProcessMetrics struct {
	// Identification
	InstanceID   int    `json:"instance_id"`
	TaskIndex    int    `json:"task_index"`
	TaskIndexMax int    `json:"task_index_max"`
	JobID        string `json:"job_id,omitempty"`

	// Byte range info
	StartByte      int64 `json:"start_byte"`
	EndByte        int64 `json:"end_byte"`
	BytesProcessed int64 `json:"bytes_processed"`

	// Counts
	LinesCount      int64 `json:"lines_count"`
	WordsCount      int64 `json:"words_count"`
	CharactersCount int64 `json:"characters_count"`

	// Timing
	ProcessingTimeSeconds float64 `json:"processing_time_seconds"`
	ThroughputMBPerSecond float64 `json:"throughput_mb_per_second"`

	// Metadata
	Timestamp string `json:"timestamp"`
}

// ToJSON returns the metrics as formatted JSON
func (m *ProcessMetrics) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// CalculateThroughput computes MB/s based on processed bytes and seconds
func (m *ProcessMetrics) CalculateThroughput() {
	if m.ProcessingTimeSeconds > 0 {
		bytesPerSecond := float64(m.BytesProcessed) / m.ProcessingTimeSeconds
		m.ThroughputMBPerSecond = bytesPerSecond / (1024 * 1024)
	}
}

// AggregatedMetrics combines results from all instances
type AggregatedMetrics struct {
	TotalInstances int              `json:"total_instances"`
	Instances      []ProcessMetrics `json:"instance_results"`

	// Aggregated counts
	TotalBytesProcessed int64 `json:"total_bytes_processed"`
	TotalLines          int64 `json:"total_lines"`
	TotalWords          int64 `json:"total_words"`
	TotalCharacters     int64 `json:"total_characters"`

	// Timing metrics
	MaxProcessingTime float64 `json:"max_processing_time_seconds"`
	MinProcessingTime float64 `json:"min_processing_time_seconds"`
	AvgProcessingTime float64 `json:"avg_processing_time_seconds"`

	// Speedup metrics (vs baseline single instance)
	Speedup    float64 `json:"speedup,omitempty"`
	Efficiency float64 `json:"efficiency,omitempty"`

	// Metadata
	Timestamp string `json:"timestamp"`
}

// Calculate aggregates metrics from individual instances
func (a *AggregatedMetrics) Calculate() {
	if len(a.Instances) == 0 {
		return
	}

	// Sum all counts
	for _, m := range a.Instances {
		a.TotalBytesProcessed += m.BytesProcessed
		a.TotalLines += m.LinesCount
		a.TotalWords += m.WordsCount
		a.TotalCharacters += m.CharactersCount
	}

	// Calculate time statistics
	maxTime := a.Instances[0].ProcessingTimeSeconds
	minTime := a.Instances[0].ProcessingTimeSeconds
	totalTime := 0.0

	for _, m := range a.Instances {
		if m.ProcessingTimeSeconds > maxTime {
			maxTime = m.ProcessingTimeSeconds
		}
		if m.ProcessingTimeSeconds < minTime {
			minTime = m.ProcessingTimeSeconds
		}
		totalTime += m.ProcessingTimeSeconds
	}

	a.MaxProcessingTime = maxTime
	a.MinProcessingTime = minTime
	a.AvgProcessingTime = totalTime / float64(len(a.Instances))

	// Calculate efficiency
	if a.MaxProcessingTime > 0 {
		a.Efficiency = a.AvgProcessingTime / a.MaxProcessingTime
	}

	a.Timestamp = time.Now().UTC().Format(time.RFC3339)
}

// ToJSON returns aggregated metrics as formatted JSON
func (a *AggregatedMetrics) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}
