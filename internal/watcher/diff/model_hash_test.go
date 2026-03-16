package diff

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestComputeCodexModelsHash_Deterministic(t *testing.T) {
	models := []config.CodexModel{
		{Name: "gpt-5", Alias: "g5"},
		{Name: "gpt-5-mini"},
	}
	hash1 := ComputeCodexModelsHash(models)
	hash2 := ComputeCodexModelsHash(models)
	if hash1 == "" {
		t.Fatal("hash should not be empty")
	}
	if hash1 != hash2 {
		t.Fatalf("hash should be deterministic, got %s vs %s", hash1, hash2)
	}
	changed := ComputeCodexModelsHash([]config.CodexModel{{Name: "gpt-5"}, {Name: "gpt-5.1"}})
	if hash1 == changed {
		t.Fatal("hash should change when model list changes")
	}
}

func TestComputeCodexModelsHash_NormalizesAndDedups(t *testing.T) {
	a := []config.CodexModel{
		{Name: "gpt-5", Alias: "g5"},
		{Name: " "},
		{Name: "GPT-5", Alias: "G5"},
		{Alias: "alias-only"},
	}
	b := []config.CodexModel{
		{Alias: "ALIAS-ONLY"},
		{Name: "gpt-5", Alias: "g5"},
	}
	h1 := ComputeCodexModelsHash(a)
	h2 := ComputeCodexModelsHash(b)
	if h1 == "" || h2 == "" {
		t.Fatal("expected non-empty hashes for non-empty model sets")
	}
	if h1 != h2 {
		t.Fatalf("expected normalized hashes to match, got %s / %s", h1, h2)
	}
}

func TestComputeCodexModelsHash_Empty(t *testing.T) {
	if got := ComputeCodexModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil models, got %q", got)
	}
	if got := ComputeCodexModelsHash([]config.CodexModel{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
	if got := ComputeCodexModelsHash([]config.CodexModel{{Name: " "}, {Alias: ""}}); got != "" {
		t.Fatalf("expected empty hash for blank models, got %q", got)
	}
}

func TestComputeExcludedModelsHash_Normalizes(t *testing.T) {
	hash1 := ComputeExcludedModelsHash([]string{" A ", "b", "a"})
	hash2 := ComputeExcludedModelsHash([]string{"a", " b", "A"})
	if hash1 == "" || hash2 == "" {
		t.Fatal("hash should not be empty for non-empty input")
	}
	if hash1 != hash2 {
		t.Fatalf("hash should be order/space insensitive for same multiset, got %s vs %s", hash1, hash2)
	}
	hash3 := ComputeExcludedModelsHash([]string{"c"})
	if hash1 == hash3 {
		t.Fatal("hash should differ for different normalized sets")
	}
}

func TestComputeExcludedModelsHash_Empty(t *testing.T) {
	if got := ComputeExcludedModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil input, got %q", got)
	}
	if got := ComputeExcludedModelsHash([]string{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
	if got := ComputeExcludedModelsHash([]string{"  ", ""}); got != "" {
		t.Fatalf("expected empty hash for whitespace-only entries, got %q", got)
	}
}
