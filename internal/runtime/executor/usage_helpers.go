package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	"github.com/tidwall/gjson"
)

type usageReporter struct {
	provider    string
	model       string
	authID      string
	authIndex   string
	apiKey      string
	source      string
	requestedAt time.Time
	once        sync.Once
}

func newUsageReporter(ctx context.Context, provider, model string, auth *cliproxyauth.Auth) *usageReporter {
	apiKey := apiKeyFromContext(ctx)
	reporter := &usageReporter{
		provider:    provider,
		model:       model,
		requestedAt: time.Now(),
		apiKey:      apiKey,
		source:      resolveUsageSource(auth, apiKey),
	}
	if auth != nil {
		reporter.authID = auth.ID
		reporter.authIndex = auth.EnsureIndex()
	}
	return reporter
}

func (r *usageReporter) publish(ctx context.Context, detail usage.Detail) {
	r.publishWithOutcome(ctx, detail, false)
}

func (r *usageReporter) publishFailure(ctx context.Context) {
	r.publishWithOutcome(ctx, usage.Detail{}, true)
}

func (r *usageReporter) trackFailure(ctx context.Context, errPtr *error) {
	if r == nil || errPtr == nil {
		return
	}
	if *errPtr != nil {
		r.publishFailure(ctx)
	}
}

func (r *usageReporter) publishWithOutcome(ctx context.Context, detail usage.Detail, failed bool) {
	if r == nil {
		return
	}
	if detail.TotalTokens == 0 {
		total := detail.InputTokens + detail.OutputTokens + detail.ReasoningTokens
		if total > 0 {
			detail.TotalTokens = total
		}
	}
	if detail.InputTokens == 0 && detail.OutputTokens == 0 && detail.ReasoningTokens == 0 && detail.CachedTokens == 0 && detail.TotalTokens == 0 && !failed {
		return
	}
	r.once.Do(func() {
		usage.PublishRecord(ctx, usage.Record{
			Provider:    r.provider,
			Model:       r.model,
			Source:      r.source,
			APIKey:      r.apiKey,
			AuthID:      r.authID,
			AuthIndex:   r.authIndex,
			RequestedAt: r.requestedAt,
			Failed:      failed,
			Detail:      detail,
		})
	})
}

// ensurePublished guarantees that a usage record is emitted exactly once.
// It is safe to call multiple times; only the first call wins due to once.Do.
// This is used to ensure request counting even when upstream responses do not
// include any usage fields (tokens), especially for streaming paths.
func (r *usageReporter) ensurePublished(ctx context.Context) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		usage.PublishRecord(ctx, usage.Record{
			Provider:    r.provider,
			Model:       r.model,
			Source:      r.source,
			APIKey:      r.apiKey,
			AuthID:      r.authID,
			AuthIndex:   r.authIndex,
			RequestedAt: r.requestedAt,
			Failed:      false,
			Detail:      usage.Detail{},
		})
	})
}

func apiKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ginCtx, ok := ctx.Value("gin").(*gin.Context)
	if !ok || ginCtx == nil {
		return ""
	}
	if v, exists := ginCtx.Get("apiKey"); exists {
		switch value := v.(type) {
		case string:
			return value
		case fmt.Stringer:
			return value.String()
		default:
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

func resolveUsageSource(auth *cliproxyauth.Auth, ctxAPIKey string) string {
	if auth != nil {
		if _, value := auth.AccountInfo(); value != "" {
			return strings.TrimSpace(value)
		}
		if auth.Metadata != nil {
			if email, ok := auth.Metadata["email"].(string); ok {
				if trimmed := strings.TrimSpace(email); trimmed != "" {
					return trimmed
				}
			}
		}
		if auth.Attributes != nil {
			if key := strings.TrimSpace(auth.Attributes["api_key"]); key != "" {
				return key
			}
		}
	}
	if trimmed := strings.TrimSpace(ctxAPIKey); trimmed != "" {
		return trimmed
	}
	return ""
}

func parseCodexUsage(data []byte) (usage.Detail, bool) {
	usageNode := gjson.ParseBytes(data).Get("response.usage")
	if !usageNode.Exists() {
		return usage.Detail{}, false
	}
	detail := usage.Detail{
		InputTokens:  usageNode.Get("input_tokens").Int(),
		OutputTokens: usageNode.Get("output_tokens").Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}
	if cached := usageNode.Get("input_tokens_details.cached_tokens"); cached.Exists() {
		detail.CachedTokens = cached.Int()
	}
	if reasoning := usageNode.Get("output_tokens_details.reasoning_tokens"); reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}
	return detail, true
}

func parseOpenAIUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data).Get("usage")
	if !usageNode.Exists() {
		return usage.Detail{}
	}

	inputNode := usageNode.Get("prompt_tokens")
	if !inputNode.Exists() {
		inputNode = usageNode.Get("input_tokens")
	}

	outputNode := usageNode.Get("completion_tokens")
	if !outputNode.Exists() {
		outputNode = usageNode.Get("output_tokens")
	}

	detail := usage.Detail{
		InputTokens:  inputNode.Int(),
		OutputTokens: outputNode.Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}

	cached := usageNode.Get("prompt_tokens_details.cached_tokens")
	if !cached.Exists() {
		cached = usageNode.Get("input_tokens_details.cached_tokens")
	}
	if cached.Exists() {
		detail.CachedTokens = cached.Int()
	}

	reasoning := usageNode.Get("completion_tokens_details.reasoning_tokens")
	if !reasoning.Exists() {
		reasoning = usageNode.Get("output_tokens_details.reasoning_tokens")
	}
	if reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}

	return detail
}
