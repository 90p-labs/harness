package docs_test

import (
	"reflect"
	"strings"
	"testing"

	deploydomain "github.com/90p-labs/harness/internal/deploy/domain"
	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	lintdomain "github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	configPage    = "docs/configuration.md"
	keyTag        = "mapstructure"
	keySeparator  = "."
	guardSection  = "guard"
	lintSection   = "lint"
	deploySection = "deploy"
	rolesSection  = "roles"
)

func TestConfigurationPage(t *testing.T) {
	page := repository.Read(t, configPage)
	if strings.Contains(page, repository.EmDash) || strings.ContainsFunc(page, repository.IsEmoji) {
		t.Errorf("%s has an em dash or an emoji", configPage)
	}

	sections := map[string]string{}
	top := reflect.TypeFor[config.Config]()
	for i := range top.NumField() {
		field := top.Field(i)
		key := field.Tag.Get(keyTag)
		body := repository.Section(t, page, "\n## `"+key+"`\n")
		sections[key] = body
		for _, path := range leafPaths(field.Type, "") {
			if !strings.Contains(body, "\n| `"+path+"` |") {
				t.Errorf("%s: the section %s has no row for `%s`", configPage, key, path)
			}
		}
	}

	rows := map[string][]string{guardSection: {guarddomain.RuleStopHoldsTurn, guarddomain.RuleReviewModels}}
	for _, rule := range guarddomain.Rules() {
		rows[guardSection] = append(rows[guardSection], rule.ID)
	}
	for _, pack := range lintdomain.AllPacks {
		rows[lintSection] = append(rows[lintSection], string(pack))
	}
	rows[deploySection] = deploydomain.Tools
	for _, role := range []shareddomain.Role{shareddomain.RoleLead, shareddomain.RoleWorker, shareddomain.RoleReviewer, shareddomain.RoleExplorer, shareddomain.RoleAdvisor} {
		rows[rolesSection] = append(rows[rolesSection], string(role))
	}
	for _, host := range []shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex} {
		rows[rolesSection] = append(rows[rolesSection], string(host))
	}
	for key, names := range rows {
		for _, name := range names {
			if !strings.Contains(sections[key], "\n| `"+name+"` |") {
				t.Errorf("%s: the section %s has no row for `%s`", configPage, key, name)
			}
		}
	}
}

func leafPaths(typ reflect.Type, prefix string) []string {
	if typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.Struct {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		if prefix == "" {
			return nil
		}
		return []string{prefix}
	}
	var paths []string
	for i := range typ.NumField() {
		field := typ.Field(i)
		path := field.Tag.Get(keyTag)
		if prefix != "" {
			path = prefix + keySeparator + path
		}
		paths = append(paths, leafPaths(field.Type, path)...)
	}
	return paths
}
