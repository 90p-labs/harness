package domain

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	toolBash       = "Bash"
	toolEdit       = "Edit"
	toolMultiEdit  = "MultiEdit"
	toolWrite      = "Write"
	toolRead       = "Read"
	toolGrep       = "Grep"
	toolGlob       = "Glob"
	toolAgent      = "Agent"
	toolApplyPatch = "apply_patch"
)

const (
	patchAddFile    = "*** Add File: "
	patchUpdateFile = "*** Update File: "
	patchDeleteFile = "*** Delete File: "
	patchMoveTo     = "*** Move to: "
	patchAddedLine  = "+"
)

var (
	patchPathHeaders = []string{patchAddFile, patchUpdateFile, patchDeleteFile, patchMoveTo}
	inputTools       = []string{toolBash, toolEdit, toolMultiEdit, toolWrite, toolRead, toolGrep, toolGlob, toolAgent, toolApplyPatch}
)

type ToolCall struct {
	Kind      shareddomain.ToolKind
	Command   string
	Paths     []string
	NewText   string
	AgentType string
	Model     string
	Texts     [][]string
}

type toolInput struct {
	Command      string `json:"command"`
	FilePath     string `json:"file_path"`
	NewString    string `json:"new_string"`
	Content      string `json:"content"`
	Path         string `json:"path"`
	SubagentType string `json:"subagent_type"`
	Model        string `json:"model"`
	Edits        []struct {
		NewString string `json:"new_string"`
	} `json:"edits"`
}

func ToolCallOf(p shareddomain.ToolPayload) (ToolCall, error) {
	call := ToolCall{Kind: p.Kind}
	if !slices.Contains(inputTools, p.Tool) {
		return call, nil
	}
	var in toolInput
	if len(p.Input) > 0 {
		if err := json.Unmarshal(p.Input, &in); err != nil {
			return ToolCall{}, fmt.Errorf("decode %s input: %w", p.Tool, err)
		}
	}
	switch p.Tool {
	case toolBash:
		call.Command = in.Command
	case toolEdit:
		call.Paths, call.NewText = nonEmpty(in.FilePath), in.NewString
	case toolMultiEdit:
		texts := make([]string, len(in.Edits))
		for i, e := range in.Edits {
			texts[i] = e.NewString
		}
		call.Paths, call.NewText = nonEmpty(in.FilePath), strings.Join(texts, "\n")
	case toolWrite:
		call.Paths, call.NewText = nonEmpty(in.FilePath), in.Content
	case toolRead:
		call.Paths = nonEmpty(in.FilePath)
	case toolGrep, toolGlob:
		call.Paths = nonEmpty(in.Path)
	case toolAgent:
		call.AgentType, call.Model = in.SubagentType, in.Model
	case toolApplyPatch:
		call.Paths, call.NewText = parsePatch(in.Command)
	}
	return call, nil
}

func nonEmpty(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

func parsePatch(patch string) ([]string, string) {
	var paths, added []string
	for line := range strings.SplitSeq(patch, "\n") {
		if text, ok := strings.CutPrefix(line, patchAddedLine); ok {
			added = append(added, text)
			continue
		}
		for _, header := range patchPathHeaders {
			if path, ok := strings.CutPrefix(line, header); ok {
				paths = append(paths, path)
			}
		}
	}
	return paths, strings.Join(added, "\n")
}
