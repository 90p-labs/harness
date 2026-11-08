package domain

import "errors"

const (
	ToolHelm     = "helm"
	ToolPromtool = "promtool"
)

var Tools = []string{ToolHelm, ToolPromtool}

var (
	ErrUnknownTool      = errors.New("unknown deploy plugin")
	ErrToolMissing      = errors.New("deploy tool is missing")
	ErrRendererConflict = errors.New("the helm plugin already renders the chart")
	ErrNoRenderer       = errors.New("no plugin rendered the chart")
)
