package thinking

import (
	"bytes"
	"testing"
)

func TestExtractThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	legacyBudgetBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := extractThinkingConfig(legacyBudgetBody, "other-a"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(other-a) = %+v, want empty config", got)
	}

	legacyTokenBody := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024}}`)
	if got := extractThinkingConfig(legacyTokenBody, "other-b"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(other-b) = %+v, want empty config", got)
	}
}

func TestStripThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	legacyBudgetBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := StripThinkingConfig(legacyBudgetBody, "other-a"); !bytes.Equal(got, legacyBudgetBody) {
		t.Fatalf("StripThinkingConfig(other-a) changed body to %s", got)
	}

	legacyTokenBody := []byte(`{"thinking":{"type":"disabled"},"output_config":{"effort":"high"}}`)
	if got := StripThinkingConfig(legacyTokenBody, "other-b"); !bytes.Equal(got, legacyTokenBody) {
		t.Fatalf("StripThinkingConfig(other-b) changed body to %s", got)
	}
}
