package domain_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const samplePatch = `*** Begin Patch
*** Add File: a/new.go
+package a
+
+func A() {}
*** Update File: b/old.go
@@
 context
-removed
+added
*** Delete File: c/gone.go
*** Update File: d/from.go
*** Move to: d/to.go
*** End Patch`

func TestToolCallOf(t *testing.T) {
	patchInput, err := json.Marshal(map[string]string{"command": samplePatch})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		tool    string
		kind    shareddomain.ToolKind
		input   string
		want    domain.ToolCall
		wantErr bool
	}{
		{
			name:  "Bash reads command",
			tool:  "Bash",
			kind:  shareddomain.ToolKindShell,
			input: `{"command":"git push origin main"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindShell, Command: "git push origin main"},
		},
		{
			name:  "Edit reads path and new string",
			tool:  "Edit",
			kind:  shareddomain.ToolKindEdit,
			input: `{"file_path":"/r/a_test.go","old_string":"x","new_string":"return 1"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindEdit, Paths: []string{"/r/a_test.go"}, NewText: "return 1"},
		},
		{
			name:  "MultiEdit joins new strings by newline",
			tool:  "MultiEdit",
			kind:  shareddomain.ToolKindEdit,
			input: `{"file_path":"/r/a.ts","edits":[{"old_string":"a","new_string":"one"},{"old_string":"b","new_string":"two"}]}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindEdit, Paths: []string{"/r/a.ts"}, NewText: "one\ntwo"},
		},
		{
			name:  "Write reads path and content",
			tool:  "Write",
			kind:  shareddomain.ToolKindWrite,
			input: `{"file_path":"/r/b.go","content":"package b"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindWrite, Paths: []string{"/r/b.go"}, NewText: "package b"},
		},
		{
			name:  "Read reads path",
			tool:  "Read",
			kind:  shareddomain.ToolKindRead,
			input: `{"file_path":"/r/.env"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindRead, Paths: []string{"/r/.env"}},
		},
		{
			name:  "Grep reads path",
			tool:  "Grep",
			kind:  shareddomain.ToolKindSearch,
			input: `{"pattern":"x","path":"/r/src"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindSearch, Paths: []string{"/r/src"}},
		},
		{
			name:  "Glob without path has no paths",
			tool:  "Glob",
			kind:  shareddomain.ToolKindSearch,
			input: `{"pattern":"**/*.go"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindSearch},
		},
		{
			name:  "Agent reads type and model",
			tool:  "Agent",
			kind:  shareddomain.ToolKindAgent,
			input: `{"subagent_type":"worker","model":"sonnet","prompt":"go"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindAgent, AgentType: "worker", Model: "sonnet"},
		},
		{
			name:  "apply_patch reads header paths and added lines",
			tool:  "apply_patch",
			kind:  shareddomain.ToolKindEdit,
			input: string(patchInput),
			want: domain.ToolCall{
				Kind:    shareddomain.ToolKindEdit,
				Paths:   []string{"a/new.go", "b/old.go", "c/gone.go", "d/from.go", "d/to.go"},
				NewText: "package a\n\nfunc A() {}\nadded",
			},
		},
		{
			name:  "unknown tool keeps only the kind",
			tool:  "WebFetch",
			kind:  shareddomain.ToolKindOther,
			input: `{"url":"https://x"}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindOther},
		},
		{
			name:  "unknown tool with list fields keeps only the kind",
			tool:  "mcp__x__run",
			kind:  shareddomain.ToolKindOther,
			input: `{"command":["ls"],"content":[{"type":"text"}]}`,
			want:  domain.ToolCall{Kind: shareddomain.ToolKindOther},
		},
		{
			name:    "malformed input is an error",
			tool:    "Bash",
			kind:    shareddomain.ToolKindShell,
			input:   `{"command":`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ToolCallOf(shareddomain.ToolPayload{Tool: tc.tool, Kind: tc.kind, Input: []byte(tc.input)})
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ToolCallOf: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestToolCallOfMCPName(t *testing.T) {
	patchInput, err := json.Marshal(map[string]string{"command": samplePatch})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tool, mcp string
		kind      shareddomain.ToolKind
		input     string
	}{
		{"Bash", "mcp__crawl_ostrich__life_Bash", shareddomain.ToolKindShell, `{"command":"git push origin main"}`},
		{"Edit", "mcp__crawl_ostrich__pilot_Edit", shareddomain.ToolKindEdit, `{"file_path":"/r/a_test.go","new_string":"return 1"}`},
		{"MultiEdit", "mcp__crawl_ostrich__pilot_MultiEdit", shareddomain.ToolKindEdit, `{"file_path":"/r/a.ts","edits":[{"new_string":"one"},{"new_string":"two"}]}`},
		{"Write", "mcp__crawl_ostrich__cradle_Write", shareddomain.ToolKindWrite, `{"file_path":"/r/b.go","content":"package b"}`},
		{"Read", "mcp__crawl_ostrich__pole_Read", shareddomain.ToolKindRead, `{"file_path":"/r/.env"}`},
		{"Grep", "mcp__crawl_ostrich__x_Grep", shareddomain.ToolKindSearch, `{"pattern":"x","path":"/r/src"}`},
		{"Glob", "mcp__crawl_ostrich__x_Glob", shareddomain.ToolKindSearch, `{"path":"/r/src"}`},
		{"Agent", "mcp__crawl_ostrich__verb_Agent", shareddomain.ToolKindAgent, `{"subagent_type":"worker","model":"sonnet"}`},
		{"apply_patch", "mcp__srv__apply_patch", shareddomain.ToolKindEdit, string(patchInput)},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			plain, err := domain.ToolCallOf(shareddomain.ToolPayload{Tool: tc.tool, Kind: tc.kind, Input: []byte(tc.input)})
			if err != nil {
				t.Fatalf("plain ToolCallOf: %v", err)
			}
			got, err := domain.ToolCallOf(shareddomain.ToolPayload{Tool: tc.mcp, Kind: tc.kind, Input: []byte(tc.input)})
			if err != nil {
				t.Fatalf("mcp ToolCallOf: %v", err)
			}
			if !reflect.DeepEqual(got, plain) {
				t.Fatalf("ToolCallOf(%s) = %+v, want %+v", tc.mcp, got, plain)
			}
		})
	}
}
