package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

type GeminiSentimentAnalyzer struct {
	client         *genai.Client
	model          string
	language       string
	selectedFields map[string]struct{}
}

func NewGeminiSentimentAnalyzer(cfg *DistributedConfig) (*GeminiSentimentAnalyzer, error) {
	project := os.Getenv("BATCH_PROJECT_ID")
	if project == "" {
		project = os.Getenv("GCP_PROJECT")
	}
	if project == "" {
		return nil, fmt.Errorf("BATCH_PROJECT_ID or GCP_PROJECT must be set for Gemini sentiment provider")
	}

	location := os.Getenv("BATCH_REGION")
	if location == "" {
		location = "us-central1"
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	return &GeminiSentimentAnalyzer{
		client:         client,
		model:          cfg.SentimentModel,
		language:       cfg.SentimentLanguage,
		selectedFields: toFieldSet(cfg.SentimentFields),
	}, nil
}

func (a *GeminiSentimentAnalyzer) ProviderName() string { return SentimentProviderGemini }

type geminiSentimentResponse struct {
	Label    string   `json:"label"`
	Score    float64  `json:"score"`
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

func (a *GeminiSentimentAnalyzer) Analyze(ctx context.Context, record string) (*RecordSentiment, error) {
	text := textForSentiment(record, a.selectedFields)
	if strings.TrimSpace(text) == "" {
		return &RecordSentiment{Label: "neutral", Score: 0}, nil
	}

	systemInstruction := `You are a deterministic sentiment analyzer.
Given text content, produce ONLY valid JSON with this schema:
{"label":"positive|neutral|negative","score":<number from -1 to 1>,"summary":"one concise sentence","keywords":["k1","k2","k3","k4","k5"]}
Rules:
- Score must be in [-1,1].
- label must align with score.
- keywords should be lowercase, non-empty, max 8 total.
- summary should be concise, factual, and mention key polarity drivers.
- No markdown, no code fences, no extra text.`

	prompt := fmt.Sprintf("Language hint: %s\nText:\n%s", a.language, text)

	result, err := a.client.Models.GenerateContent(ctx, a.model, genai.Text(prompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser),
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	raw := strings.TrimSpace(result.Text())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var resp geminiSentimentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response %q: %w", raw, err)
	}

	score := resp.Score
	if score > 1 {
		score = 1
	}
	if score < -1 {
		score = -1
	}

	label := strings.ToLower(strings.TrimSpace(resp.Label))
	switch label {
	case "positive", "negative", "neutral":
	default:
		label = SentimentLabel(score)
	}

	keywords := make([]string, 0, len(resp.Keywords))
	seen := make(map[string]struct{})
	for _, k := range resp.Keywords {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" {
			continue
		}
		if _, ok := seen[kk]; ok {
			continue
		}
		seen[kk] = struct{}{}
		keywords = append(keywords, kk)
		if len(keywords) >= 8 {
			break
		}
	}

	return &RecordSentiment{
		Score:    score,
		Label:    label,
		Keywords: keywords,
		Summary:  strings.TrimSpace(resp.Summary),
	}, nil
}
