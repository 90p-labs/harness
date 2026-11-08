package domain

import (
	"path"
	"slices"
	"strings"
)

const (
	RuleNoLibFolder = "next/no-lib-folder"
	libFolder       = "lib"
)

func NoLibFolder(tree Tree, _ Graph, _ Settings) []Violation {
	var found []Violation
	seen := map[string]bool{}
	for _, file := range tree.Files {
		if !underDir(file, sourceFolder) {
			continue
		}
		segments := strings.Split(path.Dir(file), pathSeparator)
		index := slices.Index(segments, libFolder)
		if index < 0 {
			continue
		}
		folder := path.Join(segments[:index+1]...)
		if !seen[folder] {
			seen[folder] = true
			found = append(found, Violation{File: folder})
		}
	}
	return found
}
