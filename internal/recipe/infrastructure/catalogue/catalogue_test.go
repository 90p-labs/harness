package catalogue_test

import (
	"errors"
	"io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/90p-labs/harness/examples"
	"github.com/90p-labs/harness/internal/recipe/domain"
	"github.com/90p-labs/harness/internal/recipe/infrastructure/catalogue"
)

const (
	pluginFolder   = ".harness/plugins/"
	ignoreLines    = ".harness/*"
	keepLine       = "!.harness/plugins/"
	forbiddenHost  = "init --host"
	repositoryRoot = "../../../.."
)

var exampleNames = []string{"go", "nextjs", "python", "symfony"}

func excluded(file string) bool {
	switch path.Base(file) {
	case "README.md", "recipe.yaml", "guide.md":
		return true
	}
	return strings.HasSuffix(file, "_test.go")
}

func trackedFiles(t *testing.T, example string) []string {
	t.Helper()
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "--", "examples/"+example).Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		file := strings.TrimPrefix(line, "examples/"+example+"/")
		if line != "" && !excluded(file) {
			files = append(files, file)
		}
	}
	slices.Sort(files)
	return files
}

func TestCatalogueCoversExamples(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	if got := len(cat.List()); got != len(exampleNames) {
		t.Fatalf("List() has %d recipes, want %d", got, len(exampleNames))
	}
	for _, example := range exampleNames {
		t.Run(example, func(t *testing.T) {
			recipe, err := cat.Get(domain.Name(example))
			if err != nil {
				t.Fatalf("Get(%s) = %v", example, err)
			}
			if err := recipe.Validate(); err != nil {
				t.Errorf("Validate() = %v", err)
			}
			if recipe.Summary == "" {
				t.Error("summary is empty")
			}
			listed := slices.Clone(recipe.Files)
			slices.Sort(listed)
			if want := trackedFiles(t, example); !slices.Equal(listed, want) {
				t.Errorf("files = %v\nwant git ls-files minus the excluded names = %v", listed, want)
			}
			for _, file := range recipe.Files {
				if _, err := cat.Read(recipe.Name, file); err != nil {
					t.Errorf("Read(%s) = %v", file, err)
				}
			}
			next := strings.Join(recipe.Next, "\n")
			if strings.Contains(next, forbiddenHost) {
				t.Errorf("next steps name %q", forbiddenHost)
			}
			if !strings.Contains(next, "harness verify") {
				t.Error("next steps do not name harness verify")
			}
			hasPlugin := slices.ContainsFunc(recipe.Files, func(f string) bool { return strings.HasPrefix(f, pluginFolder) })
			ignoreStep := slices.ContainsFunc(recipe.Next, func(s string) bool {
				return strings.Contains(s, ignoreLines) && strings.Contains(s, keepLine)
			})
			if hasPlugin != ignoreStep {
				t.Errorf("plugin = %v but the .gitignore next step = %v", hasPlugin, ignoreStep)
			}
		})
	}
}

func TestCatalogueSymfonyNeedsGatesInit(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := cat.Get("symfony")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(recipe.Next, "harness gates init") {
		t.Errorf("next = %v, want harness gates init", recipe.Next)
	}
}

func TestCatalogueLintInitBeforeVerify(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range exampleNames {
		t.Run(example, func(t *testing.T) {
			recipe, err := cat.Get(domain.Name(example))
			if err != nil {
				t.Fatal(err)
			}
			lintInit := slices.IndexFunc(recipe.Next, func(s string) bool { return strings.Contains(s, "harness lint init") })
			verify := slices.IndexFunc(recipe.Next, func(s string) bool { return strings.Contains(s, "harness verify") })
			if lintInit < 0 {
				t.Fatalf("next = %v, want a harness lint init step", recipe.Next)
			}
			if verify < 0 || lintInit > verify {
				t.Errorf("next = %v, want harness lint init before harness verify", recipe.Next)
			}
			baseline := slices.IndexFunc(recipe.Next, func(s string) bool { return strings.Contains(s, "harness lint baseline") })
			if baseline < 0 || baseline < lintInit || (verify >= 0 && baseline > verify) {
				t.Errorf("next = %v, want harness lint baseline after harness lint init and before harness verify", recipe.Next)
			}
		})
	}
}

