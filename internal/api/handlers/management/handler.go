// Package management provides the management API handlers and middleware
// for configuring the server and managing auth files.
package management

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"golang.org/x/crypto/bcrypt"
)

type attemptInfo struct {
	count        int
	blockedUntil time.Time
	lastActivity time.Time // track last activity for cleanup
}

// attemptCleanupInterval controls how often stale IP entries are purged
const attemptCleanupInterval = 1 * time.Hour

// attemptMaxIdleTime controls how long an IP can be idle before cleanup
const attemptMaxIdleTime = 2 * time.Hour

// Handler aggregates config reference, persistence path and helpers.
type Handler struct {
	cfg                 *config.Config
	configFilePath      string
	mu                  sync.Mutex
	attemptsMu          sync.Mutex
	cacheMu             sync.Mutex
	failedAttempts      map[string]*attemptInfo // keyed by client IP
	authManager         *coreauth.Manager
	usageStats          *usage.RequestStatistics
	tokenStore          coreauth.Store
	codexRefresher      func(context.Context, *coreauth.Auth) (*coreauth.Auth, error)
	localPassword       string
	allowRemoteOverride bool
	envSecret           string
	logDir              string
	postAuthHook        coreauth.PostAuthHook
	replenishmentStatus func(context.Context) (ReplenishmentStatus, error)
	replenishmentRun    func(context.Context) (ReplenishmentStatus, error)
	now                 func() time.Time
	codexAccountsCache  codexAccountsCacheEntry
	overviewCache       overviewCacheEntry
}

const codexAccountsCacheTTL = time.Second
const overviewCacheTTL = time.Second

type codexAccountsCacheEntry struct {
	entries   []gin.H
	expiresAt time.Time
	ready     bool
}

type overviewCacheEntry struct {
	payload   gin.H
	expiresAt time.Time
	ready     bool
}

// NewHandler creates a new management handler instance.
func NewHandler(cfg *config.Config, configFilePath string, manager *coreauth.Manager) *Handler {
	envSecret, _ := os.LookupEnv("MANAGEMENT_PASSWORD")
	envSecret = strings.TrimSpace(envSecret)

	h := &Handler{
		cfg:                 cfg,
		configFilePath:      configFilePath,
		failedAttempts:      make(map[string]*attemptInfo),
		authManager:         manager,
		usageStats:          usage.GetRequestStatistics(),
		tokenStore:          sdkAuth.GetTokenStore(),
		allowRemoteOverride: envSecret != "",
		envSecret:           envSecret,
		now:                 time.Now,
	}
	h.startAttemptCleanup()
	return h
}

// startAttemptCleanup launches a background goroutine that periodically
// removes stale IP entries from failedAttempts to prevent memory leaks.
func (h *Handler) startAttemptCleanup() {
	go func() {
		ticker := time.NewTicker(attemptCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			h.purgeStaleAttempts()
		}
	}()
}

// purgeStaleAttempts removes IP entries that have been idle beyond attemptMaxIdleTime
// and whose ban (if any) has expired.
func (h *Handler) purgeStaleAttempts() {
	now := time.Now()
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()
	for ip, ai := range h.failedAttempts {
		// Skip if still banned
		if !ai.blockedUntil.IsZero() && now.Before(ai.blockedUntil) {
			continue
		}
		// Remove if idle too long
		if now.Sub(ai.lastActivity) > attemptMaxIdleTime {
			delete(h.failedAttempts, ip)
		}
	}
}

// NewHandler creates a new management handler instance.
func NewHandlerWithoutConfigFilePath(cfg *config.Config, manager *coreauth.Manager) *Handler {
	return NewHandler(cfg, "", manager)
}

// SetConfig updates the in-memory config reference when the server hot-reloads.
func (h *Handler) SetConfig(cfg *config.Config) {
	h.cfg = cfg
	h.invalidateCodexAccountsCache()
}

// SetAuthManager updates the auth manager reference used by management endpoints.
func (h *Handler) SetAuthManager(manager *coreauth.Manager) {
	h.authManager = manager
	h.invalidateCodexAccountsCache()
}

// SetUsageStatistics allows replacing the usage statistics reference.
func (h *Handler) SetUsageStatistics(stats *usage.RequestStatistics) {
	h.usageStats = stats
	h.invalidateOverviewCache()
}

// SetLocalPassword configures the runtime-local password accepted for localhost requests.
func (h *Handler) SetLocalPassword(password string) { h.localPassword = password }

