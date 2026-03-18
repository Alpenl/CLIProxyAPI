package management

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/codex"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/synthesizer"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

var lastRefreshKeys = []string{"last_refresh", "lastRefresh", "last_refreshed_at", "lastRefreshedAt"}

func (h *Handler) buildAuthFileEntry(auth *coreauth.Auth) gin.H {
	if auth == nil {
		return nil
	}
	auth.EnsureIndex()
	runtimeOnly := isRuntimeOnlyAuth(auth)
	if runtimeOnly && (auth.Disabled || auth.Status == coreauth.StatusDisabled) {
		return nil
	}
	path := strings.TrimSpace(authAttribute(auth, "path"))
	if path == "" && !runtimeOnly {
		return nil
	}
	name := strings.TrimSpace(auth.FileName)
	if name == "" {
		name = auth.ID
	}
	entry := gin.H{
		"id":             auth.ID,
		"auth_index":     auth.Index,
		"name":           name,
		"type":           strings.TrimSpace(auth.Provider),
		"provider":       strings.TrimSpace(auth.Provider),
		"label":          auth.Label,
		"status":         auth.Status,
		"status_message": auth.StatusMessage,
		"disabled":       auth.Disabled,
		"unavailable":    auth.Unavailable,
		"runtime_only":   runtimeOnly,
		"source":         "memory",
		"size":           int64(0),
	}
	if email := authEmail(auth); email != "" {
		entry["email"] = email
	}
	if accountType, account := auth.AccountInfo(); accountType != "" || account != "" {
		if accountType != "" {
			entry["account_type"] = accountType
		}
		if account != "" {
			entry["account"] = account
		}
	}
	if !auth.CreatedAt.IsZero() {
		entry["created_at"] = auth.CreatedAt
	}
	if !auth.UpdatedAt.IsZero() {
		entry["modtime"] = auth.UpdatedAt
		entry["updated_at"] = auth.UpdatedAt
	}
	if !auth.LastRefreshedAt.IsZero() {
		entry["last_refresh"] = auth.LastRefreshedAt
	}
	if !auth.NextRetryAfter.IsZero() {
		entry["next_retry_after"] = auth.NextRetryAfter
	}
	if path != "" {
		entry["path"] = path
		entry["source"] = "file"
		if info, err := os.Stat(path); err == nil {
			entry["size"] = info.Size()
			entry["modtime"] = info.ModTime()
		} else if os.IsNotExist(err) {
			// Hide credentials removed from disk but still lingering in memory.
			if !runtimeOnly && (auth.Disabled || auth.Status == coreauth.StatusDisabled || strings.EqualFold(strings.TrimSpace(auth.StatusMessage), "removed via management api")) {
				return nil
			}
			entry["source"] = "memory"
		} else {
			log.WithError(err).Warnf("failed to stat auth file %s", path)
		}
	}
	if claims := extractCodexIDTokenClaims(auth); claims != nil {
		entry["id_token"] = claims
	}
	return entry
}

