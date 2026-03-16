// Package registry provides model definitions and lookup helpers for Codex model tiers.
package registry

import "strings"

// staticModelsJSON mirrors the Codex sections of models.json.
type staticModelsJSON struct {
	CodexFree []*ModelInfo `json:"codex-free"`
	CodexTeam []*ModelInfo `json:"codex-team"`
	CodexPlus []*ModelInfo `json:"codex-plus"`
	CodexPro  []*ModelInfo `json:"codex-pro"`
}

// GetCodexFreeModels returns model definitions for the Codex free plan tier.
func GetCodexFreeModels() []*ModelInfo {
	return cloneModelInfos(getModels().CodexFree)
}

// GetCodexTeamModels returns model definitions for the Codex team plan tier.
func GetCodexTeamModels() []*ModelInfo {
	return cloneModelInfos(getModels().CodexTeam)
}

// GetCodexPlusModels returns model definitions for the Codex plus plan tier.
func GetCodexPlusModels() []*ModelInfo {
	return cloneModelInfos(getModels().CodexPlus)
}

// GetCodexProModels returns model definitions for the Codex pro plan tier.
func GetCodexProModels() []*ModelInfo {
	return cloneModelInfos(getModels().CodexPro)
}

func cloneModelInfos(models []*ModelInfo) []*ModelInfo {
	if len(models) == 0 {
		return nil
	}
	out := make([]*ModelInfo, len(models))
	for i, m := range models {
		out[i] = cloneModelInfo(m)
	}
	return out
}

// GetStaticModelDefinitionsByChannel returns static model definitions for a given channel/provider.
// Only the Codex channel remains supported.
func GetStaticModelDefinitionsByChannel(channel string) []*ModelInfo {
	if strings.EqualFold(strings.TrimSpace(channel), "codex") {
		return GetCodexProModels()
	}
	return nil
}

// LookupStaticModelInfo searches Codex static model definitions for a model by ID.
// Returns nil if no matching model is found.
func LookupStaticModelInfo(modelID string) *ModelInfo {
	if modelID == "" {
		return nil
	}

	data := getModels()
	allModels := [][]*ModelInfo{
		data.CodexFree,
		data.CodexTeam,
		data.CodexPlus,
		data.CodexPro,
	}
	for _, models := range allModels {
		for _, m := range models {
			if m != nil && m.ID == modelID {
				return cloneModelInfo(m)
			}
		}
	}

	return nil
}
