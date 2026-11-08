package domain

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	RuleDomainPurity         = "ddd/domain-purity"
	RuleLayerDirection       = "ddd/layer-direction"
	RuleCrossContext         = "ddd/cross-context"
	RuleClientServerBoundary = "next/client-server-boundary"
	RuleUIImports            = "shadcn/ui-imports"
	RuleComponentsNoBusiness = "combo/components-no-business"
	RuleAppThroughKernel     = "combo/app-through-kernel"
	serverOnlyPackage        = "server-only"
	nextHeadersPackage       = "next/headers"
	uiFolder                 = "ui"
	eventHandlersApplication = layerApplication + pathSeparator + eventHandlersFolder
)

var (
	uiAllowedPackages = []string{"@radix-ui", "radix-ui", "class-variance-authority", "lucide-react", "react", "react-dom"}

	useClientDirective = regexp.MustCompile(`(?s)\A(?:\s|//[^\n]*|/\*.*?\*/)*['"]use client['"]`)
)

func DomainPurity(_ Tree, graph Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, edge := range graph.Edges {
		if layerOf(root, edge.From) != layerDomain {
			continue
		}
		toLayer := layerOf(root, edge.To)
		forbidden := toLayer == layerApplication || toLayer == layerInfrastructure
		if edge.Package && !isAllowed(edge.Specifier, settings.DDD.DomainAllow) || !edge.Package && forbidden {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func LayerDirection(_ Tree, graph Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, edge := range graph.Edges {
		if layerOf(root, edge.From) == layerApplication && layerOf(root, edge.To) == layerInfrastructure {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func CrossContext(tree Tree, graph Graph, settings Settings) []Violation {
	l := newLayout(tree, settings)
	var found []Violation
	for _, edge := range graph.Edges {
		from, to := l.contextOf(edge.From), l.contextOf(edge.To)
		if edge.Package || from == "" || to == "" || from == to {
			continue
		}
		moduleToModule := path.Dir(edge.From) == from && fileSuffix(edge.From) == suffixModule &&
			path.Dir(edge.To) == to && fileSuffix(edge.To) == suffixModule
		handlerToEvent := underDir(edge.From, path.Join(from, eventHandlersApplication)) && fileSuffix(edge.To) == suffixEvent
		if !moduleToModule && !handlerToEvent {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func ClientServerBoundary(tree Tree, graph Graph, _ Settings) []Violation {
	serverOnly := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.Package && edge.Specifier == serverOnlyPackage {
			serverOnly[edge.From] = true
		}
	}
	clients := map[string]bool{}
	var found []Violation
	for _, edge := range graph.Edges {
		isClient, known := clients[edge.From]
		if !known {
			source, err := tree.Source(edge.From)
			isClient = err == nil && useClientDirective.Match(source)
			clients[edge.From] = isClient
		}
		reachesServer := edge.Package && (edge.Specifier == serverOnlyPackage || edge.Specifier == nextHeadersPackage) ||
			!edge.Package && serverOnly[edge.To]
		if isClient && reachesServer {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func UIImports(tree Tree, graph Graph, settings Settings) []Violation {
	config, _, _ := readComponents(tree)
	uiDir := path.Join(sourceFolder, componentsFolder, uiFolder)
	var found []Violation
	for _, edge := range graph.Edges {
		if !underDir(edge.From, uiDir) {
			continue
		}
		allowed := isAllowed(edge.Specifier, settings.Shadcn.UIAllow) ||
			config.Aliases.Utils != "" && edge.Specifier == config.Aliases.Utils ||
			edge.Package && isAllowed(edge.Specifier, uiAllowedPackages) ||
			!edge.Package && underDir(edge.To, uiDir)
		if !allowed {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func ComponentsNoBusiness(_ Tree, graph Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, edge := range graph.Edges {
		if underDir(edge.From, path.Join(sourceFolder, componentsFolder)) && !edge.Package && underDir(edge.To, root) {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func AppThroughKernel(_ Tree, graph Graph, settings Settings) []Violation {
	root := businessRoot(settings)
	var found []Violation
	for _, edge := range graph.Edges {
		if underDir(edge.From, path.Join(sourceFolder, appFolder)) && !edge.Package && underDir(edge.To, root) && edge.To != path.Join(root, kernelFile) {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

func isAllowed(specifier string, allowed []string) bool {
	return slices.ContainsFunc(allowed, func(name string) bool {
		return specifier == name || strings.HasPrefix(specifier, name+pathSeparator)
	})
}
