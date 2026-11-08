package domain

import shareddomain "github.com/jorge07/harness/internal/shared/domain"

type EngineName string

const (
	EngineBiomeGrit EngineName = "biome-grit"
	EngineBiomeRule EngineName = "biome-rule"
	EngineOxlint    EngineName = "oxlint"
	EngineHarness   EngineName = "harness"
)

func (e EngineName) IsPlugin() bool {
	switch e {
	case EngineBiomeGrit, EngineBiomeRule, EngineOxlint, EngineHarness:
		return false
	}
	return true
}

type Rule struct {
	ID         string                `json:"id"`
	Engine     EngineName            `json:"engine"`
	Severity   shareddomain.Severity `json:"severity"`
	Scope      string                `json:"scope,omitempty"`
	Message    string                `json:"message,omitempty"`
	Plugin     string                `json:"plugin,omitempty"`
	BiomeRule  string                `json:"biome,omitempty"`
	OxlintRule string                `json:"oxlint,omitempty"`
	Reason     string                `json:"reason,omitempty"`
}