func extractCodexIDTokenClaims(auth *coreauth.Auth) gin.H {
	if auth == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return nil
	}

	result := gin.H{}
	if auth.Attributes != nil {
		if v := strings.TrimSpace(auth.Attributes["chatgpt_account_id"]); v != "" {
			result["chatgpt_account_id"] = v
		}
		if v := strings.TrimSpace(auth.Attributes["plan_type"]); v != "" {
			result["plan_type"] = v
		}
	}
	if len(result) == 0 && auth.Metadata != nil {
		if v, ok := auth.Metadata["account_id"].(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				result["chatgpt_account_id"] = trimmed
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	if auth.Metadata == nil {
		return nil
	}
	idTokenRaw, ok := auth.Metadata["id_token"].(string)
	if !ok {
		return nil
	}
	idToken := strings.TrimSpace(idTokenRaw)
	if idToken == "" {
		return nil
	}
	claims, err := codex.ParseJWTToken(idToken)
	if err != nil || claims == nil {
		return nil
	}

	result = gin.H{}
	if v := strings.TrimSpace(claims.CodexAuthInfo.ChatgptAccountID); v != "" {
		result["chatgpt_account_id"] = v
	}
	if v := strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType); v != "" {
		result["plan_type"] = v
	}
	if v := claims.CodexAuthInfo.ChatgptSubscriptionActiveStart; v != nil {
		result["chatgpt_subscription_active_start"] = v
	}
	if v := claims.CodexAuthInfo.ChatgptSubscriptionActiveUntil; v != nil {
		result["chatgpt_subscription_active_until"] = v
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func authEmail(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Metadata != nil {
		if v, ok := auth.Metadata["email"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	if auth.Attributes != nil {
		if v := strings.TrimSpace(auth.Attributes["email"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(auth.Attributes["account_email"]); v != "" {
			return v
		}
	}
	return ""
}

func authAttribute(auth *coreauth.Auth, key string) string {
	if auth == nil || len(auth.Attributes) == 0 {
		return ""
	}
	return auth.Attributes[key]
}

func isRuntimeOnlyAuth(auth *coreauth.Auth) bool {
	if auth == nil || len(auth.Attributes) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(auth.Attributes["runtime_only"]), "true")
}

func extractLastRefreshTimestamp(meta map[string]any) (time.Time, bool) {
	if len(meta) == 0 {
		return time.Time{}, false
	}
	for _, key := range lastRefreshKeys {
		if val, ok := meta[key]; ok {
			if ts, ok := parseLastRefreshValue(val); ok {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

func parseLastRefreshValue(v any) (time.Time, bool) {
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return time.Time{}, false
		}
		layouts := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
		for _, layout := range layouts {
			if ts, err := time.Parse(layout, s); err == nil {
				return ts.UTC(), true
			}
		}
		if unix, err := strconv.ParseInt(s, 10, 64); err == nil && unix > 0 {
			return time.Unix(unix, 0).UTC(), true
		}
	case float64:
		if val > 0 {
			return time.Unix(int64(val), 0).UTC(), true
		}
	case int64:
		if val > 0 {
			return time.Unix(val, 0).UTC(), true
		}
	case int:
		if val > 0 {
			return time.Unix(int64(val), 0).UTC(), true
		}
	case json.Number:
		if i, err := val.Int64(); err == nil && i > 0 {
			return time.Unix(i, 0).UTC(), true
		}
	}
	return time.Time{}, false
}

func (h *Handler) findAuthForDelete(name string) *coreauth.Auth {
	if h == nil || h.authManager == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if auth, ok := h.authManager.GetByID(name); ok {
		return auth
	}
	auths := h.authManager.List()
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if strings.TrimSpace(auth.FileName) == name {
			return auth
		}
		if filepath.Base(strings.TrimSpace(authAttribute(auth, "path"))) == name {
			return auth
		}
	}
	return nil
}

func (h *Handler) authIDForPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	id := path
	if h != nil && h.cfg != nil {
		authDir := strings.TrimSpace(h.cfg.AuthDir)
		if authDir != "" {
			if rel, errRel := filepath.Rel(authDir, path); errRel == nil && rel != "" {
				id = rel
			}
		}
	}
	if runtime.GOOS == "windows" {
		id = strings.ToLower(id)
	}
	return id
}

func populateAuthDerivedAttributes(authType string, metadata map[string]any, attr map[string]string) {
	if attr == nil {
		return
	}
	if email := stringMetadata(metadata, "email"); email != "" {
		attr["email"] = email
		attr["account_email"] = email
	}
	if !strings.EqualFold(strings.TrimSpace(authType), "codex") {
		return
	}
	if v := stringMetadata(metadata, "account_id"); v != "" {
		attr["chatgpt_account_id"] = v
	}
	idTokenRaw, _ := metadata["id_token"].(string)
	idToken := strings.TrimSpace(idTokenRaw)
	if idToken == "" {
		return
	}
	claims, err := codex.ParseJWTToken(idToken)
	if err != nil || claims == nil {
		return
	}
	if v := strings.TrimSpace(claims.CodexAuthInfo.ChatgptAccountID); v != "" {
		attr["chatgpt_account_id"] = v
	}
	if v := strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType); v != "" {
		attr["plan_type"] = v
	}
}

func (h *Handler) registerAuthFromFile(ctx context.Context, path string, data []byte) error {
	if h == nil {
		return fmt.Errorf("handler not initialized")
	}
	if h.authManager == nil {
		h.invalidateCodexAccountsCache()
		return nil
	}
	if path == "" {
		return fmt.Errorf("auth path is empty")
	}
	var err error
	if data == nil {
		data, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read auth file: %w", err)
		}
	}
	var auth *coreauth.Auth
	auth, err = h.synthesizeManagedAuth(path, data)
	if err != nil {
		return err
	}
	if existing, ok := h.authManager.GetByID(auth.ID); ok {
		auth.CreatedAt = existing.CreatedAt
		if auth.LastRefreshedAt.IsZero() {
			auth.LastRefreshedAt = existing.LastRefreshedAt
		}
		auth.NextRefreshAfter = existing.NextRefreshAfter
		auth.Runtime = existing.Runtime
		_, err = h.authManager.Update(ctx, auth)
		if err == nil {
			h.invalidateCodexAccountsCache()
			h.syncManagedAuthRuntime(auth)
		}
		return err
	}
	_, err = h.authManager.Register(ctx, auth)
	if err == nil {
		h.invalidateCodexAccountsCache()
		h.syncManagedAuthRuntime(auth)
	}
	return err
}

func (h *Handler) synthesizeManagedAuth(path string, data []byte) (*coreauth.Auth, error) {
	if h == nil || h.cfg == nil {
		return nil, fmt.Errorf("handler not initialized")
	}
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("invalid auth file: %w", err)
	}
	authType, _ := metadata["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(authType), "codex") {
		label := authType
		if email, ok := metadata["email"].(string); ok && email != "" {
			label = email
		}
		auth := &coreauth.Auth{
			ID:       h.authIDForPath(path),
			Provider: authType,
			FileName: filepath.Base(path),
			Label:    label,
			Status:   coreauth.StatusActive,
			Attributes: map[string]string{
				"path":   path,
				"source": path,
			},
			Metadata:  metadata,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		populateAuthDerivedAttributes(authType, metadata, auth.Attributes)
		if lastRefresh, ok := extractLastRefreshTimestamp(metadata); ok {
			auth.LastRefreshedAt = lastRefresh
		}
		return auth, nil
	}

	sctx := &synthesizer.SynthesisContext{
		Config:      h.cfg,
		AuthDir:     strings.TrimSpace(h.cfg.AuthDir),
		Now:         time.Now(),
		IDGenerator: synthesizer.NewStableIDGenerator(),
	}
	generated := synthesizer.SynthesizeAuthFile(sctx, path, data)
	if len(generated) == 0 || generated[0] == nil {
		return nil, fmt.Errorf("invalid auth file: unsupported auth payload")
	}
	auth := generated[0].Clone()
	auth.FileName = filepath.Base(path)
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	populateAuthDerivedAttributes(auth.Provider, auth.Metadata, auth.Attributes)
	if email := authEmail(auth); email != "" {
		auth.Attributes["email"] = email
		auth.Attributes["account_email"] = email
	}
	if lastRefresh, ok := extractLastRefreshTimestamp(auth.Metadata); ok {
		auth.LastRefreshedAt = lastRefresh
	}
	return auth, nil
}

func (h *Handler) syncManagedAuthRuntime(auth *coreauth.Auth) {
	if h == nil || h.authManager == nil || auth == nil || strings.TrimSpace(auth.ID) == "" {
		return
	}
	registerManagedAuthModels(auth)
	h.authManager.RefreshSchedulerEntry(auth.ID)
}

func registerManagedAuthModels(auth *coreauth.Auth) {
	if auth == nil || strings.TrimSpace(auth.ID) == "" {
		return
	}
	if auth.Disabled || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		registry.GetGlobalRegistry().UnregisterClient(auth.ID)
		return
	}

	var excluded []string
	if auth.Attributes != nil {
		if raw := strings.TrimSpace(auth.Attributes["excluded_models"]); raw != "" {
			excluded = strings.Split(raw, ",")
		}
	}

	planType := ""
	if auth.Attributes != nil {
		planType = strings.TrimSpace(auth.Attributes["plan_type"])
	}

	var models []*registry.ModelInfo
	switch strings.ToLower(planType) {
	case "free":
		models = registry.GetCodexFreeModels()
	case "team":
		models = registry.GetCodexTeamModels()
	case "plus":
		models = registry.GetCodexPlusModels()
	default:
		models = registry.GetCodexProModels()
	}

	models = filterManagedModels(models, excluded)
	models = prefixManagedModels(models, auth.Prefix)
	if len(models) == 0 {
		registry.GetGlobalRegistry().UnregisterClient(auth.ID)
		return
	}
	registry.GetGlobalRegistry().RegisterClient(auth.ID, models)
}

func filterManagedModels(models []*registry.ModelInfo, excluded []string) []*registry.ModelInfo {
	if len(models) == 0 || len(excluded) == 0 {
		return models
	}
	patterns := make([]string, 0, len(excluded))
	for _, item := range excluded {
		if trimmed := strings.ToLower(strings.TrimSpace(item)); trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}
	if len(patterns) == 0 {
		return models
	}
	filtered := make([]*registry.ModelInfo, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		modelID := strings.ToLower(strings.TrimSpace(model.ID))
		blocked := false
		for _, pattern := range patterns {
			if managedModelWildcardMatch(pattern, modelID) {
				blocked = true
				break
			}
		}
		if !blocked {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func prefixManagedModels(models []*registry.ModelInfo, prefix string) []*registry.ModelInfo {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" || len(models) == 0 {
		return models
	}

	out := make([]*registry.ModelInfo, 0, len(models)*2)
	seen := make(map[string]struct{}, len(models)*2)
	addModel := func(model *registry.ModelInfo) {
		if model == nil {
			return
		}
		id := strings.TrimSpace(model.ID)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		out = append(out, model)
	}

	for _, model := range models {
		if model == nil {
			continue
		}
		baseID := strings.TrimSpace(model.ID)
		if baseID == "" {
			continue
		}
		addModel(model)
		clone := *model
		clone.ID = trimmedPrefix + "/" + baseID
		addModel(&clone)
	}
	return out
}

func managedModelWildcardMatch(pattern, value string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	parts := strings.Split(pattern, "*")
	if prefix := parts[0]; prefix != "" {
		if !strings.HasPrefix(value, prefix) {
			return false
		}
		value = value[len(prefix):]
	}
	if suffix := parts[len(parts)-1]; suffix != "" {
		if !strings.HasSuffix(value, suffix) {
			return false
		}
		value = value[:len(value)-len(suffix)]
	}
	for i := 1; i < len(parts)-1; i++ {
		segment := parts[i]
		if segment == "" {
			continue
		}
		idx := strings.Index(value, segment)
		if idx < 0 {
			return false
		}
		value = value[idx+len(segment):]
	}
	return true
}

func (h *Handler) deleteTokenRecord(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("auth path is empty")
	}
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		return fmt.Errorf("token store unavailable")
	}
	return store.Delete(ctx, path)
}

func (h *Handler) tokenStoreWithBaseDir() coreauth.Store {
	if h == nil {
		return nil
	}
	store := h.tokenStore
	if store == nil {
		store = sdkAuth.GetTokenStore()
		h.tokenStore = store
	}
	if h.cfg != nil {
		if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(h.cfg.AuthDir)
		}
	}
	return store
}

func (h *Handler) saveTokenRecord(ctx context.Context, record *coreauth.Auth) (string, error) {
	if record == nil {
		return "", fmt.Errorf("token record is nil")
	}
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		return "", fmt.Errorf("token store unavailable")
	}
	if h.postAuthHook != nil {
		if err := h.postAuthHook(ctx, record); err != nil {
			return "", fmt.Errorf("post-auth hook failed: %w", err)
		}
	}
	return store.Save(ctx, record)
}
