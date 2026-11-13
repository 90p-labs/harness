package webkit

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVendoredChecksums(t *testing.T) {
	notice, err := os.ReadFile("NOTICE.md")
	if err != nil {
		t.Fatalf("read notice: %v", err)
	}
	row := regexp.MustCompile("(?m)^\\| `(third_party/[^`]+)` \\|.*\\| ([0-9a-f]{64}) \\|$")
	rows := row.FindAllStringSubmatch(string(notice), -1)
	seen := map[string]bool{}
	for _, r := range rows {
		raw, err := os.ReadFile(r[1])
		if err != nil {
			t.Errorf("read %s: %v", r[1], err)
			continue
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != r[2] {
			t.Errorf("%s: notice says %s, file is %s", r[1], r[2], got)
		}
		seen[r[1]] = true
	}
	for _, f := range []string{"third_party/mermaid.min.js", "third_party/htmx.min.js", "third_party/idiomorph-ext.min.js"} {
		if !seen[f] {
			t.Errorf("%s has no sha256 in the notice", f)
		}
	}
}

func TestLinkedAssets(t *testing.T) {
	links, err := Linked()
	if err != nil {
		t.Fatalf("Linked: %v", err)
	}
	url := regexp.MustCompile(`^/assets/([0-9a-f]{12})/([A-Za-z0-9._-]+)$`)

	var hash string
	scripts := map[string]string{links.Mermaid: "application/javascript", links.Htmx: "", links.Morph: "", links.Panel: ""}
	for u := range scripts {
		m := url.FindStringSubmatch(u)
		if m == nil {
			t.Fatalf("script link %q is not /assets/<hash>/<file>", u)
		}
		if hash != "" && m[1] != hash {
			t.Errorf("link %q uses another hash than %s", u, hash)
		}
		hash = m[1]
		body, ctype, ok := Asset(m[1], m[2])
		if !ok || len(body) == 0 {
			t.Errorf("Asset(%s, %s) not served", m[1], m[2])
		}
		if !strings.HasPrefix(ctype, "text/javascript") && !strings.HasPrefix(ctype, "application/javascript") {
			t.Errorf("Asset(%s) content type %q", m[2], ctype)
		}
	}

	fonts := regexp.MustCompile(`url\(/assets/([0-9a-f]{12})/([A-Za-z0-9._-]+\.woff2)\)`).FindAllStringSubmatch(string(links.Fonts), -1)
	if len(fonts) != 4 {
		t.Fatalf("want 4 linked fonts, got %d", len(fonts))
	}
	for _, f := range fonts {
		if f[1] != hash {
			t.Errorf("font %s uses hash %s, want %s", f[2], f[1], hash)
		}
		body, ctype, ok := Asset(f[1], f[2])
		if !ok || len(body) < 4 || string(body[:4]) != "wOF2" || ctype != "font/woff2" {
			t.Errorf("font %s not served as woff2 (ok=%v, type=%q)", f[2], ok, ctype)
		}
	}
	if !strings.Contains(string(links.Tokens), "--added:") {
		t.Errorf("Tokens is not the token rule: %s", links.Tokens)
	}

	panelFile := url.FindStringSubmatch(links.Panel)[2]
	body, _, _ := Asset(hash, panelFile)
	if !strings.Contains(string(body), "htmx:configRequest") {
		t.Errorf("panel.js does not listen to htmx:configRequest")
	}

	denied := [][2]string{
		{"000000000000", panelFile},
		{"", panelFile},
		{hash, "NOTICE.md"},
		{hash, "webkit.go"},
		{hash, "../NOTICE.md"},
		{hash, "../../webkit.go"},
		{hash, "assets/SourceSans3.woff2"},
		{hash, "third_party/htmx.min.js"},
		{hash, "/etc/passwd"},
		{hash, ""},
		{hash[:11], panelFile},
	}
	for _, d := range denied {
		if body, _, ok := Asset(d[0], d[1]); ok || body != nil {
			t.Errorf("Asset(%q, %q) must not be served", d[0], d[1])
		}
	}
}
