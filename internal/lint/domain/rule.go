package domain

import shareddomain "github.com/jorge07/harness/internal/shared/domain"

type EngineName string

const (
	EngineBiomeGrit EngineName = "biome-grit"
	EngineBiomeRule EngineName = "biome-rule"
	EngineOxlint    EngineName = "oxlint"
	EngineHarness   EngineName = "harness"
)

type Rule struct {
	ID         string
	Engine     EngineName
	Severity   shareddomain.Severity
	Scope      string
	Message    string
	Plugin     string
	BiomeRule  string
	OxlintRule string
	Reason     string
}
