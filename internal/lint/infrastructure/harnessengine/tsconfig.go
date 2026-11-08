package harnessengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/jorge07/harness/internal/lint/domain"
)

const (
	tsconfigFile   = "tsconfig.json"
	wildcard       = "*"
	currentFolder  = "."
	defaultBaseURL = currentFolder

	quote       = '"'
	escape      = '\\'
	newlineByte = '\n'
	objectClose = '}'
	arrayClose  = ']'
	jsonSpace   = " \t\r\n"
)

var (
	lineComment       = []byte("//")
	blockCommentOpen  = []byte("/*")
	blockCommentClose = []byte("*/")
	trailingComma     = []byte(",")
)

type tsconfig struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

type aliases struct {
	baseURL string
	paths   map[string][]string
}

func loadAliases(tree domain.Tree) (aliases, error) {
	if !slices.Contains(tree.Files, tsconfigFile) {
		return aliases{}, nil
	}
	raw, err := tree.Source(tsconfigFile)
	if err != nil {
		return aliases{}, err
	}
	var config tsconfig
	if err := json.Unmarshal(standardJSON(raw), &config); err != nil {
		return aliases{}, fmt.Errorf("decode %s: %w", tsconfigFile, err)
	}
	baseURL := config.CompilerOptions.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return aliases{baseURL: baseURL, paths: config.CompilerOptions.Paths}, nil
}

func standardJSON(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		switch {
		case raw[i] == quote:
			end := i + 1
			for end < len(raw) && raw[end] != quote {
				if raw[end] == escape {
					end++
				}
				end++
			}
			end = min(end, len(raw)-1)
			out = append(out, raw[i:end+1]...)
			i = end
		case bytes.HasPrefix(raw[i:], lineComment):
			for i < len(raw) && raw[i] != newlineByte {
				i++
			}
		case bytes.HasPrefix(raw[i:], blockCommentOpen):
			end := bytes.Index(raw[i+len(blockCommentOpen):], blockCommentClose)
			if end < 0 {
				return out
			}
			i += len(blockCommentOpen) + end + len(blockCommentClose) - 1
		case raw[i] == objectClose || raw[i] == arrayClose:
			out = bytes.TrimSuffix(bytes.TrimRight(out, jsonSpace), trailingComma)
			out = append(out, raw[i])
		default:
			out = append(out, raw[i])
		}
	}
	return out
}

func (a aliases) resolve(specifier string) ([]string, bool) {
	bestKey, bestPrefix, found := "", "", false
	for key := range a.paths {
		prefix, isWildcard := strings.CutSuffix(key, wildcard)
		matches := specifier == key || isWildcard && strings.HasPrefix(specifier, prefix)
		if matches && (!found || len(prefix) > len(bestPrefix)) {
			bestKey, bestPrefix, found = key, prefix, true
		}
	}
	if !found {
		return nil, false
	}
	rest := strings.TrimPrefix(specifier, bestPrefix)
	candidates := make([]string, 0, len(a.paths[bestKey]))
	for _, target := range a.paths[bestKey] {
		candidates = append(candidates, path.Join(a.baseURL, strings.Replace(target, wildcard, rest, 1)))
	}
	return candidates, true
}
