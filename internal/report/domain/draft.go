package domain

import (
	"errors"
	"fmt"
	"strings"
)

const (
	sectionSummary      = "Summary"
	sectionReproduction = "Reproduction"
	sectionCommands     = "Commands"
	sectionConfig       = "Config"
	sectionExpected     = "Expected"
	sectionActual       = "Actual"
	sectionTraces       = "Traces"
	sectionHost         = "Host"
	sectionOutput       = "Output"
	sectionFix          = "Fix suggestion"
	sectionWorkaround   = "Workaround"
	sectionEnvironment  = "Environment"

	RunNone = "none"

	placeholder      = "[fill:"
	sectionPrefix    = "## "
	subsectionPrefix = "### "
	pathSeparator    = " > "
	fenceBacktick    = '`'
	fenceTilde       = '~'
	minFenceLength   = 3
)

var ErrDraftIncomplete = errors.New("the draft misses sections")

var required = []string{
	sectionSummary,
	sectionReproduction,
	sectionReproduction + pathSeparator + sectionCommands,
	sectionReproduction + pathSeparator + sectionConfig,
	sectionReproduction + pathSeparator + sectionExpected,
	sectionReproduction + pathSeparator + sectionActual,
	sectionTraces,
	sectionTraces + pathSeparator + sectionHost,
	sectionTraces + pathSeparator + sectionOutput,
	sectionFix,
	sectionWorkaround,
}

type Draft struct {
	Title string
	Body  string
}

type Environment struct {
	Version  string
	Platform string
	Run      string
	Home     string
}

func RunLine(id, phase string) string {
	return id + ", phase " + phase
}

func (d Draft) Check() error {
	texts := sectionTexts(d.Body)
	var missing []string
	for _, path := range required {
		own, found := texts[path]
		filled := isFilled(own)
		for other, text := range texts {
			if strings.HasPrefix(other, path+pathSeparator) && isFilled(text) {
				filled = true
			}
		}
		if !found || !filled || strings.Contains(own, placeholder) {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrDraftIncomplete, strings.Join(missing, ", "))
	}
	return nil
}

func isFilled(text string) bool {
	if strings.Contains(text, placeholder) {
		return false
	}
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && fenceMarker(trimmed) == "" {
			return true
		}
	}
	return false
}

func (d Draft) Issue(env Environment) Draft {
	return Draft{Title: Redact(d.Title, env.Home), Body: Redact(stamp(d.Body, env), env.Home)}
}

func stamp(body string, env Environment) string {
	lines := strings.Split(body, "\n")
	for i, kind := range headingKinds(lines) {
		if kind == sectionPrefix && headingTitle(lines[i], kind) == sectionTraces {
			block := fmt.Sprintf("%s%s\n\n- harness: %s\n- os: %s\n- run: %s\n",
				subsectionPrefix, sectionEnvironment, env.Version, env.Platform, env.Run)
			before := strings.Join(lines[:i+1], "\n")
			after := strings.TrimLeft(strings.Join(lines[i+1:], "\n"), "\n")
			return before + "\n\n" + block + "\n" + after
		}
	}
	return body
}

func sectionTexts(body string) map[string]string {
	lines := strings.Split(body, "\n")
	texts := map[string]string{}
	var section, subsection string
	for i, kind := range headingKinds(lines) {
		switch kind {
		case sectionPrefix:
			section, subsection = headingTitle(lines[i], kind), ""
			texts[section] += ""
		case subsectionPrefix:
			subsection = section + pathSeparator + headingTitle(lines[i], kind)
			texts[subsection] += ""
		default:
			switch {
			case subsection != "":
				texts[subsection] += lines[i] + "\n"
			case section != "":
				texts[section] += lines[i] + "\n"
			}
		}
	}
	return texts
}

func headingKinds(lines []string) []string {
	kinds := make([]string, len(lines))
	var fence string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); marker != "" {
			switch {
			case fence == "":
				fence = marker
			case marker[0] == fence[0] && len(marker) >= len(fence) && trimmed == marker:
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		for _, prefix := range []string{sectionPrefix, subsectionPrefix} {
			if strings.HasPrefix(line, prefix) {
				kinds[i] = prefix
			}
		}
	}
	return kinds
}

func fenceMarker(line string) string {
	if line == "" || (line[0] != fenceBacktick && line[0] != fenceTilde) {
		return ""
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < minFenceLength || (line[0] == fenceBacktick && strings.ContainsRune(line[n:], fenceBacktick)) {
		return ""
	}
	return line[:n]
}

func headingTitle(line, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, prefix))
}
