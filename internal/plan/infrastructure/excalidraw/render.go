package excalidraw

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

//go:embed rough.js
var roughSource string

const (
	kindRectangle = "rectangle"
	kindEllipse   = "ellipse"
	kindDiamond   = "diamond"
	kindLine      = "line"
	kindArrow     = "arrow"
	kindText      = "text"
	kindFreedraw  = "freedraw"
)

const (
	plainStroke       = "#1e1e1e"
	fontFamily        = "Excalifont"
	strokePrefix      = "stroke:"
	pathDecimals      = 2
	generatorScript   = "var g = rough.generator(); var opsToPath = g.opsToPath.bind(g); g.opsToPath = function (ops) { return opsToPath(ops, %d); };"
	defaultSeed       = 1
	defaultRoughness  = 1.0
	freedrawRoughness = 0.4
	minStrokeWidth    = 1.0
	hachureGap        = 8
	fillWeight        = 1
	arrowHead         = 14.0
	arrowAngle        = 25 * math.Pi / 180
	textAscent        = 0.85
	textGlyphWidth    = 0.55
	textLineHeight    = 1.3
	viewBoxMargin     = 20
)

var ErrUnknownElement = errors.New("unknown excalidraw element")

type element struct {
	ID              string       `json:"id"`
	IsDeleted       bool         `json:"isDeleted"`
	Type            string       `json:"type"`
	X               float64      `json:"x"`
	Y               float64      `json:"y"`
	Width           float64      `json:"width"`
	Height          float64      `json:"height"`
	Seed            int          `json:"seed"`
	Roughness       *float64     `json:"roughness"`
	StrokeWidth     float64      `json:"strokeWidth"`
	BackgroundColor string       `json:"backgroundColor"`
	FillStyle       string       `json:"fillStyle"`
	Text            string       `json:"text"`
	FontSize        float64      `json:"fontSize"`
	Points          [][2]float64 `json:"points"`
	CustomData      struct {
		Change string `json:"change"`
	} `json:"customData"`
}

type path struct {
	D           string  `json:"d"`
	Stroke      string  `json:"stroke"`
	StrokeWidth float64 `json:"strokeWidth"`
	Fill        string  `json:"fill"`
}

type call struct {
	method string
	args   []any
}

func RenderSVG(scene []byte) ([]byte, error) {
	var doc struct {
		Elements []element `json:"elements"`
	}
	if err := json.Unmarshal(scene, &doc); err != nil {
		return nil, fmt.Errorf("read excalidraw scene: %w", err)
	}
	vm := goja.New()
	if _, err := vm.RunString(roughSource); err != nil {
		return nil, fmt.Errorf("load rough.js: %w", err)
	}
	if _, err := vm.RunString(fmt.Sprintf(generatorScript, pathDecimals)); err != nil {
		return nil, fmt.Errorf("start rough.js generator: %w", err)
	}

	var out strings.Builder
	box := bounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, e := range doc.Elements {
		if e.IsDeleted {
			continue
		}
		color := strokeColor(e)
		fmt.Fprintf(&out, `<g data-id="%s">`, html.EscapeString(e.ID))
		if e.Type == kindText {
			writeText(&out, e, color)
			lineWidth := 0
			for _, line := range strings.Split(e.Text, "\n") {
				lineWidth = max(lineWidth, utf8.RuneCountInString(line))
			}
			box.add(e.X, e.Y)
			box.add(e.X+float64(lineWidth)*e.FontSize*textGlyphWidth, e.Y+e.FontSize*textLineHeight*float64(strings.Count(e.Text, "\n")+1))
		} else {
			calls, err := shapeCalls(e, color)
			if err != nil {
				return nil, err
			}
			for _, c := range calls {
				paths, err := draw(vm, c)
				if err != nil {
					return nil, fmt.Errorf("draw %s: %w", e.ID, err)
				}
				for _, p := range paths {
					fmt.Fprintf(&out, `<path d="%s" stroke="%s" stroke-width="%g" fill="%s"/>`,
						html.EscapeString(p.D), html.EscapeString(p.Stroke), p.StrokeWidth, html.EscapeString(p.Fill))
				}
			}
			box.add(e.X, e.Y)
			box.add(e.X+e.Width, e.Y+e.Height)
			for _, pt := range e.Points {
				box.add(e.X+pt[0], e.Y+pt[1])
			}
		}
		out.WriteString("</g>")
	}
	if math.IsInf(box.minX, 1) {
		box = bounds{}
	}
	svg := fmt.Sprintf(`<svg class="sketch" xmlns="http://www.w3.org/2000/svg" viewBox="%.0f %.0f %.0f %.0f" role="img">%s</svg>`,
		box.minX-viewBoxMargin, box.minY-viewBoxMargin, box.maxX-box.minX+2*viewBoxMargin, box.maxY-box.minY+2*viewBoxMargin, out.String())
	return []byte(svg), nil
}