// SetLogDirectory updates the directory where main.log should be looked up.
func (h *Handler) SetLogDirectory(dir string) {
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	h.logDir = dir
}

// SetPostAuthHook registers a hook to be called after auth record creation but before persistence.
func (h *Handler) SetPostAuthHook(hook coreauth.PostAuthHook) {
	h.postAuthHook = hook
}

// SetCodexRefresher overrides the default Codex refresh behavior, primarily for tests.
func (h *Handler) SetCodexRefresher(fn func(context.Context, *coreauth.Auth) (*coreauth.Auth, error)) {
	h.codexRefresher = fn
}

// SetReplenishmentCallbacks wires server-owned replenishment status and manual run actions.
func (h *Handler) SetReplenishmentCallbacks(
	statusFn func(context.Context) (ReplenishmentStatus, error),
	runFn func(context.Context) (ReplenishmentStatus, error),
) {
	h.replenishmentStatus = statusFn
	h.replenishmentRun = runFn
	h.invalidateOverviewCache()
}

func (h *Handler) invalidateCodexAccountsCache() {
	if h == nil {
		return
	}
	h.cacheMu.Lock()
	h.codexAccountsCache = codexAccountsCacheEntry{}
	h.overviewCache = overviewCacheEntry{}
	h.cacheMu.Unlock()
}

func (h *Handler) invalidateOverviewCache() {
	if h == nil {
		return
	}
	h.cacheMu.Lock()
	h.overviewCache = overviewCacheEntry{}
	h.cacheMu.Unlock()
}

func (h *Handler) loadCodexAccountsCache() ([]gin.H, bool) {
	if h == nil || h.now == nil {
		return nil, false
	}
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if !h.codexAccountsCache.ready || h.codexAccountsCache.expiresAt.IsZero() {
		return nil, false
	}
	if !h.now().Before(h.codexAccountsCache.expiresAt) {
		h.codexAccountsCache = codexAccountsCacheEntry{}
		return nil, false
	}
	return cloneCodexAccountsEntries(h.codexAccountsCache.entries), true
}

func (h *Handler) storeCodexAccountsCache(entries []gin.H) {
	if h == nil || h.now == nil {
		return
	}
	h.cacheMu.Lock()
	h.codexAccountsCache = codexAccountsCacheEntry{
		entries:   cloneCodexAccountsEntries(entries),
		expiresAt: h.now().Add(codexAccountsCacheTTL),
		ready:     true,
	}
	h.cacheMu.Unlock()
}

func (h *Handler) loadOverviewCache() (gin.H, bool) {
	if h == nil || h.now == nil {
		return nil, false
	}
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if !h.overviewCache.ready || h.overviewCache.expiresAt.IsZero() {
		return nil, false
	}
	if !h.now().Before(h.overviewCache.expiresAt) {
		h.overviewCache = overviewCacheEntry{}
		return nil, false
	}
	return cloneGinMap(h.overviewCache.payload), true
}

func (h *Handler) storeOverviewCache(payload gin.H) {
	if h == nil || h.now == nil {
		return
	}
	h.cacheMu.Lock()
	h.overviewCache = overviewCacheEntry{
		payload:   cloneGinMap(payload),
		expiresAt: h.now().Add(overviewCacheTTL),
		ready:     true,
	}
	h.cacheMu.Unlock()
}

func cloneCodexAccountsEntries(entries []gin.H) []gin.H {
	if entries == nil {
		return nil
	}
	cloned := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			cloned = append(cloned, nil)
			continue
		}
		cloned = append(cloned, cloneGinMap(entry))
	}
	return cloned
}

func cloneGinMap(src gin.H) gin.H {
	if src == nil {
		return nil
	}
	dst := make(gin.H, len(src))
	for key, value := range src {
		dst[key] = cloneJSONLikeValue(value)
	}
	return dst
}

func cloneJSONLikeValue(value any) any {
	switch typed := value.(type) {
	case gin.H:
		return cloneGinMap(typed)
	case map[string]any:
		dst := make(map[string]any, len(typed))
		for key, nested := range typed {
			dst[key] = cloneJSONLikeValue(nested)
		}
		return dst
	case []any:
		dst := make([]any, len(typed))
		for i, nested := range typed {
			dst[i] = cloneJSONLikeValue(nested)
		}
		return dst
	case []gin.H:
		return cloneCodexAccountsEntries(typed)
	case []map[string]any:
		dst := make([]map[string]any, len(typed))
		for i, nested := range typed {
			dst[i] = cloneGinMap(gin.H(nested))
		}
		return dst
	default:
		return value
	}
}

