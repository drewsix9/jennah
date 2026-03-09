package demo

import (
	"context"
	"strings"
	"testing"
)

func TestSentimentLabel(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.8, "positive"},
		{0.2, "positive"},
		{0.1, "neutral"},
		{0.0, "neutral"},
		{-0.1, "neutral"},
		{-0.2, "negative"},
		{-0.8, "negative"},
	}
	for _, tc := range cases {
		if got := SentimentLabel(tc.score); got != tc.want {
			t.Errorf("SentimentLabel(%v)=%q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestSentimentScore_PositiveAndNegative(t *testing.T) {
	if score := sentimentScore("this is amazing and great"); score <= 0 {
		t.Fatalf("expected positive score, got %v", score)
	}
	if score := sentimentScore("this is awful and terrible"); score >= 0 {
		t.Fatalf("expected negative score, got %v", score)
	}
}

func TestSentimentAccumulator_ProducesSummaryAndKeywords(t *testing.T) {
	acc := newSentimentAccumulator()
	lex := &LexiconSentimentAnalyzer{}
	r1, _ := lex.Analyze(context.Background(), `{"text":"amazing fast delivery and great packaging"}`)
	r2, _ := lex.Analyze(context.Background(), `{"text":"awful quality and terrible support"}`)
	r3, _ := lex.Analyze(context.Background(), `{"text":"reliable solid build quality"}`)
	acc.addResult(r1)
	acc.addResult(r2)
	acc.addResult(r3)

	got := acc.result()
	if got == nil {
		t.Fatal("expected non-nil sentiment summary")
	}
	if got.Summary == "" {
		t.Fatal("expected summary text")
	}
	if len(got.TopKeywords) == 0 {
		t.Fatal("expected top keywords")
	}
}

func TestTextForSentiment_FieldSelection(t *testing.T) {
	input := `{"title":"Great product","metadata":{"id":"x1"},"description":"Fast and reliable","notes":"Ignore me"}`
	fields := toFieldSet([]string{"title", "description"})
	text := textForSentiment(input, fields)

	if !strings.Contains(strings.ToLower(text), "great product") {
		t.Fatalf("expected selected title in text: %q", text)
	}
	if !strings.Contains(strings.ToLower(text), "fast and reliable") {
		t.Fatalf("expected selected description in text: %q", text)
	}
	if strings.Contains(strings.ToLower(text), "ignore me") {
		t.Fatalf("did not expect unselected notes in text: %q", text)
	}
}
