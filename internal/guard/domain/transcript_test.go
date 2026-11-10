package domain

import (
	"os"
	"slices"
	"testing"
)

const (
	transcriptRoot  = "/work/repo"
	transcriptModel = "claude-haiku-5-5"
)

func TestTranscriptFilesRead(t *testing.T) {
	body, err := os.ReadFile("testdata/reviewer-transcript.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	changed := []string{"a.go", "b.go", "c.go", "d.go", "f.go", "sub/e.go", "g.go"}

	model, read := FilesRead(body, transcriptRoot, changed)

	if model != transcriptModel {
		t.Errorf("model = %q, want %q", model, transcriptModel)
	}
	want := []string{"a.go", "b.go", "c.go", "f.go", "sub/e.go"}
	if !slices.Equal(read, want) {
		t.Errorf("read = %v, want %v (a Read call, a string diff, a Bash command, an array diff, a relative Read)", read, want)
	}

	if model, read := FilesRead([]byte("not json\n{}\n"), transcriptRoot, changed); model != "" || len(read) != 0 {
		t.Errorf("no valid line: model = %q, read = %v, want none", model, read)
	}
}

func TestTranscriptFilesReadNumberedDiff(t *testing.T) {
	body, err := os.ReadFile("testdata/numbered-diff-transcript.jsonl")
	if err != nil {
		t.Fatal(err)
	}

	_, read := FilesRead(body, transcriptRoot, []string{"a.go", "b.go", "c.go"})

	if want := []string{"a.go", "b.go"}; !slices.Equal(read, want) {
		t.Errorf("read = %v, want %v (a numbered header counts, a header inside text does not)", read, want)
	}
}

func TestTranscriptFilesReadUnpaddedNumbers(t *testing.T) {
	body, err := os.ReadFile("testdata/unpadded-diff-transcript.jsonl")
	if err != nil {
		t.Fatal(err)
	}

	_, read := FilesRead(body, transcriptRoot, []string{"a.go", "b.go"})

	if want := []string{"a.go"}; !slices.Equal(read, want) {
		t.Errorf("read = %v, want %v (a header after a number with no padding counts)", read, want)
	}
}

func TestListedModelFor(t *testing.T) {
	listed := []string{"sonnet", "haiku", "claude-opus-5-5"}
	cases := []struct {
		id   string
		want string
		ok   bool
	}{
		{transcriptModel, "haiku", true},
		{"claude-opus-5-5", "claude-opus-5-5", true},
		{"claude-sonnet-5-5", "sonnet", true},
		{"claude-fable-5", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := ListedModelFor(listed, c.id)
		if got != c.want || ok != c.ok {
			t.Errorf("ListedModelFor(%q) = %q, %v, want %q, %v", c.id, got, ok, c.want, c.ok)
		}
	}
}
