package docs_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	programsDir     = "docs/programs"
	programPlanName = "plan.md"
	programMapName  = "map.md"
)

func TestProgramDocuments(t *testing.T) {
	root := repository.Path(t, programsDir)
	plans, maps := 0, 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch entry.Name() {
		case programPlanName:
			plans++
			checkProgramPlan(t, path)
		case programMapName:
			maps++
			checkProgramMap(t, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if plans == 0 {
		t.Errorf("%s has no %s", programsDir, programPlanName)
	}
	if maps == 0 {
		t.Errorf("%s has no %s", programsDir, programMapName)
	}
}

func checkProgramPlan(t *testing.T, path string) {
	t.Helper()
	text := repository.Read(t, relativeToRepository(t, path))
	if !plandomain.IsProgram(text) {
		t.Errorf("%s is not a multi-run plan", path)
		return
	}
	prog, problems := plandomain.ParseProgram(text)
	problems = append(problems, plandomain.CheckProgram(prog)...)
	for _, problem := range problems {
		t.Errorf("%s: %v", path, problem)
	}
}

func checkProgramMap(t *testing.T, path string) {
	t.Helper()
	text := repository.Read(t, relativeToRepository(t, path))
	if !plandomain.IsMap(text) {
		t.Errorf("%s is not a map", path)
		return
	}
	doc, problems := plandomain.ParseMap(text)
	problems = append(problems, plandomain.CheckMap(doc)...)
	for _, problem := range problems {
		t.Errorf("%s: %v", path, problem)
	}
}

func relativeToRepository(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repository.Path(t), path)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}
