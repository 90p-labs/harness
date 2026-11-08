package config

import (
	"slices"

	"github.com/jorge07/harness/internal/lint/domain"
	sharedconfig "github.com/jorge07/harness/internal/shared/infrastructure/config"
)

func Decode(cfg sharedconfig.Lint) ([]domain.PackName, domain.Settings, error) {
	var packs []domain.PackName
	for _, name := range cfg.Packs {
		pack, err := domain.ParsePackName(name)
		if err != nil {
			return nil, domain.Settings{}, err
		}
		if !slices.Contains(packs, pack) {
			packs = append(packs, pack)
		}
	}
	return packs, domain.Settings{
		DDD:    domain.DDD{Root: cfg.DDD.Root, DomainAllow: cfg.DDD.DomainAllow, SrcFiles: cfg.DDD.SrcFiles},
		Shadcn: domain.Shadcn{UIAllow: cfg.Shadcn.UIAllow},
		Observability: domain.Observability{
			Chart:      cfg.Observability.Chart,
			Rules:      cfg.Observability.Rules,
			Dashboards: cfg.Observability.Dashboards,
		},
	}, nil
}
