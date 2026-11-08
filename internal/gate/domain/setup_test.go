package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/gate/domain"
)

func TestNewSetup(t *testing.T) {
	step, err := domain.NewSetup("deps", "composer install", []string{"composer.lock"}, "vendor/autoload.php")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Setup{
		Gate:    domain.Gate{ID: "deps", Run: "composer install", Paths: []string{"composer.lock"}, Timeout: domain.DefaultTimeout},
		Creates: "vendor/autoload.php",
	}
	if !reflect.DeepEqual(step, want) {
		t.Errorf("step = %+v, want %+v", step, want)
	}

	for name, bad := range map[string]struct{ id, run, creates string }{
		"bad id":           {id: "Deps", run: "x"},
		"no run":           {id: "deps"},
		"absolute creates": {id: "deps", run: "x", creates: "/vendor/autoload.php"},
		"outside creates":  {id: "deps", run: "x", creates: "../vendor/autoload.php"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewSetup(bad.id, bad.run, nil, bad.creates)
			if !errors.Is(err, domain.ErrInvalidSetup) || !strings.Contains(err.Error(), `setup step "`+bad.id+`"`) {
				t.Errorf("err = %v, want an invalid setup error that names the step", err)
			}
		})
	}
}

func TestValidateSetup(t *testing.T) {
	deps, _ := domain.NewSetup("deps", "make deps", nil, "")
	tools, _ := domain.NewSetup("tools", "make tools", nil, "")
	if err := domain.ValidateSetup([]domain.Setup{deps, tools}); err != nil {
		t.Errorf("unique ids: %v", err)
	}
	err := domain.ValidateSetup([]domain.Setup{deps, deps})
	if !errors.Is(err, domain.ErrInvalidSetup) || !strings.Contains(err.Error(), `setup step "deps": duplicate id`) {
		t.Errorf("duplicate err = %v", err)
	}
}
