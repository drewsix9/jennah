package demo

import "testing"

func TestExtractSemanticRecords_JSONArray(t *testing.T) {
	data := []byte(`[{"id":1,"text":"good"},{"id":2,"text":"bad"}]`)
	records := extractSemanticRecords(data)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestExtractSemanticRecords_FallbackLines(t *testing.T) {
	data := []byte("first line\nsecond line\n\nthird line\n")
	records := extractSemanticRecords(data)
	if len(records) != 3 {
		t.Fatalf("expected 3 non-empty line records, got %d", len(records))
	}
}
