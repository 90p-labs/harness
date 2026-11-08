package domain

import (
	"path"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	FolderMigrations = "migrations/"
	SuffixSQL        = ".sql"
	fullFolderCount  = 3
)

func Classify(doc Document) shareddomain.Track {
	declared := shareddomain.Track(doc.Track)
	if touchesManyFolders(doc.CodeTree) || touchesFullFolder(doc.CodeTree) {
		return shareddomain.Stricter(declared, shareddomain.TrackFull)
	}
	return shareddomain.Stricter(declared, shareddomain.TrackLight)
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
