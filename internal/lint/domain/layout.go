package domain

import (
	"encoding/json"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	RuleSrcRoots         = "ddd/src-roots"
	RuleContextLayers    = "ddd/context-layers"
	RuleParentNoCode     = "ddd/parent-no-code"
	RuleSharedLayers     = "ddd/shared-layers"
	RuleSharedBaseOnly   = "ddd/shared-base-only"
	RuleFileSuffix       = "ddd/file-suffix"
	RuleCQRSFolders      = "ddd/cqrs-folders"
	RuleHandlerPair      = "ddd/handler-pair"
	RuleModulePerContext = "ddd/module-per-context"
	RuleOneKernel        = "ddd/one-kernel"
	RuleRepositoryPort   = "ddd/repository-port"
	RuleMapperLocation   = "ddd/mapper-location"
	RuleEventPastTense   = "ddd/event-past-tense"
	RuleAppRouterFiles   = "next/app-router-files"
	RuleThinRouteHandler = "next/thin-route-handler"
	RuleComponentsJSON   = "shadcn/components-json"
	RuleUtilsLocation    = "combo/utils-location"
)

const (
	sourceFolder        = "src"
	appFolder           = "app"
	componentsFolder    = "components"
	sharedFolder        = "shared"
	commandsFolder      = "commands"
	queriesFolder       = "queries"
	eventHandlersFolder = "event-handlers"
	kernelFile          = "kernel.ts"
	componentsJSONFile  = "components.json"
	routeFile           = "route.ts"
	globalStylesFile    = "globals.css"
	cssModuleSuffix     = ".module.css"
	pathSeparator       = "/"
	maxHandlerLines     = 30
)

const (
	layerDomain         = "domain"
	layerApplication    = "application"
	layerInfrastructure = "infrastructure"
)

const (
	suffixCommand    = "command"
	suffixQuery      = "query"
	suffixHandler    = "handler"
	suffixEvent      = "event"
	suffixAggregate  = "aggregate"
	suffixEntity     = "entity"
	suffixValueObj   = "value-object"
	suffixRepository = "repository"
	suffixMapper     = "mapper"
	suffixError      = "error"
	suffixModule     = "module"
	suffixController = "controller"
	suffixAdapter    = "adapter"
	suffixSchema     = "schema"
	suffixProjection = "projection"
	suffixSpec       = "spec"
	suffixMother     = "mother"
	adapterSuffix    = "-" + suffixRepository
	pastTenseEnding  = "ed"
	wordSeparator    = "-"
)

var (
	layers = []string{layerDomain, layerApplication, layerInfrastructure}

	knownSuffixes = []string{
		suffixCommand, suffixQuery, suffixHandler, suffixEvent, suffixAggregate, suffixEntity,
		suffixValueObj, suffixRepository, suffixMapper, suffixError, suffixModule, suffixController,
		suffixAdapter, suffixSchema, suffixProjection, suffixSpec, suffixMother,
	}

	sharedBaseSuffixes = []string{suffixAggregate, suffixEntity, suffixValueObj, suffixEvent, suffixError}

	cqrsFolders = []string{commandsFolder, queriesFolder, eventHandlersFolder}

	irregularPastWords = []string{
		"sent", "made", "paid", "built", "begun", "gone", "done", "won", "lost", "left", "kept",
		"bought", "held", "set", "put", "cut", "taken", "given", "shown", "chosen", "written", "sold", "found",
	}

	suffixedExtensions = []string{".ts", ".tsx"}

	routeStems    = []string{"page", "layout", "template", "default", "loading", "error", "global-error", "not-found", "route", "forbidden", "unauthorized"}
	scriptExts    = []string{".ts", ".tsx", ".js", ".jsx"}
	metadataStems = []string{"favicon", "icon", "apple-icon", "opengraph-image", "twitter-image", "sitemap", "robots", "manifest"}

	utilsAliasPrefixes = []string{"@/", "~/", sourceFolder + pathSeparator}

	handlerStart = regexp.MustCompile(`^export\s+(?:async\s+)?function\s+(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\b`)
)

type Violation struct {
	File string
	Line int
}

type Check func(tree Tree, graph Graph, settings Settings) []Violation

type componentsConfig struct {
	Tailwind struct {
		CSSVariables bool `json:"cssVariables"`
	} `json:"tailwind"`
	Aliases struct {
		Utils string `json:"utils"`
	} `json:"aliases"`
}

type layout struct {
	root     string
	groups   []string
	contexts []string
	shared   []string
	files    []string
}

func businessRoot(settings Settings) string {
	return path.Join(sourceFolder, settings.DDD.Root)
}

func newLayout(tree Tree, settings Settings) layout {
	l := layout{root: businessRoot(settings)}
	for _, file := range tree.Files {
		if underDir(file, l.root) {
			l.files = append(l.files, file)
		}
	}
	if len(l.files) > 0 {
		l.walk(l.root, l.files)
	}
	return l
}

