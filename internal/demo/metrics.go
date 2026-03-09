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
	// DistributionMode indicates how work was split: BYTE_RANGE or RECORD.
	DistributionMode string `json:"distribution_mode,omitempty"`
	// SentimentProvider indicates which analyzer produced sentiment output.
	SentimentProvider string `json:"sentiment_provider,omitempty"`

	// Byte range info
	StartByte      int64 `json:"start_byte"`
	EndByte        int64 `json:"end_byte"`
	BytesProcessed int64 `json:"bytes_processed"`
	// Record range info (for RECORD mode). Inclusive indices.
	StartRecord int64 `json:"start_record,omitempty"`
	EndRecord   int64 `json:"end_record,omitempty"`

	// Counts
	LinesCount      int64 `json:"lines_count"`
	WordsCount      int64 `json:"words_count"`
	CharactersCount int64 `json:"characters_count"`
	// RecordsProcessed is the number of semantic records processed by this task.
	RecordsProcessed int64 `json:"records_processed,omitempty"`
	// Sentiment summary for records processed by this task.
	Sentiment *SentimentSummary `json:"sentiment,omitempty"`

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
	TotalRecords        int64 `json:"total_records,omitempty"`

	// Timing metrics
	MaxProcessingTime float64 `json:"max_processing_time_seconds"`
	MinProcessingTime float64 `json:"min_processing_time_seconds"`
	AvgProcessingTime float64 `json:"avg_processing_time_seconds"`

	// Speedup metrics (vs baseline single instance)
	Speedup    float64 `json:"speedup,omitempty"`
	Efficiency float64 `json:"efficiency,omitempty"`

	// Metadata
	Timestamp string `json:"timestamp"`
	// Aggregated sentiment across all instances.
	Sentiment *SentimentSummary `json:"sentiment,omitempty"`
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
		a.TotalRecords += m.RecordsProcessed
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

	a.aggregateSentiment()

	a.Timestamp = time.Now().UTC().Format(time.RFC3339)
}

// ToJSON returns aggregated metrics as formatted JSON
func (a *AggregatedMetrics) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

func (a *AggregatedMetrics) aggregateSentiment() {
	s := &SentimentSummary{}
	totalWeightedScore := 0.0
	totalRecordsWithSentiment := int64(0)
	keywordCounts := make(map[string]int64)

	for i := range a.Instances {
		m := a.Instances[i]
		if m.Sentiment == nil {
			continue
		}
		s.PositiveRecords += m.Sentiment.PositiveRecords
		s.NegativeRecords += m.Sentiment.NegativeRecords
		s.NeutralRecords += m.Sentiment.NeutralRecords
		totalWeightedScore += m.Sentiment.AverageScore * float64(m.Sentiment.TotalRecords())
		totalRecordsWithSentiment += m.Sentiment.TotalRecords()
		for _, k := range m.Sentiment.TopKeywords {
			keywordCounts[k]++
		}
	}

	if totalRecordsWithSentiment == 0 {
		return
	}

	s.AverageScore = totalWeightedScore / float64(totalRecordsWithSentiment)
	s.Label = SentimentLabel(s.AverageScore)
	s.TopKeywords = topKeywordsFromCounts(keywordCounts, 8)
	s.Summary = buildSentimentSummary(s)
	a.Sentiment = s
}
