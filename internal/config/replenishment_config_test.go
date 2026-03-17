package config

import "testing"

func TestDefaultBootstrapConfig_ReplenishmentDefaults(t *testing.T) {
	cfg := DefaultBootstrapConfig()

	if cfg.Replenishment.Enabled {
		t.Fatalf("Replenishment.Enabled = true, want false")
	}
	if cfg.Replenishment.TargetAccountCount != 10 {
		t.Fatalf("Replenishment.TargetAccountCount = %d, want 10", cfg.Replenishment.TargetAccountCount)
	}
	if cfg.Replenishment.CheckIntervalSeconds != 300 {
		t.Fatalf("Replenishment.CheckIntervalSeconds = %d, want 300", cfg.Replenishment.CheckIntervalSeconds)
	}
	if cfg.Replenishment.QuotaRefreshIntervalSeconds != 3600 {
		t.Fatalf("Replenishment.QuotaRefreshIntervalSeconds = %d, want 3600", cfg.Replenishment.QuotaRefreshIntervalSeconds)
	}
	if !cfg.Replenishment.CleanupInvalidAccounts {
		t.Fatalf("Replenishment.CleanupInvalidAccounts = false, want true")
	}
}
