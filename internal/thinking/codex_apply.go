package thinking

import (
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func supportsThinkingFormat(format string) bool {
	switch format {
	case "codex", "openai-response":
		return true
	default:
		return false
	}
}

func applyThinkingConfig(body []byte, config ThinkingConfig, modelInfo *registry.ModelInfo, targetFormat string) ([]byte, error) {
	if !supportsThinkingFormat(targetFormat) {
		return body, nil
	}
	if IsUserDefinedModel(modelInfo) {
		return applyCompatibleCodex(body, config)
	}
	if modelInfo == nil || modelInfo.Thinking == nil {
		return body, nil
	}

	// Codex-compatible payloads only accept discrete effort levels.
	if config.Mode != ModeLevel && config.Mode != ModeNone {
		return body, nil
	}

	if len(body) == 0 || !gjson.ValidBytes(body) {
		body = []byte(`{}`)
	}

	if config.Mode == ModeLevel {
		result, _ := sjson.SetBytes(body, "reasoning.effort", string(config.Level))
		return result, nil
	}

	effort := ""
	support := modelInfo.Thinking
	if config.Budget == 0 {
		if support.ZeroAllowed || HasLevel(support.Levels, string(LevelNone)) {
			effort = string(LevelNone)
		}
	}
	if effort == "" && config.Level != "" {
		effort = string(config.Level)
	}
	if effort == "" && len(support.Levels) > 0 {
		effort = support.Levels[0]
	}
	if effort == "" {
		return body, nil
	}

	result, _ := sjson.SetBytes(body, "reasoning.effort", effort)
	return result, nil
}

func applyCompatibleCodex(body []byte, config ThinkingConfig) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		body = []byte(`{}`)
	}

	var effort string
	switch config.Mode {
	case ModeLevel:
		if config.Level == "" {
			return body, nil
		}
		effort = string(config.Level)
	case ModeNone:
		effort = string(LevelNone)
		if config.Level != "" {
			effort = string(config.Level)
		}
	case ModeAuto:
		effort = string(LevelAuto)
	case ModeBudget:
		level, ok := ConvertBudgetToLevel(config.Budget)
		if !ok {
			return body, nil
		}
		effort = level
	default:
		return body, nil
	}

	result, _ := sjson.SetBytes(body, "reasoning.effort", effort)
	return result, nil
}
