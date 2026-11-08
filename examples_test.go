package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/gate/domain"
	gateconfig "github.com/jorge07/harness/internal/gate/infrastructure/config"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
)

const (
	examplesDir       = "examples"
	exampleConfig     = ".harness.yaml"
	exampleCompose    = "compose.yaml"
	exampleDockerfile = "Dockerfile"
	hostGatePrefix    = "harness "
	proposalSeparator = "\n"
)

var (
	imageLine   = regexp.MustCompile(`(?m)^\s*(?:image:|FROM)\s+(\S+)`)
	pinnedImage = regexp.MustCompile(`^[a-z0-9./_-]+:\d+\.\d+\.\d+[a-z0-9._-]*@sha256:[0-9a-f]{64}$`)
)

type example struct {
	name     string
	lockfile string
	setup    string
	detected []string
}

var examples = []example{
	{name: "symfony", lockfile: "composer.lock", setup: "make-install", detected: []string{"make-cs", "make-phpstan", "make-lint-container", "make-test"}},
	{name: "go", lockfile: "go.sum", setup: "go-mod"},
	{name: "nextjs", lockfile: "pnpm-lock.yaml", setup: "node-install"},
	{name: "python", lockfile: "uv.lock", setup: "py-install"},
}

func TestExamples(t *testing.T) {
	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join(examplesDir, ex.name))
			if err != nil {
				t.Fatal(err)
			}
			cfg := loadExampleConfig(t, dir)
			plugins, err := execplugin.FromConfig(cfg.Plugins)
			if err != nil {
				t.Fatal(err)
			}
			setup, gates := decodeExample(t, cfg.Setup, cfg.Gates)
			if ex.detected != nil {
				if len(setup) > 0 || len(gates) > 0 {
					t.Fatalf("%s has a setup or gates key, so harness gates init refuses to write the detected steps", exampleConfig)
				}
				setup, gates = detectGates(t, dir, plugins)
				ids := make([]string, len(gates))
				for i, gate := range gates {
					ids[i] = gate.ID
				}
				if !slices.Equal(ids, ex.detected) {
					t.Errorf("detected gates = %q, want %q", ids, ex.detected)
				}
			}
			if len(setup) != 1 || setup[0].ID != ex.setup {
				t.Fatalf("setup = %+v, want the one step %s", setup, ex.setup)
			}
			if !slices.ContainsFunc(gates, func(gate domain.Gate) bool { return len(gate.Needs) > 0 }) {
				t.Error("no gate has needs, so the tests do not wait for the fast checks")
			}
			if cfg.Verify.Parallel < 2 {
				t.Errorf("verify.parallel = %d, want 2 or more", cfg.Verify.Parallel)
			}
			imageInputs := []string{exampleCompose, ex.lockfile}
			if _, err := os.Stat(filepath.Join(dir, exampleDockerfile)); err == nil {
				imageInputs = append(imageInputs, exampleDockerfile)
			}
			for _, path := range imageInputs {
				if !slices.Contains(setup[0].Paths, path) {
					t.Errorf("setup step %s runs in the container but its paths omit %s", ex.setup, path)
				}
			}
			for _, gate := range gates {
				if gate.ID == ex.setup {
					t.Errorf("gate %s repeats the setup step", gate.ID)
				}
				if strings.HasPrefix(gate.Run, hostGatePrefix) {
					continue
				}
				for _, path := range imageInputs {
					if !slices.Contains(gate.Paths, path) {
						t.Errorf("gate %s runs in the container but its paths omit %s", gate.ID, path)
					}
				}
			}
			checkImagesPinned(t, dir)
		})
	}
}

func decodeExample(t *testing.T, setupCfg []config.Setup, gatesCfg []config.Gate) ([]domain.Setup, []domain.Gate) {
	t.Helper()
	setup, err := gateconfig.DecodeSetup(setupCfg)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := gateconfig.Decode(gatesCfg)
	if err != nil {
		t.Fatal(err)
	}
	return setup, gates
}

func loadExampleConfig(t *testing.T, dir string) config.Config {
	t.Helper()
	flags := pflag.NewFlagSet(exampleConfig, pflag.ContinueOnError)
	flags.String(config.FlagConfig, filepath.Join(dir, exampleConfig), "")
	cfg, err := config.Load(flags, dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func detectGates(t *testing.T, dir string, plugins []shareddomain.Plugin) ([]domain.Setup, []domain.Gate) {
	t.Helper()
	payload, err := json.Marshal(shareddomain.GatesDetectPayload{Files: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	var proposedSetup []config.Setup
	var proposed []config.Gate
	for _, plugin := range plugins {
		if !slices.Contains(plugin.Subscribes(), shareddomain.EventGatesDetect) {
			continue
		}
		verdict, err := plugin.Handle(context.Background(), shareddomain.Envelope{
			Version: shareddomain.EnvelopeVersion,
			Event:   shareddomain.EventGatesDetect,
			Cwd:     dir,
			Payload: payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		if verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("plugin %s verdict = %q, want %q", plugin.Name(), verdict.Kind, shareddomain.VerdictAllow)
		}
		for line := range strings.SplitSeq(verdict.Context, proposalSeparator) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var gate shareddomain.ProposedGate
			if err := json.Unmarshal([]byte(line), &gate); err != nil {
				t.Fatalf("plugin %s line %q: %v", plugin.Name(), line, err)
			}
			if gate.Setup {
				proposedSetup = append(proposedSetup, config.Setup{ID: gate.ID, Run: gate.Run, Paths: gate.Paths, Creates: gate.Creates})
				continue
			}
			proposed = append(proposed, config.Gate{ID: gate.ID, Run: gate.Run, Paths: gate.Paths, Needs: gate.Needs})
		}
	}
	return decodeExample(t, proposedSetup, proposed)
}

func checkImagesPinned(t *testing.T, dir string) {
	t.Helper()
	var images []string
	for _, name := range []string{exampleCompose, exampleDockerfile} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) && name == exampleDockerfile {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range imageLine.FindAllStringSubmatch(string(raw), -1) {
			images = append(images, m[1])
		}
	}
	if len(images) == 0 {
		t.Fatalf("no image in %s or %s", exampleCompose, exampleDockerfile)
	}
	for _, image := range images {
		if !pinnedImage.MatchString(image) {
			t.Errorf("image %s is not name:x.y.z@sha256:<digest>", image)
		}
	}
}
