package webkit

import (
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestKitStylesheet(t *testing.T) {
	css := string(Stylesheet())
	if strings.TrimSpace(css) == "" {
		t.Fatal("the kit stylesheet is empty")
	}
	for _, rule := range []string{".app", ".col", ".task"} {
		if !regexp.MustCompile(`(?m)(^|[\s,}])` + regexp.QuoteMeta(rule) + `[\s,{.:\[>]`).MatchString(css) {
			t.Errorf("the kit stylesheet has no %s rule", rule)
		}
	}
}

func TestInlineKit(t *testing.T) {
	kit, err := Inline()
	if err != nil {
		t.Fatalf("Inline: %v", err)
	}

	fonts := string(kit.Fonts)
	if got := strings.Count(fonts, "@font-face"); got != 4 {
		t.Fatalf("want 4 @font-face rules, got %d", got)
	}
	rule := regexp.MustCompile(`@font-face\{font-family:'([^']+)';src:url\(data:font/woff2;base64,([A-Za-z0-9+/=]+)\) format\('woff2'\);font-weight:([^;]+);font-display:swap\}`)
	rules := rule.FindAllStringSubmatch(fonts, -1)
	if len(rules) != 4 {
		t.Fatalf("want 4 well-formed rules, got %d", len(rules))
	}
	faces := []struct{ family, weight string }{
		{"Instrument Serif", "400"}, {"Source Sans 3", "200 900"}, {"IBM Plex Mono", "400"}, {"Excalifont", "400"},
	}
	for i, want := range faces {
		if rules[i][1] != want.family {
			t.Errorf("rule %d: want family %q, got %q", i, want.family, rules[i][1])
		}
		if rules[i][3] != want.weight {
			t.Errorf("rule %d: want weight %q, got %q", i, want.weight, rules[i][3])
		}
		raw, err := base64.StdEncoding.DecodeString(rules[i][2])
		if err != nil {
			t.Fatalf("rule %d: bad base64: %v", i, err)
		}
		if len(raw) < 4 || string(raw[:4]) != "wOF2" {
			t.Errorf("rule %d: data is not a woff2 file", i)
		}
	}

	tokens := regexp.MustCompile(`^:root\{--added:(#[0-9a-fA-F]{6});--modified:(#[0-9a-fA-F]{6});--deleted:(#[0-9a-fA-F]{6})\}$`).
		FindStringSubmatch(string(kit.Tokens))
	if tokens == nil {
		t.Fatalf("Tokens is not the :root rule: %s", kit.Tokens)
	}
	marks := []shareddomain.Mark{shareddomain.MarkNew, shareddomain.MarkChanged, shareddomain.MarkRemoved}
	for i, mark := range marks {
		if style := shareddomain.MarkStyles[mark]; !slices.Contains(strings.Split(style, ","), "color:"+tokens[i+1]) {
			t.Errorf("token %s is not the colour of mark %s: %s", tokens[i+1], mark, style)
		}
	}

	mermaid := string(kit.Mermaid)
	if len(mermaid) < 1<<20 || !strings.Contains(mermaid, "mermaid") {
		t.Errorf("Mermaid does not look like the mermaid bundle, %d bytes", len(mermaid))
	}
}
