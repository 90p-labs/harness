package harness

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	deploydomain "github.com/90p-labs/harness/internal/deploy/domain"
	gateconfig "github.com/90p-labs/harness/internal/gate/infrastructure/config"
	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

const (
	fullConfigPage = "docs/configuration-full.md"
	fullConfigLink = "configuration-full.md"
	fenceYAML      = "```yaml"
	fenceClose     = "```"
	fencePrefix    = "```"
	fullBlockCount = 1
	caseBlockCount = 11
	envPrefix      = "HARNESS_"
	envEntrySplit  = "="
	fullConfigName = "full.harness.yaml"
	folderPackMark = "/"
	rolesKey       = "roles"
)

var (
	fullConfigDocs = []string{configPage, "README.md", "DESIGN.md"}
	fullConfigRef  = regexp.MustCompile(`\]\([^)]*` + regexp.QuoteMeta(fullConfigLink) + `[^)]*\)`)
)

func yamlBlocks(page string) []string {
	var blocks []string
	var current []string
	inYAML, inOther := false, false
	for line := range strings.SplitSeq(page, "\n") {
		switch {
		case inYAML && line == fenceClose:
			blocks = append(blocks, strings.Join(current, "\n")+"\n")
			current, inYAML = nil, false
		case inYAML:
			current = append(current, line)
		case inOther:
			inOther = line != fenceClose
		case line == fenceYAML:
			inYAML = true
		case strings.HasPrefix(line, fencePrefix):
			inOther = true
		}
	}
	return blocks
}

func clearHarnessEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, envEntrySplit)
		if strings.HasPrefix(name, envPrefix) {
			t.Setenv(name, "")
		}
	}
}

func loadBlock(t *testing.T, block string) config.Config {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, fullConfigName)
	if err := os.WriteFile(file, []byte(block), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := pflag.NewFlagSet(fullConfigName, pflag.ContinueOnError)
	flags.String(config.FlagConfig, file, "")
	cfg, err := config.Load(flags, dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func guardIDs() []string {
	ids := []string{guarddomain.RuleStopHoldsTurn, guarddomain.RuleReviewModels}
	for _, rule := range guarddomain.Rules() {
		ids = append(ids, rule.ID)
	}
	return ids
}

func TestConfigurationFullBlocksLoad(t *testing.T) {
	clearHarnessEnv(t)
	blocks := yamlBlocks(readDoc(t, fullConfigPage))
	if want := fullBlockCount + caseBlockCount; len(blocks) != want {
		t.Fatalf("%s has %d yaml blocks, want %d", fullConfigPage, len(blocks), want)
	}
	for i, block := range blocks {
		cfg := loadBlock(t, block)
		if _, err := gateconfig.Decode(cfg.Gates); err != nil {
			t.Errorf("block %d gates: %v", i, err)
		}
		if _, err := gateconfig.DecodeSetup(cfg.Setup); err != nil {
			t.Errorf("block %d setup: %v", i, err)
		}
		var builtin []string
		for _, entry := range cfg.Lint.Packs {
			if !strings.Contains(entry, folderPackMark) {
				builtin = append(builtin, entry)
			}
		}
		if _, _, err := packs.Load(t.TempDir(), builtin); err != nil {
			t.Errorf("block %d packs %v: %v", i, builtin, err)
		}
		for _, id := range cfg.Guard.Disabled {
			if !slices.Contains(guardIDs(), id) {
				t.Errorf("block %d: guard.disabled has the unknown rule %s", i, id)
			}
		}
		for _, name := range cfg.Deploy.Disabled {
			if !slices.Contains(deploydomain.Tools, name) {
				t.Errorf("block %d: deploy.disabled has the unknown tool %s", i, name)
			}
		}
		for _, plugin := range cfg.Plugins {
			for _, event := range plugin.Events {
				if _, ok := shareddomain.KindOf(shareddomain.EventName(event)); !ok {
					t.Errorf("block %d: plugin %s has the unknown event %s", i, plugin.Name, event)
				}
			}
		}
	}
}

func hasPath(node any, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	switch value := node.(type) {
	case map[string]any:
		child, ok := value[parts[0]]
		return ok && hasPath(child, parts[1:])
	case []any:
		return slices.ContainsFunc(value, func(item any) bool { return hasPath(item, parts) })
	}
	return false
}

func TestConfigurationFullSetsEveryKey(t *testing.T) {
	blocks := yamlBlocks(readDoc(t, fullConfigPage))
	if len(blocks) == 0 {
		t.Fatalf("%s has no yaml block", fullConfigPage)
	}
	var full map[string]any
	if err := yaml.Unmarshal([]byte(blocks[0]), &full); err != nil {
		t.Fatal(err)
	}
	for _, path := range leafPaths(reflect.TypeFor[config.Config](), "") {
		if !hasPath(full, strings.Split(path, keySeparator)) {
			t.Errorf("the full block does not set `%s`", path)
		}
	}
	roles, _ := full[rolesKey].(map[string]any)
	hosts := map[string]bool{}
	for _, role := range []shareddomain.Role{shareddomain.RoleLead, shareddomain.RoleWorker, shareddomain.RoleReviewer, shareddomain.RoleExplorer, shareddomain.RoleAdvisor} {
		hostMap, ok := roles[string(role)].(map[string]any)
		if !ok {
			t.Errorf("the full block has no role %s", role)
		}
		for host := range hostMap {
			hosts[host] = true
		}
	}
	for _, host := range []shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex} {
		if !hosts[string(host)] {
			t.Errorf("the full block has no host %s", host)
		}
	}
}

func TestConfigurationFullPage(t *testing.T) {
	page := readDoc(t, fullConfigPage)
	if strings.Contains(page, emDash) || strings.ContainsFunc(page, isEmoji) {
		t.Errorf("%s has an em dash or an emoji", fullConfigPage)
	}
}

func TestConfigurationFullLinked(t *testing.T) {
	for _, doc := range fullConfigDocs {
		if !fullConfigRef.MatchString(readDoc(t, doc)) {
			t.Errorf("%s has no link to %s", doc, fullConfigLink)
		}
	}
}