func (l *layout) walk(dir string, files []string) {
	children := map[string][]string{}
	hasModule := false
	for _, file := range files {
		name, _, nested := strings.Cut(strings.TrimPrefix(file, dir+pathSeparator), pathSeparator)
		if nested {
			children[name] = append(children[name], file)
			continue
		}
		hasModule = hasModule || fileSuffix(name) == suffixModule
	}
	names := slices.Sorted(maps.Keys(children))
	if dir != l.root && (hasModule || slices.ContainsFunc(names, isLayer)) {
		l.contexts = append(l.contexts, dir)
		return
	}
	l.groups = append(l.groups, dir)
	for _, name := range names {
		child := path.Join(dir, name)
		if name == sharedFolder {
			l.shared = append(l.shared, child)
			continue
		}
		l.walk(child, children[name])
	}
}

func (l layout) layerFolders() []string {
	var all []string
	for _, owner := range slices.Concat(l.contexts, l.shared) {
		for _, name := range folders(l.files, owner) {
			if isLayer(name) {
				all = append(all, path.Join(owner, name))
			}
		}
	}
	return all
}

func (l layout) contextOf(file string) string {
	for _, context := range l.contexts {
		if underDir(file, context) {
			return context
		}
	}
	return ""
}

func isLayer(name string) bool {
	return slices.Contains(layers, name)
}

func underDir(file, dir string) bool {
	return strings.HasPrefix(file, dir+pathSeparator)
}

func layerOf(root, file string) string {
	rest, ok := strings.CutPrefix(path.Dir(file), root+pathSeparator)
	if !ok {
		return ""
	}
	for _, segment := range strings.Split(rest, pathSeparator) {
		if isLayer(segment) {
			return segment
		}
	}
	return ""
}

