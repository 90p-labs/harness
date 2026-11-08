package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/gate/domain"
	sharedconfig "github.com/jorge07/harness/internal/shared/infrastructure/config"
)

const (
	fileName    = ".harness.yaml"
	KeyGates    = "gates"
	KeySetup    = "setup"
	keyLint     = "lint"
	keyPacks    = "packs"
	tagString   = "!!str"
	tagMap      = "!!map"
	tagNull     = "!!null"
	fileMode    = 0o600
	yamlIndent  = 2
	nodeFirstEl = 0
)

var ErrLintNotMapping = errors.New("the lint key is not a mapping")

func Decode(cfg []sharedconfig.Gate) ([]domain.Gate, error) {
	gates := make([]domain.Gate, 0, len(cfg))
	for _, c := range cfg {
		gate, err := domain.NewGate(c.ID, c.Run, c.Paths, c.Timeout, c.Baseline)
		if err != nil {
			return nil, err
		}
		gate.Needs = c.Needs
		gates = append(gates, gate)
	}
	if err := domain.ValidateGates(gates); err != nil {
		return nil, err
	}
	return gates, nil
}

func DecodeSetup(cfg []sharedconfig.Setup) ([]domain.Setup, error) {
	steps := make([]domain.Setup, 0, len(cfg))
	for _, c := range cfg {
		step, err := domain.NewSetup(c.ID, c.Run, c.Paths, c.Creates)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if err := domain.ValidateSetup(steps); err != nil {
		return nil, err
	}
	return steps, nil
}

type Writer struct{}

type setupEntry struct {
	ID      string   `yaml:"id"`
	Run     string   `yaml:"run"`
	Paths   []string `yaml:"paths,omitempty"`
	Creates string   `yaml:"creates,omitempty"`
}

type gateEntry struct {
	ID       string   `yaml:"id"`
	Run      string   `yaml:"run"`
	Paths    []string `yaml:"paths,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Baseline string   `yaml:"baseline,omitempty"`
	Needs    []string `yaml:"needs,omitempty"`
}

func (Writer) BlockingKey(dir string) (string, error) {
	doc, err := readDocument(dir)
	if err != nil {
		return "", err
	}
	return blockingKey(doc.Content[nodeFirstEl]), nil
}

func blockingKey(root *yaml.Node) string {
	for _, key := range []string{KeyGates, KeySetup} {
		if i := keyIndex(root, key); i >= 0 && len(root.Content[i+1].Content) > 0 {
			return key
		}
	}
	return ""
}

func setKey(root *yaml.Node, key string, value *yaml.Node) {
	if i := keyIndex(root, key); i >= 0 {
		root.Content[i+1] = value
		return
	}
	root.Content = append(root.Content, keyNode(key), value)
}

func (Writer) WriteGates(dir string, setup []domain.Setup, gates []domain.Gate) error {
	doc, err := readDocument(dir)
	if err != nil {
		return err
	}
	root := doc.Content[nodeFirstEl]
	if blockingKey(root) != "" {
		return domain.ErrGatesExist
	}
	if len(setup) == 0 && len(gates) == 0 {
		return nil
	}
	if len(setup) > 0 {
		steps := make([]setupEntry, len(setup))
		for i, step := range setup {
			steps[i] = setupEntry{ID: step.ID, Run: step.Run, Paths: step.Paths, Creates: step.Creates}
		}
		var seq yaml.Node
		if err := seq.Encode(steps); err != nil {
			return fmt.Errorf("encode setup: %w", err)
		}
		setKey(root, KeySetup, &seq)
	}
	if len(gates) > 0 {
		entries := make([]gateEntry, len(gates))
		for i, gate := range gates {
			entries[i] = gateEntry{ID: gate.ID, Run: gate.Run, Paths: gate.Paths, Baseline: gate.Baseline, Needs: gate.Needs}
			if gate.Timeout != domain.DefaultTimeout {
				entries[i].Timeout = gate.Timeout.Round(time.Millisecond).String()
			}
		}
		var seq yaml.Node
		if err := seq.Encode(entries); err != nil {
			return fmt.Errorf("encode gates: %w", err)
		}
		setKey(root, KeyGates, &seq)
	}
	return writeDocument(dir, doc)
}

func (Writer) WriteLintPacks(dir string, packs []string) error {
	doc, err := readDocument(dir)
	if err != nil {
		return err
	}
	root := doc.Content[nodeFirstEl]
	lint := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	if i := keyIndex(root, keyLint); i < 0 {
		root.Content = append(root.Content, keyNode(keyLint), lint)
	} else {
		lint = root.Content[i+1]
	}
	if lint.Tag == tagNull {
		lint.Kind, lint.Tag, lint.Value = yaml.MappingNode, tagMap, ""
	}
	if lint.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: %w", fileName, ErrLintNotMapping)
	}
	var seq yaml.Node
	if err := seq.Encode(packs); err != nil {
		return fmt.Errorf("encode lint packs: %w", err)
	}
	if i := keyIndex(lint, keyPacks); i >= 0 {
		lint.Content[i+1] = &seq
	} else {
		lint.Content = append(lint.Content, keyNode(keyPacks), &seq)
	}
	return writeDocument(dir, doc)
}

func keyNode(key string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagString, Value: key}
}

func writeDocument(dir string, doc *yaml.Node) error {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(yamlIndent)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode %s: %w", fileName, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", fileName, err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), out.Bytes(), fileMode); err != nil {
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	return nil
}

func readDocument(dir string) (*yaml.Node, error) {
	raw, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", fileName, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", fileName, err)
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: tagMap}}}
	}
	if doc.Content[nodeFirstEl].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top level is not a mapping", fileName)
	}
	return &doc, nil
}

func keyIndex(mapping *yaml.Node, key string) int {
	content := mapping.Content
	for i := 0; i+1 < len(content); i += 2 {
		if content[i].Value == key {
			return i
		}
	}
	return -1
}
