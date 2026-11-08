package domain

import "errors"

var (
	ErrMissingPack    = errors.New("missing required pack")
	ErrUnknownPack    = errors.New("unknown pack")
	ErrStaleLintFiles = errors.New("lint files are stale, run harness lint init")
	ErrEngineMissing  = errors.New("lint engine is missing")

	ErrOldBaseline = errors.New("the lint baseline is at " + OldBaselinePath + ": run git mv " + OldBaselinePath + " " + BaselinePath +
		", then set baseline: " + BaselinePath + " on the lint gate in .harness.yaml")

	ErrUnknownEngine       = errors.New("unknown lint engine")
	ErrEngineTaken         = errors.New("lint plugin name is taken by a default lint plugin")
	ErrPrepareWithoutCheck = errors.New("a lint plugin on lint.prepare needs lint.check")
	ErrEngineDenied        = errors.New("lint plugin denied")
	ErrBadFinding          = errors.New("lint plugin returned a bad finding")

	ErrInvalidPackName   = errors.New("invalid pack name")
	ErrPackNameTaken     = errors.New("pack name is taken")
	ErrRuleTaken         = errors.New("rule id or engine rule is taken")
	ErrScopeOverlap      = errors.New("two packs check one file for the same rule")
	ErrPackOutsideRepo   = errors.New("pack folder is outside the repository")
	ErrCustomPackEngine  = errors.New("a custom pack rule needs an engine other than harness")
	ErrPluginOutsidePack = errors.New("plugin path is outside the pack folder")
	ErrUnknownSeverity   = errors.New("rule severity must be error, warn or info")

	ErrNoManifest         = errors.New("no " + ManifestFile + ": run the lint-setup skill to write it")
	ErrUnknownRule        = errors.New("unknown rule id")
	ErrDuplicateRule      = errors.New("rule id is listed twice")
	ErrCanaryOrWaiver     = errors.New("a rule needs exactly one of canary or waived")
	ErrCanaryOutsideRepo  = errors.New("canary folder is outside the repository")
	ErrCanaryMissing      = errors.New("canary folder is missing")
	ErrCanaryNotFolder    = errors.New("canary is a file, not a folder")
	ErrCanaryRuleInactive = errors.New("the canary rule is not active: its pack is not in lint.packs, or lint.disabled names it")
	ErrOneProofForEngines = errors.New("a rule on several engines needs a canary or a waiver for each engine under engines")
	ErrNoPackKey          = errors.New("the pack key is missing")
	ErrShortWaiver        = errors.New("a waiver needs a reason of 3 words or more")
	ErrAllWaived          = errors.New("every listed rule is waived, so no canary proves a rule: give at least one rule a canary")
)
