package domain

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	lineAssistant   = "assistant"
	lineUser        = "user"
	blockToolUse    = "tool_use"
	blockToolResult = "tool_result"
	blockText       = "text"
	diffHeaderStart = "diff --git a/"
	diffHeaderSplit = " b/"
	modelIDSplit    = "-"
)

var lineNumberPrefix = regexp.MustCompile(`^ *\d+\t`)

type transcriptLine struct {
	Type    string `json:"type"`
	Message struct {
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type transcriptBlock struct {
	Type    string          `json:"type"`
	Name    string          `json:"name"`
	Text    string          `json:"text"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
}

type transcriptInput struct {
	FilePath string `json:"file_path"`
	Command  string `json:"command"`
}

// FilesRead returns the last assistant model id of a Claude subagent transcript and the changed files that it read.
func FilesRead(transcript []byte, root string, changed []string) (model string, read []string) {
	var readPaths, commands, diffHeaders []string
	for _, raw := range bytes.Split(transcript, []byte("\n")) {
		var line transcriptLine
		var blocks []transcriptBlock
		if json.Unmarshal(raw, &line) != nil || json.Unmarshal(line.Message.Content, &blocks) != nil {
			continue
		}
		if line.Type == lineAssistant && line.Message.Model != "" {
			model = line.Message.Model
		}
		for _, block := range blocks {
			switch {
			case line.Type == lineAssistant && block.Type == blockToolUse:
				var input transcriptInput
				if json.Unmarshal(block.Input, &input) != nil {
					continue
				}
				switch block.Name {
				case toolRead:
					readPaths = append(readPaths, relativeTo(root, input.FilePath))
				case toolBash:
					commands = append(commands, input.Command)
				}
			case line.Type == lineUser && block.Type == blockToolResult:
				diffHeaders = append(diffHeaders, headerLines(resultText(block.Content))...)
			}
		}
	}
	for _, file := range changed {
		if slices.Contains(readPaths, file) || slices.Contains(diffHeaders, diffHeaderStart+file+diffHeaderSplit+file) ||
			slices.ContainsFunc(commands, func(command string) bool { return strings.Contains(command, file) }) {
			read = append(read, file)
		}
	}
	return model, read
}

func resultText(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) == nil {
		return text
	}
	var parts []transcriptBlock
	if json.Unmarshal(content, &parts) != nil {
		return ""
	}
	var texts []string
	for _, part := range parts {
		if part.Type == blockText {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func headerLines(text string) []string {
	var headers []string
	for _, line := range strings.Split(text, "\n") {
		line = lineNumberPrefix.ReplaceAllString(line, "")
		if strings.HasPrefix(line, diffHeaderStart) {
			headers = append(headers, strings.TrimSpace(line))
		}
	}
	return headers
}

func ListedModelFor(listed []string, modelID string) (string, bool) {
	parts := strings.Split(modelID, modelIDSplit)
	for _, model := range listed {
		if model != "" && (model == modelID || slices.Contains(parts, model)) {
			return model, true
		}
	}
	return "", false
}

func relativeTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); filepath.IsAbs(path) && err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
