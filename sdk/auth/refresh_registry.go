package auth

import (
	"time"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func init() {
	cliproxyauth.RegisterRefreshLeadProvider("codex", func() *time.Duration {
		auth := NewCodexAuthenticator()
		return auth.RefreshLead()
	})
}
