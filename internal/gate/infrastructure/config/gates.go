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
	keyGates    = "gates"
	fileMode    = 0o600
	yamlIndent  = 2
	nodeFirstEl = 0
)

func Decode(cfg []sharedconfig.Gate) ([]domain.Gate, error) {
	gates := make([]domain.Gate, 0, len(cfg))
	for _, c := range cfg {
		gate, err := domain.NewGate(c.ID, c.Run, c.Paths, c.Timeout, c.Baseline)
		if err != nil {
			return nil, err
		}
		gates = append(gates, gate)
	}
	if err := domain.ValidateGates(gates); err != nil {
		return nil, err
	}
	return gates, nil
}

type Writer struct{}

type gateEntry struct {
	ID       string   `yaml:"id"`
	Run      string   `yaml:"run"`
	Paths    []string `yaml:"paths,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Baseline string   `yaml:"baseline,omitempty"`
}

func (Writer) HasGates(dir string) (bool, error) {
	doc, err := readDocument(dir)
	if err != nil {
		return false, err
	}
	return gatesIndex(doc) >= 0, nil
}

func (Writer) WriteGates(dir string, gates []domain.Gate) error {
	doc, err := readDocument(dir)
	if err != nil {
		return err
	}
	if gatesIndex(doc) >= 0 {
		return domain.ErrGatesExist
	}

	entries := make([]gateEntry, len(gates))
	for i, gate := range gates {
		entries[i] = gateEntry{ID: gate.ID, Run: gate.Run, Paths: gate.Paths, Baseline: gate.Baseline}
		if gate.Timeout != domain.DefaultTimeout {
			entries[i].Timeout = gate.Timeout.Round(time.Millisecond).String()
		}
	}
	var seq yaml.Node
	if err := seq.Encode(entries); err != nil {
		return fmt.Errorf("encode gates: %w", err)
	}
	root := doc.Content[nodeFirstEl]
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: keyGates}, &seq)

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
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	if doc.Content[nodeFirstEl].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top level is not a mapping", fileName)
	}
	return &doc, nil
}

func gatesIndex(doc *yaml.Node) int {
	content := doc.Content[nodeFirstEl].Content
	for i := 0; i+1 < len(content); i += 2 {
		if content[i].Value == keyGates {
			return i
		}
	}
	return -1
}