func TestCatalogueScaffoldEntersAppFolderAndInitsGit(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []string{"nextjs", "symfony"} {
		t.Run(example, func(t *testing.T) {
			recipe, err := cat.Get(domain.Name(example))
			if err != nil {
				t.Fatal(err)
			}
			create := slices.IndexFunc(recipe.Scaffold, func(s string) bool {
				return strings.Contains(s, " app ") || strings.HasSuffix(s, " app")
			})
			if create < 0 {
				t.Fatalf("scaffold = %v, want a step that creates the app folder", recipe.Scaffold)
			}
			if create+1 >= len(recipe.Scaffold) || recipe.Scaffold[create+1] != "cd app" {
				t.Errorf("scaffold = %v, want cd app right after the step that creates app", recipe.Scaffold)
			}
			gitInit := slices.Index(recipe.Scaffold, "git init")
			if gitInit < create+2 {
				t.Errorf("scaffold = %v, want git init after cd app", recipe.Scaffold)
			}
		})
	}
}

func fakeFS() fstest.MapFS {
	return fstest.MapFS{
		"demo/recipe.yaml": {Data: []byte("name: demo\nsummary: A demo\nfiles:\n  - a.txt\n  - dir/b.txt\nscaffold:\n  - make app\nnext:\n  - harness verify\n")},
		"demo/a.txt":       {Data: []byte("a")},
		"demo/dir/b.txt":   {Data: []byte("b")},
		"demo/unlisted":    {Data: []byte("never read")},
		"loose.txt":        {Data: []byte("not a recipe")},
	}
}

func TestCatalogueReadsGuide(t *testing.T) {
	fsys := fakeFS()
	fsys["demo/guide.md"] = &fstest.MapFile{Data: []byte("# Guide\n")}
	cat, err := catalogue.New(fsys)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	recipe, err := cat.Get("demo")
	if err != nil {
		t.Fatal(err)
	}
	if recipe.Guide != "# Guide\n" {
		t.Errorf("Guide = %q", recipe.Guide)
	}
	if slices.Contains(recipe.Files, "guide.md") {
		t.Error("guide.md is listed as a file")
	}
	if _, err := cat.Read("demo", "guide.md"); err == nil {
		t.Error("Read(guide.md) succeeded, apply could write it")
	}
	if recipe.Summary != "A demo" || !slices.Equal(recipe.Scaffold, []string{"make app"}) || !slices.Equal(recipe.Next, []string{"harness verify"}) {
		t.Errorf("recipe = %+v", recipe)
	}
	got, err := cat.Read("demo", "dir/b.txt")
	if err != nil || string(got) != "b" {
		t.Errorf("Read(dir/b.txt) = %q, %v", got, err)
	}
}

func TestCatalogueNoGuide(t *testing.T) {
	cat, err := catalogue.New(fakeFS())
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.Get("demo")
	if recipe.Guide != "" {
		t.Errorf("Guide = %q, want empty", recipe.Guide)
	}
}

func TestCatalogueUnknownName(t *testing.T) {
	cat, err := catalogue.New(fakeFS())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Get("nope"); !errors.Is(err, domain.ErrUnknownRecipe) {
		t.Errorf("Get(nope) = %v, want ErrUnknownRecipe", err)
	}
	if _, err := cat.Read("nope", "a.txt"); !errors.Is(err, domain.ErrUnknownRecipe) {
		t.Errorf("Read(nope) = %v, want ErrUnknownRecipe", err)
	}
}

func TestCatalogueReadRefusesUnlistedFile(t *testing.T) {
	cat, err := catalogue.New(fakeFS())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Read("demo", "unlisted"); err == nil {
		t.Error("Read of an unlisted file succeeded")
	}
}

func TestCatalogueMissingListedFile(t *testing.T) {
	fsys := fakeFS()
	delete(fsys, "demo/dir/b.txt")
	if _, err := catalogue.New(fsys); err == nil || !strings.Contains(err.Error(), "dir/b.txt") {
		t.Errorf("New() = %v, want an error that names dir/b.txt", err)
	}
}

