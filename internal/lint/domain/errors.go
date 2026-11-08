package domain

import "errors"

var (
	ErrMissingPack    = errors.New("missing required pack")
	ErrUnknownPack    = errors.New("unknown pack")
	ErrStaleLintFiles = errors.New("lint files are stale, run harness lint init")
	ErrEngineMissing  = errors.New("lint engine is missing")

	ErrInvalidPackName   = errors.New("invalid pack name")
	ErrPackNameTaken     = errors.New("pack name is taken")
	ErrRuleTaken         = errors.New("rule id or engine rule is taken")
	ErrPackOutsideRepo   = errors.New("pack folder is outside the repository")
	ErrCustomPackEngine  = errors.New("a custom pack rule needs the engine biome-grit, biome-rule or oxlint")
	ErrPluginOutsidePack = errors.New("plugin path is outside the pack folder")
)
