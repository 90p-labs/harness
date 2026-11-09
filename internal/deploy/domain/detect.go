package domain

import (
	"fmt"
	"path"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	GateHelmRender   = "helm-render"
	GatePromtoolTest = "promtool-test"

	chartFile       = "Chart.yaml"
	ruleTestSuffix  = ".rules.test.yaml"
	runHelmRender   = "helm template %s > /dev/null"
	runPromtoolTest = "find %s -name '*" + ruleTestSuffix + "' -exec promtool test rules {} +"
	runAnd          = " && "
	folderSeparator = "/"
	listSeparator   = " "
	rootFolder      = "."
)

func HelmGates(files []string) []shareddomain.ProposedGate {
	var charts []string
	for _, file := range files {
		if path.Base(file) == chartFile {
			charts = append(charts, path.Dir(file))
		}
	}
	slices.Sort(charts)
	var runs []string
	var kept []string
	for _, chart := range charts {
		if slices.ContainsFunc(kept, func(parent string) bool { return below(chart, parent) }) {
			continue
		}
		kept = append(kept, chart)
		runs = append(runs, fmt.Sprintf(runHelmRender, chart))
	}
	if len(runs) == 0 {
		return nil
	}
	return []shareddomain.ProposedGate{{ID: GateHelmRender, Run: strings.Join(runs, runAnd)}}
}

func below(folder, parent string) bool {
	return parent == rootFolder || strings.HasPrefix(folder, parent+folderSeparator)
}

func PromtoolGates(files []string) []shareddomain.ProposedGate {
	var folders []string
	for _, file := range files {
		if !strings.HasSuffix(file, ruleTestSuffix) {
			continue
		}
		top, _, nested := strings.Cut(file, folderSeparator)
		if !nested {
			top = rootFolder
		}
		folders = append(folders, top)
	}
	if len(folders) == 0 {
		return nil
	}
	slices.Sort(folders)
	folders = slices.Compact(folders)
	if slices.Contains(folders, rootFolder) {
		folders = []string{rootFolder}
	}
	return []shareddomain.ProposedGate{{ID: GatePromtoolTest, Run: fmt.Sprintf(runPromtoolTest, strings.Join(folders, listSeparator))}}
}
