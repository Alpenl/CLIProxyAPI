package usage

import (
	"context"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestRequestStatisticsCapsPerModelDetails(t *testing.T) {
	stats := NewRequestStatistics()

	for i := 0; i < requestDetailsLimitPerModel+7; i++ {
		stats.Record(context.Background(), coreusage.Record{
			APIKey:      "cap-test",
			Provider:    "codex",
			Model:       "gpt-5-codex",
			RequestedAt: time.Unix(int64(1700000000+i), 0).UTC(),
			Detail: coreusage.Detail{
				TotalTokens: int64(i + 1),
			},
		})
	}

	snapshot := stats.Snapshot()
	apiSnapshot := snapshot.APIs["cap-test"]
	modelSnapshot := apiSnapshot.Models["gpt-5-codex"]

	if got := len(modelSnapshot.Details); got != requestDetailsLimitPerModel {
		t.Fatalf("details len = %d, want %d", got, requestDetailsLimitPerModel)
	}
	if got := modelSnapshot.Details[0].Tokens.TotalTokens; got != 8 {
		t.Fatalf("oldest retained total tokens = %d, want 8", got)
	}
	if got := modelSnapshot.Details[len(modelSnapshot.Details)-1].Tokens.TotalTokens; got != int64(requestDetailsLimitPerModel+7) {
		t.Fatalf("newest retained total tokens = %d, want %d", got, requestDetailsLimitPerModel+7)
	}
}

func TestRequestStatisticsSummarySnapshotOmitsDetails(t *testing.T) {
	stats := NewRequestStatistics()
	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "summary-test",
		Provider:    "codex",
		Model:       "gpt-5-codex",
		RequestedAt: time.Unix(1700000000, 0).UTC(),
		Detail: coreusage.Detail{
			TotalTokens: 21,
		},
	})

	summary := stats.SummarySnapshot()
	apiSnapshot := summary.APIs["summary-test"]
	modelSnapshot := apiSnapshot.Models["gpt-5-codex"]

	if got := modelSnapshot.TotalRequests; got != 1 {
		t.Fatalf("summary total requests = %d, want 1", got)
	}
	if modelSnapshot.Details != nil {
		t.Fatalf("expected summary details to be omitted, got %#v", modelSnapshot.Details)
	}

	full := stats.Snapshot()
	if got := len(full.APIs["summary-test"].Models["gpt-5-codex"].Details); got != 1 {
		t.Fatalf("full snapshot details len = %d, want 1", got)
	}
}
