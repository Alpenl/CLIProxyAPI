package executor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func TestCodexExecutorExecuteStream_ConvertsUpstreamErrorEventToStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.created\"}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"error\",\"message\":\"Please try signing in again\",\"error\":{\"message\":\"Please try signing in again\"}}\n\n")
	}))
	defer server.Close()

	exec := &CodexExecutor{}
	auth := &cliproxyauth.Auth{
		ID:       "auth-1",
		Provider: "codex",
		Attributes: map[string]string{
			"api_key":  "sk-test",
			"base_url": server.URL,
		},
	}

	streamResult, err := exec.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model: "gpt-5.3-codex",
		Payload: []byte(`{
			"model":"gpt-5.3-codex",
			"instructions":"",
			"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]
		}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("codex"),
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	first, ok := <-streamResult.Chunks
	if !ok {
		t.Fatalf("expected bootstrap chunk")
	}
	if first.Err != nil {
		t.Fatalf("expected bootstrap payload before error, got err %v", first.Err)
	}

	var terminalErr error
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			terminalErr = chunk.Err
			break
		}
	}
	if terminalErr == nil {
		t.Fatalf("expected terminal error chunk")
	}

	statusProvider, ok := terminalErr.(interface{ StatusCode() int })
	if !ok {
		t.Fatalf("expected status error, got %T", terminalErr)
	}
	if got := statusProvider.StatusCode(); got != http.StatusUnauthorized {
		t.Fatalf("StatusCode() = %d, want %d", got, http.StatusUnauthorized)
	}
}
