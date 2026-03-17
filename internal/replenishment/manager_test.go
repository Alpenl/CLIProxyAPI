package replenishment

import (
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestComputeDeficit_ExcludesReservedJobs(t *testing.T) {
	snapshot := PoolSnapshot{
		Target:   10,
		Healthy:  6,
		Warming:  1,
		Reserved: 2,
	}

	if got := ComputeDeficit(snapshot); got != 1 {
		t.Fatalf("ComputeDeficit() = %d, want 1", got)
	}
}

func TestSummarizeAuthPool_ClassifiesCodexAccounts(t *testing.T) {
	now := time.Now()
	auths := []*coreauth.Auth{
		{
			Provider: "codex",
			Status:   coreauth.StatusActive,
		},
		{
			Provider: "codex",
			Status:   coreauth.StatusPending,
		},
		{
			Provider:    "codex",
			Status:      coreauth.StatusActive,
			Unavailable: true,
			Quota: coreauth.QuotaState{
				Exceeded:      true,
				NextRecoverAt: now.Add(time.Hour),
			},
		},
		{
			Provider: "codex",
			Status:   coreauth.StatusError,
		},
		{
			Provider: "other",
			Status:   coreauth.StatusActive,
		},
	}

	summary := SummarizeAuthPool(auths)
	if summary.Total != 4 {
		t.Fatalf("Total = %d, want 4", summary.Total)
	}
	if summary.Healthy != 1 {
		t.Fatalf("Healthy = %d, want 1", summary.Healthy)
	}
	if summary.Warming != 1 {
		t.Fatalf("Warming = %d, want 1", summary.Warming)
	}
	if summary.Cooling != 1 {
		t.Fatalf("Cooling = %d, want 1", summary.Cooling)
	}
	if summary.Invalid != 1 {
		t.Fatalf("Invalid = %d, want 1", summary.Invalid)
	}
}
