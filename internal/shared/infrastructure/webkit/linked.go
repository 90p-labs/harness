package webkit

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

//go:embed third_party/htmx.min.js
var htmxJS []byte

//go:embed third_party/idiomorph-ext.min.js
var morphJS []byte

//go:embed third_party/htmx-ext-sse.min.js
var sseJS []byte

//go:embed live.css
var liveCSS []byte

//go:embed icon.svg
var iconSVG []byte

//go:embed panel.js
var panelJS []byte

const (
	assetPrefix    = "/assets/"
	linkedFontFace = "@font-face{font-family:'%s';src:url(" + assetPrefix + "%s/%s) format('woff2');font-weight:%s;font-display:swap}"
	jsType         = "text/javascript; charset=utf-8"
	fontType       = "font/woff2"
	cssType        = "text/css; charset=utf-8"
	svgType        = "image/svg+xml"
)

// Links is the shared assets of a served page. Each field names an asset by
// the content hash of the kit, so a browser may cache it for good.
type Links struct {
	Fonts, Tokens        template.CSS
	Mermaid, Htmx, Morph string
	Sse, Panel, Live     string
	Icon                 string
	// Hash is the kit hash that names every asset, for Asset.
	Hash string
}

type asset struct {
	body []byte
	typ  string
}

type served struct {
	hash   string
	files  map[string]asset
	links  Links
	failed error
}

// load builds the served set once: the kit hash covers every served file.
var load = sync.OnceValue(func() served {
	s := served{files: map[string]asset{}}
	names := []string{}
	add := func(name string, body []byte, typ string) {
		s.files[name] = asset{body, typ}
		names = append(names, name)
	}
	for _, f := range fontFaces {
		raw, err := fontFiles.ReadFile("assets/" + f.file)
		if err != nil {
			s.failed = fmt.Errorf("read font %s: %w", f.file, err)
			return s
		}
		add(f.file, raw, fontType)
	}
	add("mermaid.min.js", []byte(mermaidJS), jsType)
	add("htmx.min.js", htmxJS, jsType)
	add("idiomorph-ext.min.js", morphJS, jsType)
	add("htmx-ext-sse.min.js", sseJS, jsType)
	add("panel.js", panelJS, jsType)
	add("live.css", liveCSS, cssType)
	add("icon.svg", iconSVG, svgType)

	sum := sha256.New()
	for _, name := range names { // fixed order: fonts first, then the scripts
		fmt.Fprintf(sum, "%s\x00%d\x00", name, len(s.files[name].body))
		sum.Write(s.files[name].body)
	}
	s.hash = hex.EncodeToString(sum.Sum(nil))[:12]

	var css strings.Builder
	for _, f := range fontFaces {
		fmt.Fprintf(&css, linkedFontFace, f.family, s.hash, f.file, f.weight)
	}
	url := func(file string) string { return assetPrefix + s.hash + "/" + file }
	s.links = Links{
		Fonts:   template.CSS(css.String()),
		Tokens:  tokenCSS(),
		Mermaid: url("mermaid.min.js"),
		Htmx:    url("htmx.min.js"),
		Morph:   url("idiomorph-ext.min.js"),
		Sse:     url("htmx-ext-sse.min.js"),
		Panel:   url("panel.js"),
		Live:    url("live.css"),
		Icon:    url("icon.svg"),
		Hash:    s.hash,
	}
	return s
})

// Linked gives the shared assets of a served page, each named by the kit hash.
// It returns an error when an embedded asset could not be read.
func Linked() (Links, error) {
	s := load()
	if s.failed != nil {
		return Links{}, s.failed
	}
	return s.links, nil
}

// Asset gives a served file under the current kit hash. Any other hash or
// file, a path included, gives ok=false.
func Asset(hash, file string) (body []byte, contentType string, ok bool) {
	s := load()
	if s.failed != nil || hash != s.hash {
		return nil, "", false
	}
	a, found := s.files[file] // an exact name only: a path never matches
	if !found {
		return nil, "", false
	}
	return a.body, a.typ, true
}
