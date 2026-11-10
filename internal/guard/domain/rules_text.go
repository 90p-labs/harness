package domain

import (
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	emDashRune         = '—'
	coAuthorTag        = "co-authored-by"
	subComment         = "comment"
	cmdCd              = "cd"
	cdArgCount         = 2
	reasonTextFileRead = "cannot read the text file: "
	reasonEmDash       = "the text has an em dash"
	reasonEmoji        = "the text has an emoji"
	reasonCoAuth       = "the text has a Co-Authored-By line"
	nextStepStyle      = "rewrite it with commas, colons or sentences, and drop any emoji or Co-Authored-By line"
)

var (
	textFlags     = []string{"-m", "--message", "--title", "-t", "--body", "-b"}
	textFileFlags = []string{"--file", "--body-file", "-F"}
	prTextActions = []string{subCreate, subEdit, subComment}
	emojiRanges   = &unicode.RangeTable{R32: []unicode.Range32{
		{Lo: 0x1F000, Hi: 0x1F2FF, Stride: 1},
		{Lo: 0x1F300, Hi: 0x1FAFF, Stride: 1},
	}, R16: []unicode.Range16{
		{Lo: 0x2600, Hi: 0x27BF, Stride: 1},
	}}
)

func carriesText(c Command) bool {
	if isCommand(c, cmdGit, subCommit) {
		return true
	}
	return isCommand(c, cmdGh, subPR) && len(c.Args) > 2 && slices.Contains(prTextActions, c.Args[2])
}

func mainTextFiles(commands []Command) [][]string {
	files := make([][]string, len(commands))
	dir := ""
	for i, c := range commands {
		if isCommand(c, cmdCd) && len(c.Args) == cdArgCount {
			dir = resolvePath(dir, c.Args[1])
			continue
		}
		if !carriesText(c) {
			continue
		}
		for _, name := range fileNames(c, textFileFlags...) {
			files[i] = append(files[i], resolvePath(dir, name))
		}
	}
	return files
}

func resolvePath(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

func TextFiles(line string) [][]string {
	commands, flat := flatInvocations(line)
	if !flat {
		commands = invocations(line)
	}
	return mainTextFiles(commands)
}

func fileNames(c Command, flags ...string) []string {
	return slices.DeleteFunc(flagValues(c, flags...), func(v string) bool { return v == flagStdin })
}

func textsOf(call ToolCall, command int) []string {
	if command < len(call.Texts) {
		return call.Texts[command]
	}
	return nil
}

func commandTexts(c Command, files []string) []string {
	texts := flagValues(c, textFlags...)
	if readsStdin(c, textFileFlags...) {
		texts = append(texts, c.Heredoc)
	}
	return append(texts, files...)
}

func denyWritingStyle(call ToolCall, rc RuleContext) (string, bool) {
	var texts []string
	for i, c := range invocations(call.Command) {
		if !carriesText(c) {
			continue
		}
		if rc.TextErr != nil && len(fileNames(c, textFileFlags...)) > 0 {
			return deny(reasonTextFileRead+rc.TextErr.Error(), nextStepBody)
		}
		texts = append(texts, commandTexts(c, textsOf(call, i))...)
	}
	for _, text := range texts {
		switch {
		case strings.ContainsRune(text, emDashRune):
			return deny(reasonEmDash, nextStepStyle)
		case strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(emojiRanges, r) }):
			return deny(reasonEmoji, nextStepStyle)
		case slices.ContainsFunc(strings.Split(text, "\n"), isCoAuthorLine):
			return deny(reasonCoAuth, nextStepStyle)
		}
	}
	return "", false
}

func isCoAuthorLine(line string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), coAuthorTag)
}
