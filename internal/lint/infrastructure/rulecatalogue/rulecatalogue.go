package rulecatalogue

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
)

//go:embed rules.yaml
var rules []byte

func Load() (domain.RuleCatalogue, error) {
	var catalogue domain.RuleCatalogue
	if err := yaml.Unmarshal(rules, &catalogue); err != nil {
		return domain.RuleCatalogue{}, fmt.Errorf("decode the rule catalogue: %w", err)
	}
	return catalogue, nil
}