type bounds struct{ minX, minY, maxX, maxY float64 }

func (b *bounds) add(x, y float64) {
	b.minX, b.minY = math.Min(b.minX, x), math.Min(b.minY, y)
	b.maxX, b.maxY = math.Max(b.maxX, x), math.Max(b.maxY, y)
}

func strokeColor(e element) string {
	style := shareddomain.MarkStyles[shareddomain.Mark(e.CustomData.Change)]
	for _, part := range strings.Split(style, ",") {
		if color, found := strings.CutPrefix(part, strokePrefix); found {
			return color
		}
	}
	return plainStroke
}

func writeText(out *strings.Builder, e element, color string) {
	for i, line := range strings.Split(e.Text, "\n") {
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" font-family="%s" font-size="%.0f" fill="%s">%s</text>`,
			e.X, e.Y+e.FontSize*(float64(i)+textAscent), fontFamily, e.FontSize, color, html.EscapeString(line))
	}
}

func options(e element, color string, roughness float64) map[string]any {
	seed := e.Seed
	if seed == 0 {
		seed = defaultSeed
	}
	opts := map[string]any{
		"seed":        seed,
		"roughness":   roughness,
		"strokeWidth": math.Max(e.StrokeWidth, minStrokeWidth),
		"stroke":      color,
	}
	if e.BackgroundColor != "" && e.BackgroundColor != "transparent" {
		opts["fill"] = e.BackgroundColor
		opts["hachureGap"] = hachureGap
		opts["fillWeight"] = fillWeight
		if e.FillStyle != "" {
			opts["fillStyle"] = e.FillStyle
		}
	}
	return opts
}

func shapeCalls(e element, color string) ([]call, error) {
	roughness := defaultRoughness
	if e.Roughness != nil {
		roughness = *e.Roughness
	}
	opts := options(e, color, roughness)
	switch e.Type {
	case kindRectangle:
		return []call{{"rectangle", []any{e.X, e.Y, e.Width, e.Height, opts}}}, nil
	case kindEllipse:
		return []call{{"ellipse", []any{e.X + e.Width/2, e.Y + e.Height/2, e.Width, e.Height, opts}}}, nil
	case kindDiamond:
		corners := [][2]float64{
			{e.X + e.Width/2, e.Y}, {e.X + e.Width, e.Y + e.Height/2},
			{e.X + e.Width/2, e.Y + e.Height}, {e.X, e.Y + e.Height/2},
		}
		return []call{{"polygon", []any{corners, opts}}}, nil
	case kindLine:
		return []call{{"linearPath", []any{absolutePoints(e), opts}}}, nil
	case kindFreedraw:
		return []call{curveOrLine(absolutePoints(e), options(e, color, freedrawRoughness))}, nil
	case kindArrow:
		points := absolutePoints(e)
		calls := []call{curveOrLine(points, opts)}
		if n := len(points); n >= 2 {
			tip, from := points[n-1], points[n-2]
			angle := math.Atan2(tip[1]-from[1], tip[0]-from[0])
			for _, side := range []float64{-1, 1} {
				t := angle + math.Pi + side*arrowAngle
				calls = append(calls, call{"line", []any{tip[0], tip[1], tip[0] + arrowHead*math.Cos(t), tip[1] + arrowHead*math.Sin(t), opts}})
			}
		}
		return calls, nil
	}
	return nil, fmt.Errorf("%w: %s (%s)", ErrUnknownElement, e.ID, e.Type)
}

func absolutePoints(e element) [][2]float64 {
	points := make([][2]float64, len(e.Points))
	for i, p := range e.Points {
		points[i] = [2]float64{e.X + p[0], e.Y + p[1]}
	}
	return points
}

func curveOrLine(points [][2]float64, opts map[string]any) call {
	const minCurvePoints = 3
	if len(points) < minCurvePoints {
		return call{"linearPath", []any{points, opts}}
	}
	return call{"curve", []any{points, opts}}
}

func draw(vm *goja.Runtime, c call) ([]path, error) {
	args, err := json.Marshal(c.args)
	if err != nil {
		return nil, err
	}
	value, err := vm.RunString(fmt.Sprintf("JSON.stringify(g.toPaths(g.%s(...%s)))", c.method, args))
	if err != nil {
		return nil, err
	}
	var paths []path
	if err := json.Unmarshal([]byte(value.String()), &paths); err != nil {
		return nil, err
	}
	return paths, nil
}
