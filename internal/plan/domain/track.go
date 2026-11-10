package domain

import (
	"path"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	FolderMigrations = "migrations/"
	SuffixSQL        = ".sql"
	fullFolderCount  = 3
	folderHooks      = "hooks"
	folderPluginHook = "pluginhooks"
)

var hookFolders = []string{folderHooks, folderPluginHook}

func Classify(doc Document) shareddomain.Track {
	declared := shareddomain.Track(doc.Track)
	if touchesManyFolders(doc.CodeTree) || touchesFullFolder(doc.CodeTree) {
		return shareddomain.Stricter(declared, shareddomain.TrackFull)
	}
	return shareddomain.Stricter(declared, shareddomain.TrackLight)
}

func NeedsAdvisor(doc Document) bool {
	return Classify(doc) == shareddomain.TrackFull || hasThreatFolder(doc.CodeTree) || hasHookFolder(doc.CodeTree)
}

func hasHookFolder(tree Tree) bool {
	return slices.ContainsFunc(tree.Entries, func(e TreeEntry) bool {
		if e.IsDir {
			return false
		}
		return slices.ContainsFunc(hookFolders, func(f string) bool { return slices.Contains(strings.Split(path.Dir(e.Path), "/"), f) })
	})
}

func touchesManyFolders(tree Tree) bool {
	folders := map[string]bool{}
	for _, e := range tree.Entries {
		if !e.IsDir {
			folders[path.Dir(e.Path)] = true
		}
	}
	return len(folders) >= fullFolderCount
}

func touchesFullFolder(tree Tree) bool {
	if tree.Touches(FolderDomain, FolderMigrations, FolderCommand, FolderQuery) {
		return true
	}
	for _, e := range tree.Entries {
		if !e.IsDir && strings.HasSuffix(e.Path, SuffixSQL) {
			return true
		}
	}
	return false
}
