package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
					BuildCreateJobInput: func(requestedSuccesses int) replenishment.CreateJobInput {
						return replenishment.CreateJobInput{
							RequestedSuccesses: requestedSuccesses,
							Source:             "auto",
							ZipRequired:        true,
							Callback:           s.replenishmentCallbackConfig(),
						}
					},
				})
			}
		}
	}

	s.replenishmentMu.Lock()
	defer s.replenishmentMu.Unlock()
	s.replenishmentManager = manager
}

func (s *Server) replenishmentCallbackConfig() *replenishment.CallbackConfig {
	if s == nil || s.cfg == nil {
		return nil
	}

	url := s.replenishmentCallbackURL()
	token := strings.TrimSpace(s.cfg.Replenishment.ServiceToken)
	if url == "" || token == "" {
		return nil
	}

	return &replenishment.CallbackConfig{
		URL:   url,
		Token: token,
	}
}

func (s *Server) replenishmentCallbackURL() string {
	if s == nil || s.cfg == nil || s.cfg.Port <= 0 {
		return ""
	}

	if baseURL := s.observedCallbackBaseURL(); baseURL != "" {
		return baseURL + "/v0/internal/replenishment/accounts"
	}

	scheme := "http"
	if s.cfg.TLS.Enable {
		scheme = "https"
	}
	host := resolveReplenishmentCallbackHost(s.cfg.Host, s.cfg.Port, os.Environ())
	return fmt.Sprintf("%s://%s:%d/v0/internal/replenishment/accounts", scheme, host, s.cfg.Port)
}

func (s *Server) rememberObservedCallbackBaseURL(req *http.Request) {
	if s == nil || req == nil {
		return
	}

	baseURL := requestBaseURL(req)
	if baseURL == "" {
		return
	}

	s.callbackBaseURLMu.Lock()
	s.callbackBaseURL = baseURL
	s.callbackBaseURLMu.Unlock()
}

func (s *Server) observedCallbackBaseURL() string {
	if s == nil {
		return ""
	}
	s.callbackBaseURLMu.RLock()
	defer s.callbackBaseURLMu.RUnlock()
	return s.callbackBaseURL
}

func requestBaseURL(req *http.Request) string {
	if req == nil {
		return ""
	}

	scheme := firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := firstForwardedValue(req.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(req.Host)
	}
	if host == "" {
		return ""
	}

	return strings.TrimRight(fmt.Sprintf("%s://%s", scheme, host), "/")
}

func firstForwardedValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.Index(value, ","); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func resolveReplenishmentCallbackHost(host string, port int, env []string) string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0", "::":
		if detected := detectKubernetesServiceHostForPort(port, env); detected != "" {
			return normalizeReplenishmentCallbackHost(detected)
		}
	}
	return normalizeReplenishmentCallbackHost(host)
}

func detectKubernetesServiceHostForPort(port int, env []string) string {
	if port <= 0 || len(env) == 0 {
		return ""
	}

	values := make(map[string]string, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}

	if strings.TrimSpace(values["KUBERNETES_SERVICE_HOST"]) == "" {
		return ""
	}

	targetPort := strconv.Itoa(port)
	type candidate struct {
		prefix string
		host   string
	}
	candidates := make([]candidate, 0, 2)
	for key, value := range values {
		if !strings.HasSuffix(key, "_SERVICE_PORT") || strings.TrimSpace(value) != targetPort {
			continue
		}

		prefix := strings.TrimSuffix(key, "_SERVICE_PORT")
		if prefix == "" || prefix == "KUBERNETES" {
			continue
		}

		host := strings.TrimSpace(values[prefix+"_SERVICE_HOST"])
		if host == "" {
			continue
		}

		candidates = append(candidates, candidate{
			prefix: prefix,
			host:   host,
		})
	}

	if len(candidates) == 0 {
		return ""
	}

	workloadPrefix := normalizeServiceEnvToken(strings.TrimSpace(values["HOSTNAME"]))
	for _, candidate := range candidates {
		if workloadPrefix != "" && strings.HasPrefix(candidate.prefix, workloadPrefix) {
			return candidate.host
		}
	}

	if len(candidates) == 1 {
		return candidates[0].host
	}

	return ""
}

func normalizeServiceEnvToken(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return ""
	}

	if index := strings.LastIndex(hostname, "-"); index > 0 && isDigits(hostname[index+1:]) {
		hostname = hostname[:index]
	}

	var builder strings.Builder
	builder.Grow(len(hostname))
	for _, r := range hostname {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r - ('a' - 'A'))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeReplenishmentCallbackHost(host string) string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "[::1]"
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
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

type replenishmentAccountCallbackRequest struct {
	JobID    string          `json:"jobId"`
	FileName string          `json:"fileName"`
	Account  json.RawMessage `json:"account"`
}

func (s *Server) handleReplenishmentAccountCallback(c *gin.Context) {
	if s == nil || s.cfg == nil || s.mgmt == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "replenishment callback is unavailable"})
		return
	}

	if !s.authorizeReplenishmentCallback(c.Request) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req replenishmentAccountCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.JobID = strings.TrimSpace(req.JobID)
	if req.JobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jobId is required"})
		return
	}
	if len(req.Account) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account is required"})
		return
	}

	if manager := s.currentReplenishmentManager(); manager != nil {
		if pending := manager.CurrentJob(); pending != nil && pending.ID != "" && pending.ID != req.JobID {
			c.JSON(http.StatusConflict, gin.H{"error": "job does not match current replenishment request"})
			return
		}
	}

	fileName := filepath.Base(strings.TrimSpace(req.FileName))
	result, err := s.mgmt.ImportCodexAccountBytes(c.Request.Context(), fileName, req.Account)
	if err != nil {
		if strings.EqualFold(result.Status, "skipped") {
			c.JSON(http.StatusOK, gin.H{"result": result})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "result": result})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (s *Server) authorizeReplenishmentCallback(req *http.Request) bool {
	if s == nil || s.cfg == nil || req == nil {
		return false
	}

	expected := strings.TrimSpace(s.cfg.Replenishment.ServiceToken)
	provided := readBearerToken(req)
	if expected == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func readBearerToken(req *http.Request) string {
	if req == nil {
		return ""
	}
	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return strings.TrimSpace(req.Header.Get("X-Replenishment-Token"))
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
