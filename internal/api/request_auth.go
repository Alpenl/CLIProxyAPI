package api

import (
	"net/http"
	"strings"
	"sync"
)

const inlineAccessProvider = "config-inline"

type requestAuthError struct {
	Message    string
	StatusCode int
}

type apiKeyAuthenticator struct {
	mu   sync.RWMutex
	keys map[string]struct{}
}

func newAPIKeyAuthenticator(keys []string) *apiKeyAuthenticator {
	auth := &apiKeyAuthenticator{}
	auth.SetKeys(keys)
	return auth
}

func (a *apiKeyAuthenticator) SetKeys(keys []string) {
	if a == nil {
		return
	}

	normalized := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		normalized[trimmed] = struct{}{}
	}

	a.mu.Lock()
	a.keys = normalized
	a.mu.Unlock()
}

func (a *apiKeyAuthenticator) Enabled() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.keys) > 0
}

func (a *apiKeyAuthenticator) Authenticate(r *http.Request) (string, string, map[string]string, *requestAuthError) {
	if a == nil || !a.Enabled() {
		return "", "", nil, nil
	}

	authHeader := ""
	authHeaderGoogle := ""
	authHeaderAnthropic := ""
	queryKey := ""
	queryAuthToken := ""
	if r != nil {
		authHeader = strings.TrimSpace(r.Header.Get("Authorization"))
		authHeaderGoogle = strings.TrimSpace(r.Header.Get("X-Goog-Api-Key"))
		authHeaderAnthropic = strings.TrimSpace(r.Header.Get("X-Api-Key"))
		if r.URL != nil {
			queryKey = strings.TrimSpace(r.URL.Query().Get("key"))
			queryAuthToken = strings.TrimSpace(r.URL.Query().Get("auth_token"))
		}
	}

	if authHeader == "" && authHeaderGoogle == "" && authHeaderAnthropic == "" && queryKey == "" && queryAuthToken == "" {
		return "", "", nil, &requestAuthError{
			Message:    "Missing API key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	candidates := []struct {
		value  string
		source string
	}{
		{extractBearerToken(authHeader), "authorization"},
		{authHeaderGoogle, "x-goog-api-key"},
		{authHeaderAnthropic, "x-api-key"},
		{queryKey, "query-key"},
		{queryAuthToken, "query-auth-token"},
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		if _, ok := a.keys[candidate.value]; ok {
			return candidate.value, inlineAccessProvider, map[string]string{"source": candidate.source}, nil
		}
	}

	return "", "", nil, &requestAuthError{
		Message:    "Invalid API key",
		StatusCode: http.StatusUnauthorized,
	}
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return header
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return header
	}
	return strings.TrimSpace(parts[1])
}
