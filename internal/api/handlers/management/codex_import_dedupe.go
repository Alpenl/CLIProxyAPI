package management

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/codex"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func (h *Handler) codexDuplicateReason(data []byte, metadata map[string]any) (string, bool) {
	if h == nil || h.authManager == nil {
		return "", false
	}

	incomingEmail := strings.TrimSpace(stringMetadata(metadata, "email"))
	incomingAccountID := strings.TrimSpace(codexAccountIDFromMetadata(metadata))
	incomingHash := codexCanonicalHash(metadata)

	for _, auth := range h.authManager.List() {
		if auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			continue
		}

		switch {
		case incomingEmail != "" && strings.EqualFold(incomingEmail, authEmail(auth)):
			return fmt.Sprintf("duplicate email already exists in %s", auth.FileName), true
		case incomingAccountID != "" && strings.EqualFold(incomingAccountID, codexAccountIDFromAuth(auth)):
			return fmt.Sprintf("duplicate account already exists in %s", auth.FileName), true
		case incomingHash != "" && incomingHash == codexAuthHash(auth):
			return fmt.Sprintf("duplicate credentials already exists in %s", auth.FileName), true
		}
	}

	return "", false
}

func codexCanonicalHash(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func codexAuthHash(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if len(auth.Metadata) > 0 {
		if hash := codexCanonicalHash(auth.Metadata); hash != "" {
			return hash
		}
	}
	path := strings.TrimSpace(authAttribute(auth, "path"))
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var metadata map[string]any
	if err = json.Unmarshal(data, &metadata); err != nil {
		return ""
	}
	return codexCanonicalHash(metadata)
}

func codexAccountIDFromAuth(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	return codexAccountIDFromMetadata(auth.Metadata)
}

func codexAccountIDFromMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	if value, ok := metadata["account_id"].(string); ok {
		return strings.TrimSpace(value)
	}
	if raw, ok := metadata["id_token"].(string); ok {
		claims, err := codex.ParseJWTToken(strings.TrimSpace(raw))
		if err == nil && claims != nil {
			return strings.TrimSpace(claims.GetAccountID())
		}
	}
	return ""
}
