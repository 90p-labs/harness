package hookcodec

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	pluginHooksDir  = "../../../../hooks/"
	installDir      = "../install/"
	commandHostFlag = " hook --host "
	pluginLauncher  = `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`
	pathLauncher    = "harness"
)

type hookFile struct {
	Hooks map[string][]struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func TestHookConfigsMatchCodec(t *testing.T) {
	cases := []struct {
		path     string
		codec    Codec
		launcher string
	}{
		{pluginHooksDir + "hooks.json", NewClaude(), pluginLauncher},
		{pluginHooksDir + "codex.json", NewCodex(), pluginLauncher},
		{installDir + "claude.json", NewClaude(), pathLauncher},
		{installDir + "codex.json", NewCodex(), pathLauncher},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			var file hookFile
			if err := json.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			var events []shareddomain.EventName
			for key, groups := range file.Hooks {
				for _, group := range groups {
					for _, hook := range group.Hooks {
						events = append(events, eventOfCommand(t, tc.codec, tc.launcher, key, hook.Command))
					}
				}
			}
			slices.Sort(events)
			want := tc.codec.Capabilities()
			slices.Sort(want)
			if !slices.Equal(events, want) {
				t.Errorf("events = %v, want %v", events, want)
			}
		})
	}
}

func eventOfCommand(t *testing.T, codec Codec, launcher, key, command string) shareddomain.EventName {
	t.Helper()
	program, args, found := strings.Cut(command, commandHostFlag)
	host, name, ok := strings.Cut(args, " ")
	if !found || !ok || program != launcher || shareddomain.Host(host) != codec.Host() {
		t.Fatalf("%s: command %q does not call the hook of host %s", key, command, codec.Host())
	}
	hook, err := domain.ParseHookName(name)
	if err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	if eventNames[hook] != key {
		t.Errorf("hook %s is under %s, want %s", hook, key, eventNames[hook])
	}
	event, _ := domain.EventOf(hook)
	return event
}
