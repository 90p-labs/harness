// Package webkit holds the web assets that pages share: the font faces,
// the mark colour tokens and the mermaid bundle. Each one is ready to inline.
package webkit

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

//go:embed third_party/mermaid.min.js
var mermaidJS string

//go:embed assets/*.woff2
var fontFiles embed.FS

const (
	fontFaceFormat = "@font-face{font-family:'%s';src:url(data:font/woff2;base64,%s) format('woff2');font-weight:%s;font-display:swap}"
	tokensFormat   = ":root{--added:%s;--modified:%s;--deleted:%s}"
)

var markColor = regexp.MustCompile(`color:(#[0-9a-fA-F]{6})`)

var fontFaces = []struct{ file, family, weight string }{
	{"InstrumentSerif.woff2", "Instrument Serif", "400"},
	{"SourceSans3.woff2", "Source Sans 3", "200 900"},
	{"IBMPlexMono.woff2", "IBM Plex Mono", "400"},
	{"Excalifont.woff2", "Excalifont", "400"},
}

// Kit is the shared assets of an inline page.
type Kit struct {
	Fonts, Tokens template.CSS
	Mermaid       template.JS
}

// Inline gives the font faces, the mark tokens and mermaid for a self-contained page.
// The kit is built once and shared.
func Inline() (Kit, error) {
	return inline()
}

var inline = sync.OnceValues(func() (Kit, error) {
	fonts, err := fontCSS()
	if err != nil {
		return Kit{}, err
	}
	return Kit{Fonts: fonts, Tokens: tokenCSS(), Mermaid: template.JS(mermaidJS)}, nil
})

func fontCSS() (template.CSS, error) {
	var css strings.Builder
	for _, f := range fontFaces {
		raw, err := fontFiles.ReadFile("assets/" + f.file)
		if err != nil {
			return "", fmt.Errorf("read font %s: %w", f.file, err)
		}
		fmt.Fprintf(&css, fontFaceFormat, f.family, base64.StdEncoding.EncodeToString(raw), f.weight)
	}
	return template.CSS(css.String()), nil
}

func tokenCSS() template.CSS {
	color := func(mark shareddomain.Mark) string {
		return markColor.FindStringSubmatch(shareddomain.MarkStyles[mark])[1]
	}
	return template.CSS(fmt.Sprintf(tokensFormat,
		color(shareddomain.MarkNew), color(shareddomain.MarkChanged), color(shareddomain.MarkRemoved)))
}
