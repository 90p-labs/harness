package harnessengine

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
)

const (
	kindDeployment = "Deployment"
	kindHPA        = "HorizontalPodAutoscaler"
	latestTag      = "latest"
	digestMarker   = "@"
	tagMarker      = ":"
	slash          = "/"
	sourceFile     = 1
)

var (
	documentSeparator = regexp.MustCompile(`(?m)^---[ \t]*$`)
	sourceComment     = regexp.MustCompile(`(?m)^# Source: (\S+)`)
	errNoSource       = errors.New("the rendered manifest has no `# Source:` comment")
)

type probe struct {
	HTTPGet struct {
		Path string `yaml:"path"`
	} `yaml:"httpGet"`
}

type container struct {
	Image          string `yaml:"image"`
	ReadinessProbe *probe `yaml:"readinessProbe"`
	LivenessProbe  *probe `yaml:"livenessProbe"`
}

type chartResource struct {
	file     string
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		ScaleTargetRef struct {
			Name string `yaml:"name"`
		} `yaml:"scaleTargetRef"`
		Template struct {
			Spec struct {
				Containers []container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

var chartChecks = map[string]func([]chartResource) []domain.Violation{
	domain.RuleProbes:              missingProbes,
	domain.RuleLivenessProcessOnly: livenessProcessOnly,
	domain.RuleImmutableTags:       mutableTags,
	domain.RuleHPATarget:           orphanHPAs,
}

type RenderChart func(ctx context.Context, dir, chart string) ([]byte, error)

func renderChart(ctx context.Context, render RenderChart, dir string, tree domain.Tree, chart string, rules []domain.Rule) ([]chartResource, error) {
	wanted := slices.ContainsFunc(rules, func(rule domain.Rule) bool { _, ok := chartChecks[rule.ID]; return ok })
	if !wanted || !slices.ContainsFunc(tree.Files, func(file string) bool { return strings.HasPrefix(file, chart+slash) }) {
		return nil, nil
	}
	out, err := render(ctx, dir, chart)
	if err != nil {
		return nil, fmt.Errorf("the chart rules: %w", err)
	}
	var resources []chartResource
	for _, document := range documentSeparator.Split(string(out), -1) {
		if strings.TrimSpace(document) == "" {
			continue
		}
		source := sourceComment.FindStringSubmatch(document)
		if source == nil {
			return nil, fmt.Errorf("the chart rules: %s: %w", chart, errNoSource)
		}
		var resource chartResource
		if err := yaml.Unmarshal([]byte(document), &resource); err != nil {
			return nil, fmt.Errorf("parse %s: %w", source[sourceFile], err)
		}
		_, rest, _ := strings.Cut(source[sourceFile], slash)
		resource.file = path.Join(chart, rest)
		resources = append(resources, resource)
	}
	return resources, nil
}

func deployments(resources []chartResource) []chartResource {
	return slices.DeleteFunc(slices.Clone(resources), func(resource chartResource) bool { return resource.Kind != kindDeployment })
}

func missingProbes(resources []chartResource) []domain.Violation {
	var found []domain.Violation
	for _, deployment := range deployments(resources) {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			if c.ReadinessProbe == nil || c.LivenessProbe == nil {
				found = append(found, domain.Violation{File: deployment.file})
			}
		}
	}
	return found
}

func livenessProcessOnly(resources []chartResource) []domain.Violation {
	var found []domain.Violation
	for _, deployment := range deployments(resources) {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			if c.ReadinessProbe != nil && c.LivenessProbe != nil && c.LivenessProbe.HTTPGet.Path != "" &&
				c.LivenessProbe.HTTPGet.Path == c.ReadinessProbe.HTTPGet.Path {
				found = append(found, domain.Violation{File: deployment.file})
			}
		}
	}
	return found
}

func mutableTags(resources []chartResource) []domain.Violation {
	var found []domain.Violation
	for _, deployment := range deployments(resources) {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			if mutableImage(c.Image) {
				found = append(found, domain.Violation{File: deployment.file})
			}
		}
	}
	return found
}

func mutableImage(image string) bool {
	if strings.Contains(image, digestMarker) {
		return false
	}
	name := image[strings.LastIndex(image, slash)+1:]
	_, tag, tagged := strings.Cut(name, tagMarker)
	return !tagged || tag == latestTag
}

func orphanHPAs(resources []chartResource) []domain.Violation {
	var names []string
	for _, deployment := range deployments(resources) {
		names = append(names, deployment.Metadata.Name)
	}
	var found []domain.Violation
	for _, resource := range resources {
		if resource.Kind == kindHPA && !slices.Contains(names, resource.Spec.ScaleTargetRef.Name) {
			found = append(found, domain.Violation{File: resource.file})
		}
	}
	return found
}
