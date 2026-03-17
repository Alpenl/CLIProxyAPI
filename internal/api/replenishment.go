package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	managementHandlers "github.com/router-for-me/CLIProxyAPI/v6/internal/api/handlers/management"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/replenishment"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (s *Server) configureReplenishment() {
	if s == nil {
		return
	}

	var manager *replenishment.AutoManager
	if cfg := s.cfg; cfg != nil {
		serviceURL := strings.TrimSpace(cfg.Replenishment.ServiceURL)
		serviceToken := strings.TrimSpace(cfg.Replenishment.ServiceToken)
		if serviceURL != "" && serviceToken != "" && s.handlers != nil && s.handlers.AuthManager != nil && s.mgmt != nil {
			client, err := replenishment.NewClient(serviceURL, serviceToken, &http.Client{Timeout: 30 * time.Second})
			if err != nil {
				log.WithError(err).Warn("failed to configure replenishment client")
			} else {
				manager = replenishment.NewAutoManager(replenishment.AutoManagerOptions{
					GetConfig: func() replenishment.ConfigSnapshot {
						current := s.cfg
						if current == nil {
							return replenishment.ConfigSnapshot{}
						}
						return replenishment.ConfigSnapshot{
							Enabled:            current.Replenishment.Enabled,
							TargetAccountCount: current.Replenishment.TargetAccountCount,
						}
					},
					ListAuths: func() []*auth.Auth {
						if s.handlers == nil || s.handlers.AuthManager == nil {
							return nil
						}
						return s.handlers.AuthManager.List()
					},
					Client: client,
					ImportArchive: func(ctx context.Context, data []byte) error {
						_, err := s.mgmt.ImportCodexArchiveBytes(ctx, data)
						return err
					},
				})
			}
		}
	}

	s.replenishmentMu.Lock()
	defer s.replenishmentMu.Unlock()
	s.replenishmentManager = manager
}

func (s *Server) replenishmentEnabled() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	cfg := s.cfg.Replenishment
	return cfg.Enabled && strings.TrimSpace(cfg.ServiceURL) != "" && strings.TrimSpace(cfg.ServiceToken) != ""
}

func (s *Server) replenishmentInterval() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.Replenishment.CheckIntervalSeconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(s.cfg.Replenishment.CheckIntervalSeconds) * time.Second
}

func (s *Server) currentReplenishmentManager() *replenishment.AutoManager {
	if s == nil {
		return nil
	}
	s.replenishmentMu.RLock()
	defer s.replenishmentMu.RUnlock()
	manager, _ := s.replenishmentManager.(*replenishment.AutoManager)
	return manager
}

func (s *Server) runReplenishmentOnce(ctx context.Context) error {
	manager := s.currentReplenishmentManager()
	if manager == nil {
		return nil
	}
	return manager.RunOnce(ctx)
}

func (s *Server) triggerReplenishment(ctx context.Context) (managementHandlers.ReplenishmentStatus, error) {
	manager := s.currentReplenishmentManager()
	if manager == nil {
		status, _ := s.replenishmentStatus(ctx)
		return status, fmt.Errorf("replenishment service is not configured")
	}
	if err := manager.RunManual(ctx); err != nil {
		status, _ := s.replenishmentStatus(ctx)
		return status, err
	}
	return s.replenishmentStatus(ctx)
}

func (s *Server) replenishmentStatus(_ context.Context) (managementHandlers.ReplenishmentStatus, error) {
	status := managementHandlers.ReplenishmentStatus{}
	if s == nil || s.cfg == nil {
		return status, nil
	}

	cfg := s.cfg.Replenishment
	status.Configured = strings.TrimSpace(cfg.ServiceURL) != "" && strings.TrimSpace(cfg.ServiceToken) != ""
	status.Enabled = cfg.Enabled
	status.ServiceURL = cfg.ServiceURL
	status.TargetAccountCount = cfg.TargetAccountCount
	status.CheckInterval = cfg.CheckIntervalSeconds

	var auths []*auth.Auth
	if s.handlers != nil && s.handlers.AuthManager != nil {
		auths = s.handlers.AuthManager.List()
	}

	pool := replenishment.SummarizeAuthPool(auths)
	pool.Target = cfg.TargetAccountCount

	manager := s.currentReplenishmentManager()
	if manager != nil {
		state := manager.State()
		status.ServiceReady = true
		status.CurrentJob = state.CurrentJob
		status.FailureCount = state.FailureCount
		if !state.NextAttemptAt.IsZero() {
			next := state.NextAttemptAt
			status.NextAttemptAt = &next
		}
		if state.CurrentJob != nil {
			reserved := state.CurrentJob.RequestedSuccesses - state.CurrentJob.SuccessCount
			if reserved > 0 {
				pool.Reserved = reserved
			}
		}
	}

	status.Pool = pool
	status.Deficit = replenishment.ComputeDeficit(pool)
	return status, nil
}

func (s *Server) startReplenishmentLoop() {
	if s == nil || !s.replenishmentEnabled() {
		return
	}

	s.replenishmentMu.Lock()
	if s.replenishmentStop != nil {
		s.replenishmentMu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	s.replenishmentStop = stop
	s.replenishmentDone = done
	s.replenishmentMu.Unlock()

	go func() {
		defer close(done)

		if err := s.runReplenishmentOnce(context.Background()); err != nil {
			log.WithError(err).Warn("initial replenishment sync failed")
		}

		ticker := time.NewTicker(s.replenishmentInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.runReplenishmentOnce(context.Background()); err != nil {
					log.WithError(err).Warn("scheduled replenishment sync failed")
				}
			case <-stop:
				return
			}
		}
	}()
}

func (s *Server) stopReplenishmentLoop() {
	if s == nil {
		return
	}

	s.replenishmentMu.Lock()
	stop := s.replenishmentStop
	done := s.replenishmentDone
	s.replenishmentStop = nil
	s.replenishmentDone = nil
	s.replenishmentMu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	if done != nil {
		<-done
	}
}

func (s *Server) restartReplenishmentLoopIfRunning() {
	if s == nil {
		return
	}

	s.replenishmentMu.RLock()
	running := s.replenishmentStop != nil
	s.replenishmentMu.RUnlock()

	if !running {
		if s.replenishmentEnabled() {
			s.startReplenishmentLoop()
		}
		return
	}
	s.stopReplenishmentLoop()
	s.startReplenishmentLoop()
}
