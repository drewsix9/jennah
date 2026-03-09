package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// SentimentSummary is a lightweight sentiment aggregation.
type SentimentSummary struct {
	PositiveRecords int64    `json:"positive_records"`
	NegativeRecords int64    `json:"negative_records"`
	NeutralRecords  int64    `json:"neutral_records"`
	AverageScore    float64  `json:"average_score"`
	Label           string   `json:"label"`
	TopKeywords     []string `json:"top_keywords,omitempty"`
	Summary         string   `json:"summary,omitempty"`
}

func (s *SentimentSummary) TotalRecords() int64 {
	return s.PositiveRecords + s.NegativeRecords + s.NeutralRecords
}

type RecordSentiment struct {
	Score    float64
	Label    string
	Keywords []string
	Summary  string
}

type SentimentAnalyzer interface {
	ProviderName() string
	Analyze(ctx context.Context, record string) (*RecordSentiment, error)
}

type LexiconSentimentAnalyzer struct {
	fields map[string]struct{}
}

func (a *LexiconSentimentAnalyzer) ProviderName() string { return SentimentProviderLexicon }

func (a *LexiconSentimentAnalyzer) Analyze(_ context.Context, record string) (*RecordSentiment, error) {
	analysisText := textForSentiment(record, a.fields)
	score := sentimentScore(analysisText)
	return &RecordSentiment{
		Score:    score,
		Label:    SentimentLabel(score),
		Keywords: extractKeywords(analysisText),
	}, nil
}

type fallbackSentimentAnalyzer struct {
	primary  SentimentAnalyzer
	fallback SentimentAnalyzer
	once     sync.Once
}

func (a *fallbackSentimentAnalyzer) ProviderName() string {
	return a.primary.ProviderName()
}

func (a *fallbackSentimentAnalyzer) Analyze(ctx context.Context, record string) (*RecordSentiment, error) {
	res, err := a.primary.Analyze(ctx, record)
	if err == nil {
		return res, nil
	}

	a.once.Do(func() {
		log.Printf("sentiment provider %s failed; falling back to %s: %v",
			a.primary.ProviderName(), a.fallback.ProviderName(), err)
	})

	return a.fallback.Analyze(ctx, record)
}

func newSentimentAnalyzer(cfg *DistributedConfig) SentimentAnalyzer {
	lexicon := &LexiconSentimentAnalyzer{
		fields: toFieldSet(cfg.SentimentFields),
	}

	if cfg.SentimentProvider != SentimentProviderGemini {
		return lexicon
	}

	gemini, err := NewGeminiSentimentAnalyzer(cfg)
	if err != nil {
		log.Printf("sentiment provider gemini unavailable; using lexicon fallback: %v", err)
		return lexicon
	}

	return &fallbackSentimentAnalyzer{
		primary:  gemini,
		fallback: lexicon,
	}
}

type sentimentAccumulator struct {
	summary    SentimentSummary
	totalScore float64
	keywords   map[string]int64
}

func newSentimentAccumulator() *sentimentAccumulator {
	return &sentimentAccumulator{
		summary: SentimentSummary{
			Label: "neutral",
		},
		keywords: make(map[string]int64),
	}
}

func (a *sentimentAccumulator) addResult(result *RecordSentiment) {
	if result == nil {
		return
	}

	label := result.Label
	if label == "" {
		label = SentimentLabel(result.Score)
	}
	switch label {
	case "positive":
		a.summary.PositiveRecords++
	case "negative":
		a.summary.NegativeRecords++
	default:
		a.summary.NeutralRecords++
	}

	for _, tok := range result.Keywords {
		if t := strings.ToLower(strings.TrimSpace(tok)); t != "" {
			a.keywords[t]++
		}
	}

	a.totalScore += result.Score
}

func (a *sentimentAccumulator) result() *SentimentSummary {
	total := a.summary.TotalRecords()
	if total == 0 {
		return nil
	}
	a.summary.AverageScore = a.totalScore / float64(total)
	a.summary.Label = SentimentLabel(a.summary.AverageScore)
	a.summary.TopKeywords = topKeywordsFromCounts(a.keywords, 8)
	a.summary.Summary = buildSentimentSummary(&a.summary)
	return &a.summary
}

// SentimentLabel maps a sentiment score in [-1,1] to a label.
func SentimentLabel(score float64) string {
	switch {
	case score >= 0.2:
		return "positive"
	case score <= -0.2:
		return "negative"
	default:
		return "neutral"
	}
}

var tokenPattern = regexp.MustCompile(`[a-zA-Z']+`)

