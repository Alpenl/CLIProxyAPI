package management

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

type webConfigManagementPayload struct {
	AllowRemote      bool   `json:"allowRemote"`
	SecretKey        string `json:"secretKey"`
	SecretConfigured bool   `json:"secretConfigured"`
}

type webConfigTLSPayload struct {
	Enable bool   `json:"enable"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
}

type webConfigReplenishmentPayload struct {
	Enabled                     bool   `json:"enabled"`
	TargetAccountCount          int    `json:"targetAccountCount"`
	CheckIntervalSeconds        int    `json:"checkIntervalSeconds"`
	QuotaRefreshIntervalSeconds int    `json:"quotaRefreshIntervalSeconds"`
	CleanupInvalidAccounts      bool   `json:"cleanupInvalidAccounts"`
	ServiceURL                  string `json:"serviceUrl"`
	ServiceToken                string `json:"serviceToken"`
	ServiceTokenConfigured      bool   `json:"serviceTokenConfigured"`
}

type webConfigPayload struct {
	Host                   string                        `json:"host"`
	Port                   int                           `json:"port"`
	TLS                    webConfigTLSPayload           `json:"tls"`
	Management             webConfigManagementPayload    `json:"management"`
	Replenishment          webConfigReplenishmentPayload `json:"replenishment"`
	AuthDir                string                        `json:"authDir"`
	APIKeys                []string                      `json:"apiKeys"`
	ProxyURL               string                        `json:"proxyUrl"`
	LoggingToFile          bool                          `json:"loggingToFile"`
	LogsMaxTotalSizeMB     int                           `json:"logsMaxTotalSizeMB"`
	ErrorLogsMaxFiles      int                           `json:"errorLogsMaxFiles"`
	UsageStatisticsEnabled bool                          `json:"usageStatisticsEnabled"`
	RequestLog             bool                          `json:"requestLog"`
	DisableCooling         bool                          `json:"disableCooling"`
	RequestRetry           int                           `json:"requestRetry"`
	MaxRetryCredentials    int                           `json:"maxRetryCredentials"`
	MaxRetryInterval       int                           `json:"maxRetryInterval"`
	RoutingStrategy        string                        `json:"routingStrategy"`
}

func isZeroReplenishmentPayload(payload webConfigReplenishmentPayload) bool {
	return !payload.Enabled &&
		payload.TargetAccountCount == 0 &&
		payload.CheckIntervalSeconds == 0 &&
		payload.QuotaRefreshIntervalSeconds == 0 &&
		!payload.CleanupInvalidAccounts &&
		strings.TrimSpace(payload.ServiceURL) == "" &&
		strings.TrimSpace(payload.ServiceToken) == ""
}

func (h *Handler) GetBootstrapStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"bootstrap_required": h.bootstrapRequired(),
		"config":             h.webConfigView(),
	})
}

func (h *Handler) UpdateBootstrapConfig(c *gin.Context) {
	if !h.bootstrapRequired() {
		c.JSON(http.StatusConflict, gin.H{"error": "bootstrap already completed"})
		return
	}
	h.updateWebConfig(c, true)
}

func (h *Handler) GetWebConfig(c *gin.Context) {
	if h.bootstrapRequired() {
		c.JSON(http.StatusConflict, gin.H{"error": "bootstrap required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"config":             h.webConfigView(),
		"bootstrap_required": false,
	})
}

func (h *Handler) UpdateWebConfig(c *gin.Context) {
	if h.bootstrapRequired() {
		c.JSON(http.StatusConflict, gin.H{"error": "bootstrap required"})
		return
	}
	h.updateWebConfig(c, false)
}

func (h *Handler) updateWebConfig(c *gin.Context, requireSecret bool) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	var body webConfigPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	nextCfg, restartFields, err := h.mergeWebConfig(body, requireSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	h.cfg = nextCfg
	saveErr := config.SaveConfigPreserveComments(h.configFilePath, h.cfg)
	h.mu.Unlock()
	if saveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save config: %v", saveErr)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"config":             h.webConfigView(),
		"bootstrap_required": h.bootstrapRequired(),
		"restart_required":   len(restartFields) > 0,
		"restart_fields":     restartFields,
	})
}

func (h *Handler) mergeWebConfig(body webConfigPayload, requireSecret bool) (*config.Config, []string, error) {
	current := h.cfg
	if current == nil {
		current = config.DefaultBootstrapConfig()
	}

	nextCfg := *current
	restartFields := make([]string, 0, 3)

	host := strings.TrimSpace(body.Host)
	port := body.Port
	if port <= 0 || port > 65535 {
		return nil, nil, fmt.Errorf("port must be between 1 and 65535")
	}
	if host != current.Host {
		restartFields = append(restartFields, "host")
	}
	if port != current.Port {
		restartFields = append(restartFields, "port")
	}
	nextCfg.Host = host
	nextCfg.Port = port

	tlsCert := strings.TrimSpace(body.TLS.Cert)
	tlsKey := strings.TrimSpace(body.TLS.Key)
	if body.TLS.Enable && (tlsCert == "" || tlsKey == "") {
		return nil, nil, fmt.Errorf("tls enabled requires both cert and key")
	}
	if body.TLS.Enable != current.TLS.Enable || tlsCert != strings.TrimSpace(current.TLS.Cert) || tlsKey != strings.TrimSpace(current.TLS.Key) {
		restartFields = appendUniqueString(restartFields, "tls")
	}
	nextCfg.TLS = config.TLSConfig{
		Enable: body.TLS.Enable,
		Cert:   tlsCert,
		Key:    tlsKey,
	}

	authDir := strings.TrimSpace(body.AuthDir)
	if authDir == "" {
		return nil, nil, fmt.Errorf("authDir is required")
	}
	nextCfg.AuthDir = authDir

	apiKeys := normalizeNonEmptyStrings(body.APIKeys)
	nextCfg.APIKeys = apiKeys

	secret := strings.TrimSpace(body.Management.SecretKey)
	if requireSecret && secret == "" {
		return nil, nil, fmt.Errorf("management secret is required")
	}
	if secret != "" {
		hashedSecret, err := config.HashManagementSecret(secret)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to hash management secret: %w", err)
		}
		nextCfg.RemoteManagement.SecretKey = hashedSecret
	}
	nextCfg.RemoteManagement.AllowRemote = body.Management.AllowRemote

	if body.LogsMaxTotalSizeMB < 0 {
		return nil, nil, fmt.Errorf("logsMaxTotalSizeMB must be >= 0")
	}
	if body.ErrorLogsMaxFiles < 0 {
		return nil, nil, fmt.Errorf("errorLogsMaxFiles must be >= 0")
	}
	if body.RequestRetry < 0 {
		return nil, nil, fmt.Errorf("requestRetry must be >= 0")
	}
	if body.MaxRetryCredentials < 0 {
		return nil, nil, fmt.Errorf("maxRetryCredentials must be >= 0")
	}
	if body.MaxRetryInterval < 0 {
		return nil, nil, fmt.Errorf("maxRetryInterval must be >= 0")
	}

	strategy := normalizeRoutingStrategy(body.RoutingStrategy)
	if strategy == "" {
		return nil, nil, fmt.Errorf("routingStrategy must be round-robin or fill-first")
	}

	nextCfg.ProxyURL = strings.TrimSpace(body.ProxyURL)
	nextCfg.LoggingToFile = body.LoggingToFile
	nextCfg.LogsMaxTotalSizeMB = body.LogsMaxTotalSizeMB
	nextCfg.ErrorLogsMaxFiles = body.ErrorLogsMaxFiles
	nextCfg.UsageStatisticsEnabled = body.UsageStatisticsEnabled
	nextCfg.RequestLog = body.RequestLog
	nextCfg.DisableCooling = body.DisableCooling
	nextCfg.RequestRetry = body.RequestRetry
	nextCfg.MaxRetryCredentials = body.MaxRetryCredentials
	nextCfg.MaxRetryInterval = body.MaxRetryInterval
	nextCfg.Routing.Strategy = strategy

	nextCfg.Replenishment = current.Replenishment
	if nextCfg.Replenishment.TargetAccountCount <= 0 {
		nextCfg.Replenishment = config.DefaultReplenishmentConfig()
	}

	if !isZeroReplenishmentPayload(body.Replenishment) {
		if body.Replenishment.TargetAccountCount <= 0 {
			return nil, nil, fmt.Errorf("replenishment.targetAccountCount must be >= 1")
		}
		if body.Replenishment.CheckIntervalSeconds <= 0 {
			return nil, nil, fmt.Errorf("replenishment.checkIntervalSeconds must be >= 1")
		}
		if body.Replenishment.QuotaRefreshIntervalSeconds <= 0 {
			return nil, nil, fmt.Errorf("replenishment.quotaRefreshIntervalSeconds must be >= 1")
		}

		serviceURL := strings.TrimSpace(body.Replenishment.ServiceURL)
		if serviceURL != "" {
			parsedURL, err := url.Parse(serviceURL)
			if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
				return nil, nil, fmt.Errorf("replenishment.serviceUrl must be a valid http(s) URL")
			}
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return nil, nil, fmt.Errorf("replenishment.serviceUrl must be a valid http(s) URL")
			}
		}

		nextCfg.Replenishment.Enabled = body.Replenishment.Enabled
		nextCfg.Replenishment.TargetAccountCount = body.Replenishment.TargetAccountCount
		nextCfg.Replenishment.CheckIntervalSeconds = body.Replenishment.CheckIntervalSeconds
		nextCfg.Replenishment.QuotaRefreshIntervalSeconds = body.Replenishment.QuotaRefreshIntervalSeconds
		nextCfg.Replenishment.CleanupInvalidAccounts = body.Replenishment.CleanupInvalidAccounts
		nextCfg.Replenishment.ServiceURL = serviceURL
		if token := strings.TrimSpace(body.Replenishment.ServiceToken); token != "" {
			nextCfg.Replenishment.ServiceToken = token
		}
	}

	return &nextCfg, restartFields, nil
}

func (h *Handler) webConfigView() webConfigPayload {
	cfg := h.cfg
	if cfg == nil {
		cfg = config.DefaultBootstrapConfig()
	}
	return webConfigPayload{
		Host: cfg.Host,
		Port: cfg.Port,
		TLS: webConfigTLSPayload{
			Enable: cfg.TLS.Enable,
			Cert:   cfg.TLS.Cert,
			Key:    cfg.TLS.Key,
		},
		Management: webConfigManagementPayload{
			AllowRemote:      cfg.RemoteManagement.AllowRemote,
			SecretConfigured: strings.TrimSpace(cfg.RemoteManagement.SecretKey) != "",
			SecretKey:        "",
		},
		Replenishment: webConfigReplenishmentPayload{
			Enabled:                     cfg.Replenishment.Enabled,
			TargetAccountCount:          cfg.Replenishment.TargetAccountCount,
			CheckIntervalSeconds:        cfg.Replenishment.CheckIntervalSeconds,
			QuotaRefreshIntervalSeconds: cfg.Replenishment.QuotaRefreshIntervalSeconds,
			CleanupInvalidAccounts:      cfg.Replenishment.CleanupInvalidAccounts,
			ServiceURL:                  cfg.Replenishment.ServiceURL,
			ServiceToken:                "",
			ServiceTokenConfigured:      strings.TrimSpace(cfg.Replenishment.ServiceToken) != "",
		},
		AuthDir:                cfg.AuthDir,
		APIKeys:                append([]string(nil), cfg.APIKeys...),
		ProxyURL:               cfg.ProxyURL,
		LoggingToFile:          cfg.LoggingToFile,
		LogsMaxTotalSizeMB:     cfg.LogsMaxTotalSizeMB,
		ErrorLogsMaxFiles:      cfg.ErrorLogsMaxFiles,
		UsageStatisticsEnabled: cfg.UsageStatisticsEnabled,
		RequestLog:             cfg.RequestLog,
		DisableCooling:         cfg.DisableCooling,
		RequestRetry:           cfg.RequestRetry,
		MaxRetryCredentials:    cfg.MaxRetryCredentials,
		MaxRetryInterval:       cfg.MaxRetryInterval,
		RoutingStrategy:        normalizeRoutingStrategy(cfg.Routing.Strategy),
	}
}

func (h *Handler) bootstrapRequired() bool {
	if h == nil || h.cfg == nil {
		return true
	}
	return h.cfg.BootstrapRequired()
}

func normalizeRoutingStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "round-robin", "roundrobin", "rr":
		return "round-robin"
	case "fill-first", "fillfirst", "ff":
		return "fill-first"
	default:
		return ""
	}
}

func appendUniqueString(values []string, value string) []string {
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

func normalizeNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
