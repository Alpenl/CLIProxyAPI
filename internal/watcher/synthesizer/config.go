package synthesizer

import (
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// ConfigSynthesizer is retained as a no-op so the watcher flow can keep a
// single synthesis pipeline while config-backed credentials stay unsupported.
type ConfigSynthesizer struct{}

// NewConfigSynthesizer creates a new ConfigSynthesizer instance.
func NewConfigSynthesizer() *ConfigSynthesizer {
	return &ConfigSynthesizer{}
}

// Synthesize intentionally ignores config credential blocks. Imported auth
// files are the only supported credential source in the Codex-only runtime.
func (s *ConfigSynthesizer) Synthesize(ctx *SynthesisContext) ([]*coreauth.Auth, error) {
	return nil, nil
}