func sentimentScore(text string) float64 {
	positiveLexicon := map[string]struct{}{
		"amazing": {}, "awesome": {}, "best": {}, "excellent": {}, "fantastic": {},
		"fast": {}, "good": {}, "great": {}, "happy": {}, "love": {}, "positive": {},
		"recommend": {}, "reliable": {}, "solid": {}, "wonderful": {},
	}
	negativeLexicon := map[string]struct{}{
		"awful": {}, "bad": {}, "broken": {}, "disappointing": {}, "error": {},
		"fail": {}, "hate": {}, "negative": {}, "poor": {}, "problem": {},
		"slow": {}, "terrible": {}, "unhappy": {}, "worst": {},
	}

	tokens := tokenPattern.FindAllString(strings.ToLower(text), -1)
	if len(tokens) == 0 {
		return 0
	}

	pos := 0
	neg := 0
	for _, tok := range tokens {
		if _, ok := positiveLexicon[tok]; ok {
			pos++
		}
		if _, ok := negativeLexicon[tok]; ok {
			neg++
		}
	}
	if pos == 0 && neg == 0 {
		return 0
	}
	return float64(pos-neg) / float64(pos+neg)
}

func toFieldSet(fields []string) map[string]struct{} {
	if len(fields) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		key := strings.ToLower(strings.TrimSpace(f))
		if key != "" {
			set[key] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func textForSentiment(input string, selectedFields map[string]struct{}) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		var parts []string
		if len(selectedFields) > 0 {
			collectSelectedTextValues(payload, "", selectedFields, &parts)
		}
		if len(parts) == 0 {
			collectTextValues(payload, &parts)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}

	return trimmed
}

func collectSelectedTextValues(v interface{}, path string, selected map[string]struct{}, out *[]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, vv := range t {
			lk := strings.ToLower(k)
			nextPath := lk
			if path != "" {
				nextPath = path + "." + lk
			}

			_, directKey := selected[lk]
			_, fullPath := selected[nextPath]
			if directKey || fullPath {
				collectTextValues(vv, out)
				continue
			}

			collectSelectedTextValues(vv, nextPath, selected, out)
		}
	case []interface{}:
		for _, vv := range t {
			collectSelectedTextValues(vv, path, selected, out)
		}
	}
}

func collectTextValues(v interface{}, out *[]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		for _, vv := range t {
			collectTextValues(vv, out)
		}
	case []interface{}:
		for _, vv := range t {
			collectTextValues(vv, out)
		}
	case string:
		if s := strings.TrimSpace(t); s != "" {
			*out = append(*out, s)
		}
	case float64:
		*out = append(*out, strconv.FormatFloat(t, 'f', -1, 64))
	case bool:
		*out = append(*out, strconv.FormatBool(t))
	}
}

func extractKeywords(text string) []string {
	stopwords := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
		"for": {}, "from": {}, "in": {}, "is": {}, "it": {}, "of": {}, "on": {}, "or": {},
		"that": {}, "the": {}, "this": {}, "to": {}, "with": {}, "without": {}, "you": {},
		"your": {}, "was": {}, "were": {}, "will": {}, "can": {}, "could": {}, "should": {},
	}

	seen := make(map[string]struct{})
	var keywords []string
	for _, tok := range tokenPattern.FindAllString(strings.ToLower(text), -1) {
		if len(tok) < 3 {
			continue
		}
		if _, stop := stopwords[tok]; stop {
			continue
		}
		if _, exists := seen[tok]; exists {
			continue
		}
		seen[tok] = struct{}{}
		keywords = append(keywords, tok)
	}
	return keywords
}

func topKeywordsFromCounts(counts map[string]int64, limit int) []string {
	type kv struct {
		key   string
		count int64
	}
	all := make([]kv, 0, len(counts))
	for k, c := range counts {
		if c > 0 {
			all = append(all, kv{key: k, count: c})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].count == all[j].count {
			return all[i].key < all[j].key
		}
		return all[i].count > all[j].count
	})

	if limit <= 0 || limit > len(all) {
		limit = len(all)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, all[i].key)
	}
	return out
}

func buildSentimentSummary(s *SentimentSummary) string {
	if s == nil || s.TotalRecords() == 0 {
		return ""
	}
	text := fmt.Sprintf(
		"Processed %d records: %d positive, %d neutral, %d negative. Overall sentiment is %s (score %.3f).",
		s.TotalRecords(), s.PositiveRecords, s.NeutralRecords, s.NegativeRecords, s.Label, s.AverageScore,
	)
	if len(s.TopKeywords) > 0 {
		text += " Top keywords: " + strings.Join(s.TopKeywords, ", ") + "."
	}
	return text
}
