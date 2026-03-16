package thinking

import (
	"bytes"
	"testing"
)

func TestExtractThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	legacyBudgetBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := extractThinkingConfig(legacyBudgetBody, "legacy-a"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(legacy-a) = %+v, want empty config", got)
	}

	legacyTokenBody := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024}}`)
	if got := extractThinkingConfig(legacyTokenBody, "legacy-b"); hasThinkingConfig(got) {
		t.Fatalf("extractThinkingConfig(legacy-b) = %+v, want empty config", got)
	}
}

func TestStripThinkingConfigIgnoresRemovedProviders(t *testing.T) {
	t.Parallel()

	legacyBudgetBody := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)
	if got := StripThinkingConfig(legacyBudgetBody, "legacy-a"); !bytes.Equal(got, legacyBudgetBody) {
		t.Fatalf("StripThinkingConfig(legacy-a) changed body to %s", got)
	}

	legacyTokenBody := []byte(`{"thinking":{"type":"disabled"},"output_config":{"effort":"high"}}`)
	if got := StripThinkingConfig(legacyTokenBody, "legacy-b"); !bytes.Equal(got, legacyTokenBody) {
		t.Fatalf("StripThinkingConfig(legacy-b) changed body to %s", got)
	}
}