func TestCatalogueRejectsInvalidRecipe(t *testing.T) {
	fsys := fakeFS()
	fsys["demo/recipe.yaml"] = &fstest.MapFile{Data: []byte("name: demo\nsummary: s\nfiles:\n  - ../x\n")}
	if _, err := catalogue.New(fsys); !errors.Is(err, domain.ErrBadPath) {
		t.Errorf("New() = %v, want ErrBadPath", err)
	}
}

func TestCatalogueRejectsUnknownKey(t *testing.T) {
	fsys := fakeFS()
	fsys["demo/recipe.yaml"] = &fstest.MapFile{Data: []byte("name: demo\nsummary: s\nfiles:\n  - a.txt\nscafold:\n  - make app\n")}
	if _, err := catalogue.New(fsys); err == nil || !strings.Contains(err.Error(), "scafold") {
		t.Errorf("New() = %v, want an error that names scafold", err)
	}
}

func TestCatalogueRejectsDuplicateName(t *testing.T) {
	fsys := fakeFS()
	fsys["other/recipe.yaml"] = &fstest.MapFile{Data: []byte("name: demo\nsummary: s\nfiles:\n  - a.txt\n")}
	fsys["other/a.txt"] = &fstest.MapFile{Data: []byte("a")}
	if _, err := catalogue.New(fsys); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("New() = %v, want a duplicate name error", err)
	}
}

func TestCatalogueNextNamesPlaceholders(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	names := func(next []string, file, placeholder string) bool {
		return slices.ContainsFunc(next, func(s string) bool {
			return strings.Contains(s, file) && strings.Contains(s, placeholder)
		})
	}
	for _, example := range exampleNames {
		t.Run(example, func(t *testing.T) {
			recipe, err := cat.Get(domain.Name(example))
			if err != nil {
				t.Fatal(err)
			}
			if !names(recipe.Next, "compose.yaml", "name:") {
				t.Errorf("next = %v, want a step that names the name: of compose.yaml", recipe.Next)
			}
			if example == "python" && !names(recipe.Next, ".importlinter", "root_package") {
				t.Errorf("next = %v, want a step that names root_package of .importlinter", recipe.Next)
			}
			if example == "python" {
				body, err := cat.Read(recipe.Name, ".importlinter")
				if err != nil {
					t.Fatal(err)
				}
				outside := 0
				for _, line := range strings.Split(string(body), "\n") {
					if !strings.HasPrefix(line, "root_package") && strings.Contains(line, "shop") {
						outside++
					}
				}
				if outside > 0 && !names(recipe.Next, ".importlinter", "every shop") {
					t.Errorf("next = %v, want a step that says to replace every shop in .importlinter (%d lines outside root_package)", recipe.Next, outside)
				}
			}
		})
	}
}

func TestCataloguePythonLeadsToTestsAndContexts(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := cat.Get("python")
	if err != nil {
		t.Fatal(err)
	}
	// F51: the gates run mypy src tests and pytest, so a fresh scaffold needs a test.
	const smokeStep = "mkdir -p tests && printf 'def test_smoke() -> None:\\n    assert True\\n' > tests/test_smoke.py"
	smoke := slices.Index(recipe.Scaffold, smokeStep)
	gitInit := slices.Index(recipe.Scaffold, "git init")
	if smoke < 0 || gitInit < 0 || smoke > gitInit {
		t.Errorf("scaffold = %v, want the step %q before git init", recipe.Scaffold, smokeStep)
	}
	// F52: the exhaustive contracts of .importlinter list the placeholder contexts.
	body, err := cat.Read(recipe.Name, ".importlinter")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "(order) : (billing) : (shared)") {
		t.Fatal(".importlinter no longer lists the contexts (order) : (billing) : (shared)")
	}
	const contextsStep = "in .importlinter, edit the contexts (order) : (billing) : (shared) to match the contexts under src/, in every contract that lists them"
	if !slices.Contains(recipe.Next, contextsStep) {
		t.Errorf("next = %v, want the step %q", recipe.Next, contextsStep)
	}
}