// Middleware enforces access control for management endpoints.
// All requests (local and remote) require a valid management key.
// Additionally, remote access requires allow-remote-management=true.
func (h *Handler) Middleware() gin.HandlerFunc {
	const maxFailures = 5
	const banDuration = 30 * time.Minute

	return func(c *gin.Context) {
		c.Header("X-CPA-VERSION", buildinfo.Version)
		c.Header("X-CPA-COMMIT", buildinfo.Commit)
		c.Header("X-CPA-BUILD-DATE", buildinfo.BuildDate)

		clientIP := c.ClientIP()
		localClient := clientIP == "127.0.0.1" || clientIP == "::1"
		cfg := h.cfg
		var (
			allowRemote bool
			secretHash  string
		)
		if cfg != nil {
			allowRemote = cfg.RemoteManagement.AllowRemote
			secretHash = cfg.RemoteManagement.SecretKey
		}
		if cfg == nil || cfg.BootstrapRequired() {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "bootstrap required"})
			return
		}
		if h.allowRemoteOverride {
			allowRemote = true
		}
		envSecret := h.envSecret

		fail := func() {}
		if !localClient {
			h.attemptsMu.Lock()
			ai := h.failedAttempts[clientIP]
			if ai != nil {
				if !ai.blockedUntil.IsZero() {
					if time.Now().Before(ai.blockedUntil) {
						remaining := time.Until(ai.blockedUntil).Round(time.Second)
						h.attemptsMu.Unlock()
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("IP banned due to too many failed attempts. Try again in %s", remaining)})
						return
					}
					// Ban expired, reset state
					ai.blockedUntil = time.Time{}
					ai.count = 0
				}
			}
			h.attemptsMu.Unlock()

			if !allowRemote {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management disabled"})
				return
			}

			fail = func() {
				h.attemptsMu.Lock()
				aip := h.failedAttempts[clientIP]
				if aip == nil {
					aip = &attemptInfo{}
					h.failedAttempts[clientIP] = aip
				}
				aip.count++
				aip.lastActivity = time.Now()
				if aip.count >= maxFailures {
					aip.blockedUntil = time.Now().Add(banDuration)
					aip.count = 0
				}
				h.attemptsMu.Unlock()
			}
		}
		if secretHash == "" && envSecret == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management key not set"})
			return
		}

		// Accept either Authorization: Bearer <key> or X-Management-Key
		var provided string
		if ah := c.GetHeader("Authorization"); ah != "" {
			parts := strings.SplitN(ah, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				provided = parts[1]
			} else {
				provided = ah
			}
		}
		if provided == "" {
			provided = c.GetHeader("X-Management-Key")
		}

		if provided == "" {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing management key"})
			return
		}

		if localClient {
			if lp := h.localPassword; lp != "" {
				if subtle.ConstantTimeCompare([]byte(provided), []byte(lp)) == 1 {
					c.Next()
					return
				}
			}
		}

		if envSecret != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(envSecret)) == 1 {
			if !localClient {
				h.attemptsMu.Lock()
				if ai := h.failedAttempts[clientIP]; ai != nil {
					ai.count = 0
					ai.blockedUntil = time.Time{}
				}
				h.attemptsMu.Unlock()
			}
			c.Next()
			return
		}

		if secretHash == "" || bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(provided)) != nil {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid management key"})
			return
		}

		if !localClient {
			h.attemptsMu.Lock()
			if ai := h.failedAttempts[clientIP]; ai != nil {
				ai.count = 0
				ai.blockedUntil = time.Time{}
			}
			h.attemptsMu.Unlock()
		}

		c.Next()
	}
}

// persist saves the current in-memory config to disk.
func (h *Handler) persist(c *gin.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Preserve comments when writing
	if err := config.SaveConfigPreserveComments(h.configFilePath, h.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save config: %v", err)})
		return false
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
	return true
}

// Helper methods for simple types
func (h *Handler) updateBoolField(c *gin.Context, set func(bool)) {
	var body struct {
		Value *bool `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateIntField(c *gin.Context, set func(int)) {
	var body struct {
		Value *int `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateStringField(c *gin.Context, set func(string)) {
	var body struct {
		Value *string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}
