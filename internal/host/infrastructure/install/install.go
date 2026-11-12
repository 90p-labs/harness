package install

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	keyHooks           = "hooks"
	harnessCommand     = "harness hook"
	dirMode            = 0o755
	fileMode           = 0o644
	jsonIndent         = "  "
	claudeSettingsDir  = ".claude"
	claudeSettingsFile = "settings.json"
	codexSettingsDir   = ".codex"
	codexSettingsFile  = "hooks.json"
)

var ErrUnsupportedHost = errors.New("unsupported host")

//go:embed claude.json
var claudeTemplate []byte

//go:embed claude_all.json
var claudeAllTemplate []byte

//go:embed codex.json
var codexTemplate []byte

type group map[string]json.RawMessage

// Install writes the hooks of the host. For Claude, matched picks the template whose tool hooks match only the rule tools.
func Install(worktree string, host shareddomain.Host, matched bool) (string, error) {
	path, template, err := targetOf(worktree, host, matched)
	if err != nil {
		return "", err
	}
	document, err := readDocument(path)
	if err != nil {
		return "", err
	}
	merged, err := mergeHooks(document, template)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	out, err := encode(merged, jsonIndent)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return "", err
	}
	if err := writeAtomic(path, out); err != nil {
		return "", err
	}
	return path, nil
}

// Uninstall removes the harness hook entries from the host config of the worktree and keeps every other hook. It
// drops the events and the hooks key that it leaves empty, and writes the file only when it removed an entry.
func Uninstall(worktree string, host shareddomain.Host) (string, int, error) {
	path, _, err := targetOf(worktree, host, false)
	if err != nil {
		return "", 0, err
	}
	document, err := readDocument(path)
	if err != nil {
		return "", 0, err
	}
	raw, ok := document[keyHooks]
	if !ok {
		return path, 0, nil
	}
	current, err := hooksOf(raw)
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", path, err)
	}
	total := 0
	for event, groups := range current {
		kept, removed, err := withoutHarness(groups)
		if err != nil {
			return "", 0, fmt.Errorf("%s: %w", path, err)
		}
		total += removed
		if len(kept) == 0 {
			delete(current, event)
			continue
		}
		current[event] = kept
	}
	if total == 0 {
		return path, 0, nil
	}
	if len(current) == 0 {
		delete(document, keyHooks)
	} else if document[keyHooks], err = encode(current, ""); err != nil {
		return "", 0, err
	}
	out, err := encode(document, jsonIndent)
	if err != nil {
		return "", 0, err
	}
	if err := writeAtomic(path, out); err != nil {
		return "", 0, err
	}
	return path, total, nil
}

func encode(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	err := encoder.Encode(v)
	return buf.Bytes(), err
}

func writeAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		return errors.Join(err, temp.Close())
	}
	if err := temp.Close(); err != nil {
		return err
	}
	mode := os.FileMode(fileMode)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func targetOf(worktree string, host shareddomain.Host, matched bool) (string, []byte, error) {
	switch host {
	case shareddomain.HostClaude:
		template := claudeAllTemplate
		if matched {
			template = claudeTemplate
		}
		return filepath.Join(worktree, claudeSettingsDir, claudeSettingsFile), template, nil
	case shareddomain.HostCodex:
		return filepath.Join(worktree, codexSettingsDir, codexSettingsFile), codexTemplate, nil
	default:
		return "", nil, fmt.Errorf("%w: %q", ErrUnsupportedHost, host)
	}
}

func readDocument(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if document == nil {
		document = map[string]json.RawMessage{}
	}
	return document, nil
}

func mergeHooks(document map[string]json.RawMessage, template []byte) (map[string]json.RawMessage, error) {
	var templateDocument map[string]json.RawMessage
	if err := json.Unmarshal(template, &templateDocument); err != nil {
		return nil, err
	}
	wanted, err := hooksOf(templateDocument[keyHooks])
	if err != nil {
		return nil, err
	}
	current, err := hooksOf(wrap(document[keyHooks]))
	if err != nil {
		return nil, err
	}
	for event, groups := range wanted {
		kept, _, err := withoutHarness(current[event])
		if err != nil {
			return nil, err
		}
		current[event] = append(kept, groups...)
	}
	encoded, err := encode(current, "")
	if err != nil {
		return nil, err
	}
	document[keyHooks] = encoded
	return document, nil
}

func wrap(hooks json.RawMessage) []byte {
	if hooks == nil {
		return []byte("{}")
	}
	return hooks
}

func hooksOf(raw []byte) (map[string][]group, error) {
	var hooks map[string][]group
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return nil, err
	}
	if hooks == nil {
		hooks = map[string][]group{}
	}
	return hooks, nil
}

// withoutHarness returns the groups without the harness hook entries, and the count of entries it removed.
func withoutHarness(groups []group) ([]group, int, error) {
	kept := []group{}
	removed := 0
	for _, g := range groups {
		raw, ok := g[keyHooks]
		if !ok {
			kept = append(kept, g)
			continue
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, 0, err
		}
		remaining := []json.RawMessage{}
		for _, entry := range entries {
			var parsed struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(entry, &parsed); err != nil {
				return nil, 0, err
			}
			if parsed.Command != harnessCommand && !strings.HasPrefix(parsed.Command, harnessCommand+" ") {
				remaining = append(remaining, entry)
			}
		}
		removed += len(entries) - len(remaining)
		if len(remaining) == 0 {
			continue
		}
		encoded, err := encode(remaining, "")
		if err != nil {
			return nil, 0, err
		}
		g[keyHooks] = encoded
		kept = append(kept, g)
	}
	return kept, removed, nil
}