func TestCataloguePythonScaffoldPinsImagePython(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := cat.Get("python")
	if err != nil {
		t.Fatal(err)
	}
	// F57: uv init pins the host Python in .python-version, but the image has only its own.
	compose, err := cat.Read(recipe.Name, "compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`image:\s*\S*-python(\d+\.\d+)-`).FindStringSubmatch(string(compose))
	if match == nil {
		t.Fatal("compose.yaml has no image tag with a pythonX.Y version")
	}
	want := "uv init --lib --python " + match[1]
	if !slices.Contains(recipe.Scaffold, want) {
		t.Errorf("scaffold = %v, want the step %q to match the image Python", recipe.Scaffold, want)
	}
}

func TestCatalogueScaffoldEndsWithApply(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range exampleNames {
		t.Run(example, func(t *testing.T) {
			recipe, err := cat.Get(domain.Name(example))
			if err != nil {
				t.Fatal(err)
			}
			if len(recipe.Scaffold) == 0 {
				t.Skip("no scaffold")
			}
			want := "harness recipe apply " + example
			if got := recipe.Scaffold[len(recipe.Scaffold)-1]; got != want {
				t.Errorf("scaffold = %v, want the last step %q", recipe.Scaffold, want)
			}
			if len(recipe.Scaffold) < 2 || recipe.Scaffold[len(recipe.Scaffold)-2] != "git init" {
				t.Errorf("scaffold = %v, want git init right before the apply", recipe.Scaffold)
			}
		})
	}
}

func TestCatalogueNextjsInstallsLintToolsBeforeLintInit(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := cat.Get("nextjs")
	if err != nil {
		t.Fatal(err)
	}
	install := slices.Index(recipe.Next, "npm i -g @biomejs/biome@2.6.6 oxlint@1.92.0")
	lintInit := slices.IndexFunc(recipe.Next, func(s string) bool { return strings.Contains(s, "harness lint init") })
	if install < 0 || lintInit < 0 || install > lintInit {
		t.Errorf("next = %v, want the biome and oxlint install before harness lint init", recipe.Next)
	}
}

func TestCatalogueReturnsCopies(t *testing.T) {
	cat, err := catalogue.New(fakeFS())
	if err != nil {
		t.Fatal(err)
	}
	damage := func(r domain.Recipe) {
		r.Files[0] = "changed"
		r.Scaffold[0] = "changed"
		r.Next[0] = "changed"
	}
	got, err := cat.Get("demo")
	if err != nil {
		t.Fatal(err)
	}
	damage(got)
	damage(cat.List()[0])

	again, err := cat.Get("demo")
	if err != nil {
		t.Fatal(err)
	}
	if again.Files[0] != "a.txt" || again.Scaffold[0] != "make app" || again.Next[0] != "harness verify" {
		t.Errorf("a returned recipe changed the catalogue: %+v", again)
	}
	if _, err := cat.Read("demo", "a.txt"); err != nil {
		t.Errorf("Read(a.txt) = %v after the change of a returned recipe", err)
	}
}

type statFailsFS struct {
	fstest.MapFS
	failOn string
	err    error
}

func (f statFailsFS) Stat(name string) (fs.FileInfo, error) {
	if name == f.failOn {
		return nil, f.err
	}
	return f.MapFS.Stat(name)
}

func TestCatalogueStatErrorIsNotAMissingFile(t *testing.T) {
	boom := errors.New("input/output error")
	fsys := statFailsFS{MapFS: fakeFS(), failOn: "demo/a.txt", err: boom}

	_, err := catalogue.New(fsys)

	if !errors.Is(err, boom) {
		t.Fatalf("New() = %v, want the stat error wrapped", err)
	}
	if strings.Contains(err.Error(), "not a file") {
		t.Errorf("New() = %v, an I/O error must not read as a missing file", err)
	}
}

func TestCatalogueListIsSorted(t *testing.T) {
	cat, err := catalogue.New(examples.FS)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range cat.List() {
		names = append(names, r.Name.String())
	}
	if !slices.IsSorted(names) {
		t.Errorf("List() = %v, want sorted", names)
	}
}