func fileSuffix(name string) string {
	parts := strings.Split(path.Base(name), ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[len(parts)-2]
}

func isKnownSuffix(suffix string) bool {
	return slices.Contains(knownSuffixes, suffix) || strings.HasSuffix(suffix, adapterSuffix)
}

func folders(files []string, dir string) []string {
	seen := map[string]bool{}
	for _, file := range files {
		if name, _, nested := strings.Cut(strings.TrimPrefix(file, dir+pathSeparator), pathSeparator); nested && underDir(file, dir) {
			seen[name] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func directFiles(files []string, dir string) []string {
	return slices.DeleteFunc(slices.Clone(files), func(file string) bool { return path.Dir(file) != dir })
}

func readComponents(tree Tree) (componentsConfig, bool, error) {
	var config componentsConfig
	if !slices.Contains(tree.Files, componentsJSONFile) {
		return config, false, nil
	}
	raw, err := tree.Source(componentsJSONFile)
	if err != nil {
		return config, true, err
	}
	return config, true, json.Unmarshal(raw, &config)
}

// IsScript tells whether the built-in engine reads the file as TypeScript or JavaScript.
func IsScript(file string) bool {
	return slices.Contains(scriptExts, path.Ext(file))
}

func SrcRoots(tree Tree, _ Graph, settings Settings) []Violation {
	var found []Violation
	seen := map[string]bool{}
	allowed := []string{appFolder, componentsFolder, settings.DDD.Root}
	for _, file := range tree.Files {
		rest, ok := strings.CutPrefix(file, sourceFolder+pathSeparator)
		if !ok {
			continue
		}
		name, _, nested := strings.Cut(rest, pathSeparator)
		switch {
		case !IsScript(file):
		case !nested && !slices.Contains(settings.DDD.SrcFiles, name):
			found = append(found, Violation{File: file})
		case nested && !slices.Contains(allowed, name) && !seen[name]:
			seen[name] = true
			found = append(found, Violation{File: path.Join(sourceFolder, name)})
		}
	}
	return found
}

func ContextLayers(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, context := range l.contexts {
		count := 0
		for _, name := range folders(l.files, context) {
			if isLayer(name) {
				count++
			}
		}
		if count == 1 {
			found = append(found, Violation{File: context})
		}
	}
	return found
}

func ParentNoCode(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, group := range l.groups {
		for _, file := range directFiles(l.files, group) {
			if file != path.Join(l.root, kernelFile) {
				found = append(found, Violation{File: file})
			}
		}
	}
	return found
}

func SharedLayers(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, shared := range l.shared {
		for _, name := range folders(l.files, shared) {
			if !isLayer(name) {
				found = append(found, Violation{File: path.Join(shared, name)})
			}
		}
	}
	return found
}

func SharedBaseOnly(tree Tree, _ Graph, settings Settings) []Violation {
	var found []Violation
	dir := path.Join(businessRoot(settings), sharedFolder, layerDomain)
	for _, file := range tree.Files {
		if underDir(file, dir) && !slices.Contains(sharedBaseSuffixes, fileSuffix(file)) {
			found = append(found, Violation{File: file})
		}
	}
	return found
}

func FileSuffix(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, folder := range l.layerFolders() {
		for _, file := range l.files {
			if underDir(file, folder) && slices.Contains(suffixedExtensions, path.Ext(file)) && !isKnownSuffix(fileSuffix(file)) {
				found = append(found, Violation{File: file})
			}
		}
	}
	return found
}

func CQRSFolders(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, folder := range l.layerFolders() {
		if path.Base(folder) != layerApplication {
			continue
		}
		for _, name := range folders(l.files, folder) {
			if !slices.Contains(cqrsFolders, name) {
				found = append(found, Violation{File: path.Join(folder, name)})
			}
		}
	}
	return found
}

func HandlerPair(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	messageSuffix := map[string]string{commandsFolder: suffixCommand, queriesFolder: suffixQuery}
	var found []Violation
	for _, folder := range l.layerFolders() {
		if path.Base(folder) != layerApplication {
			continue
		}
		for kind, message := range messageSuffix {
			for _, name := range folders(l.files, path.Join(folder, kind)) {
				messageDir := path.Join(folder, kind, name)
				files := directFiles(l.files, messageDir)
				if countSuffix(files, message) != 1 || countSuffix(files, suffixHandler) != 1 {
					found = append(found, Violation{File: messageDir})
				}
			}
		}
	}
	slices.SortFunc(found, func(a, b Violation) int { return strings.Compare(a.File, b.File) })
	return found
}

func countSuffix(files []string, suffix string) int {
	count := 0
	for _, file := range files {
		if fileSuffix(file) == suffix {
			count++
		}
	}
	return count
}

func ModulePerContext(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, context := range l.contexts {
		if countSuffix(directFiles(l.files, context), suffixModule) != 1 {
			found = append(found, Violation{File: context})
		}
	}
	return found
}

func OneKernel(tree Tree, _ Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	if len(l.files) == 0 || slices.Contains(l.files, path.Join(l.root, kernelFile)) {
		return nil
	}
	return []Violation{{File: l.root}}
}

func RepositoryPort(tree Tree, _ Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, file := range tree.Files {
		suffix := fileSuffix(file)
		port := suffix == suffixRepository && layerOf(root, file) != layerDomain
		adapter := strings.HasSuffix(suffix, adapterSuffix) && layerOf(root, file) != layerInfrastructure
		if underDir(file, sourceFolder) && (port || adapter) {
			found = append(found, Violation{File: file})
		}
	}
	return found
}

func MapperLocation(tree Tree, _ Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, file := range tree.Files {
		if underDir(file, sourceFolder) && fileSuffix(file) == suffixMapper && layerOf(root, file) != layerInfrastructure {
			found = append(found, Violation{File: file})
		}
	}
	return found
}

func EventPastTense(tree Tree, _ Graph, _ Settings) []Violation {
	var found []Violation
	for _, file := range tree.Files {
		if !underDir(file, sourceFolder) || fileSuffix(file) != suffixEvent {
			continue
		}
		stem := strings.TrimSuffix(path.Base(file), "."+suffixEvent+path.Ext(file))
		word := stem[strings.LastIndex(stem, wordSeparator)+1:]
		if !strings.HasSuffix(word, pastTenseEnding) && !slices.Contains(irregularPastWords, word) {
			found = append(found, Violation{File: file})
		}
	}
	return found
}

func AppRouterFiles(tree Tree, _ Graph, _ Settings) []Violation {
	var found []Violation
	for _, file := range tree.Files {
		if underDir(file, path.Join(sourceFolder, appFolder)) && !isAppRouterFile(path.Base(file)) {
			found = append(found, Violation{File: file})
		}
	}
	return found
}

func isAppRouterFile(name string) bool {
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	return slices.Contains(routeStems, stem) && IsScript(name) ||
		slices.Contains(metadataStems, stem) ||
		strings.HasSuffix(name, cssModuleSuffix) ||
		name == globalStylesFile
}

func ThinRouteHandler(tree Tree, _ Graph, _ Settings) []Violation {
	var found []Violation
	for _, file := range tree.Files {
		if !underDir(file, path.Join(sourceFolder, appFolder)) || path.Base(file) != routeFile {
			continue
		}
		source, err := tree.Source(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(source), "\n")
		for start, line := range lines {
			if !handlerStart.MatchString(line) {
				continue
			}
			end := start + 1
			for end < len(lines)-1 && !strings.HasPrefix(lines[end], "}") {
				end++
			}
			if end-start+1 > maxHandlerLines {
				found = append(found, Violation{File: file, Line: start + 1})
			}
		}
	}
	return found
}

func ComponentsJSON(tree Tree, _ Graph, _ Settings) []Violation {
	config, exists, err := readComponents(tree)
	if exists && (err != nil || !config.Tailwind.CSSVariables) {
		return []Violation{{File: componentsJSONFile}}
	}
	return nil
}

func UtilsLocation(tree Tree, _ Graph, _ Settings) []Violation {
	config, exists, err := readComponents(tree)
	if !exists || err != nil || config.Aliases.Utils == "" {
		return nil
	}
	alias := config.Aliases.Utils
	for _, prefix := range utilsAliasPrefixes {
		alias = strings.TrimPrefix(alias, prefix)
	}
	if underDir(alias, componentsFolder) {
		return nil
	}
	return []Violation{{File: componentsJSONFile}}
}
