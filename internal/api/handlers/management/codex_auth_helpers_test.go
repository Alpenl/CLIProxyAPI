package management

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func createTestJWT(t *testing.T, payload map[string]any) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]string{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + ".signature"
}

func TestExtractCodexIDTokenClaims_FallsBackToMetadataAccountID(t *testing.T) {
	auth := &coreauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{
			"id_token":   "not-a-jwt",
			"account_id": "acct-from-metadata",
		},
	}

	claims := extractCodexIDTokenClaims(auth)
	if claims == nil {
		t.Fatalf("expected fallback claims, got nil")
	}
	if got := claims["chatgpt_account_id"]; got != "acct-from-metadata" {
		t.Fatalf("chatgpt_account_id = %#v, want acct-from-metadata", got)
	}
}

func TestPopulateAuthDerivedAttributes_FallsBackToMetadataAccountID(t *testing.T) {
	attr := map[string]string{}
	populateAuthDerivedAttributes("codex", map[string]any{
		"id_token":   "not-a-jwt",
		"account_id": "acct-from-metadata",
	}, attr)

	if got := attr["chatgpt_account_id"]; got != "acct-from-metadata" {
		t.Fatalf("chatgpt_account_id = %q, want acct-from-metadata", got)
	}
}

func TestPopulateAuthDerivedAttributes_PrefersJWTClaimsWhenAvailable(t *testing.T) {
	idToken := createTestJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct-from-jwt",
			"chatgpt_plan_type":  "free",
		},
	})

	attr := map[string]string{}
	populateAuthDerivedAttributes("codex", map[string]any{
		"id_token":   idToken,
		"account_id": "acct-from-metadata",
	}, attr)

	if got := attr["chatgpt_account_id"]; got != "acct-from-jwt" {
		t.Fatalf("chatgpt_account_id = %q, want acct-from-jwt", got)
	}
	if got := attr["plan_type"]; got != "free" {
		t.Fatalf("plan_type = %q, want free", got)
	}
}
