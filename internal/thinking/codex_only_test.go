package thinking

import (
	"bytes"
	"testing"
)

func TestExtractThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	geminiBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := extractThinkingConfig(geminiBody, "gemini"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(gemini) = %+v, want empty config", got)
	}

	claudeBody := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024}}`)
	if got := extractThinkingConfig(claudeBody, "claude"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(claude) = %+v, want empty config", got)
	}
}

func TestStripThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	geminiBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := StripThinkingConfig(geminiBody, "gemini"); !bytes.Equal(got, geminiBody) {
		t.Fatalf("StripThinkingConfig(gemini) changed body to %s", got)
	}

	claudeBody := []byte(`{"thinking":{"type":"disabled"},"output_config":{"effort":"high"}}`)
	if got := StripThinkingConfig(claudeBody, "claude"); !bytes.Equal(got, claudeBody) {
		t.Fatalf("StripThinkingConfig(claude) changed body to %s", got)
	}
}
