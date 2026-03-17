package replenishment

import (
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func ComputeDeficit(snapshot PoolSnapshot) int {
	usable := snapshot.Healthy + snapshot.Warming
	deficit := snapshot.Target - usable - snapshot.Reserved
	if deficit < 0 {
		return 0
	}
	return deficit
}

func SummarizeAuthPool(auths []*coreauth.Auth) PoolSnapshot {
	summary := PoolSnapshot{}
	for _, auth := range auths {
		if auth == nil || auth.Provider != "codex" {
			continue
		}
		summary.Total++

		switch {
		case auth.Disabled || auth.Status == coreauth.StatusDisabled:
			summary.Invalid++
		case auth.Unavailable || auth.Quota.Exceeded:
			summary.Cooling++
		case auth.Status == coreauth.StatusPending || auth.Status == coreauth.StatusRefreshing:
			summary.Warming++
		case auth.Status == coreauth.StatusActive:
			summary.Healthy++
		default:
			summary.Invalid++
		}
	}
	return summary
}
