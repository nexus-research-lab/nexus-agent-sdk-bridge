package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseTokenUsage(t *testing.T) {
	raw := map[string]any{
		"input_tokens":                json.Number("10"),
		"output_tokens":               float64(20),
		"cache_creation_input_tokens": int64(3),
		"cache_read_input_tokens":     4,
		"reasoning_tokens":            5,
		"provider_field":              "kept",
	}

	usage, ok := ParseTokenUsage(raw)
	if !ok {
		t.Fatal("ParseTokenUsage() ok = false, want true")
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 20 || usage.CacheCreationInputTokens != 3 || usage.CacheReadInputTokens != 4 || usage.ReasoningTokens != 5 {
		t.Fatalf("usage = %#v, want parsed token fields", usage)
	}
	if usage.TotalTokens != 42 {
		t.Fatalf("TotalTokens = %d, want 42", usage.TotalTokens)
	}
	if usage.Raw["provider_field"] != "kept" {
		t.Fatalf("Raw = %#v, want provider field preserved", usage.Raw)
	}
}

func TestTokenUsageFromResultFallsBackToModelUsage(t *testing.T) {
	result := ResultMessage{
		ModelUsage: map[string]any{
			"model-a": map[string]any{"input_tokens": 10, "output_tokens": 20},
			"model-b": map[string]any{"input_tokens": 3, "output_tokens": 4},
		},
	}

	usage, ok := result.TokenUsage()
	if !ok {
		t.Fatal("TokenUsage() ok = false, want true")
	}
	if usage.InputTokens != 13 || usage.OutputTokens != 24 || usage.TotalTokens != 37 {
		t.Fatalf("usage = %#v, want aggregated model usage", usage)
	}
}

func TestTokenUsageAddAndZero(t *testing.T) {
	var usage TokenUsage
	if !usage.IsZero() {
		t.Fatal("zero usage IsZero() = false, want true")
	}
	usage = usage.Add(TokenUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3})
	if usage.IsZero() {
		t.Fatal("non-zero usage IsZero() = true, want false")
	}
	if usage.InputTokens != 1 || usage.OutputTokens != 2 || usage.TotalTokens != 3 {
		t.Fatalf("usage = %#v, want summed fields", usage)
	}
}
