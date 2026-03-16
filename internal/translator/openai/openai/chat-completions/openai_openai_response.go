// Package chat_completions provides passthrough response translation for the
// proxy's OpenAI Chat Completions compatibility layer.
package chat_completions

import (
	"bytes"
	"context"
)

// ConvertOpenAIResponseToOpenAI normalizes one streaming chunk for the OpenAI
// Chat Completions response path.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and timeout handling
//   - modelName: The name of the model being used for the response (unused in current implementation)
//   - rawJSON: The raw upstream response chunk
//   - param: A pointer to a parameter object for maintaining state between calls
//
// Returns:
//   - []string: A slice of strings, each containing an OpenAI-compatible JSON response
func ConvertOpenAIResponseToOpenAI(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	if bytes.HasPrefix(rawJSON, []byte("data:")) {
		rawJSON = bytes.TrimSpace(rawJSON[5:])
	}
	if bytes.Equal(rawJSON, []byte("[DONE]")) {
		return []string{}
	}
	return []string{string(rawJSON)}
}

// ConvertOpenAIResponseToOpenAINonStream returns the normalized non-streaming
// response payload for the OpenAI Chat Completions path.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and timeout handling
//   - modelName: The name of the model being used for the response
//   - rawJSON: The raw upstream response payload
//   - param: A pointer to a parameter object for the conversion
//
// Returns:
//   - string: An OpenAI-compatible JSON response containing all message content and metadata
func ConvertOpenAIResponseToOpenAINonStream(ctx context.Context, modelName string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return string(rawJSON)
}
