package config

import (
	"github.com/90p-labs/harness/internal/lint/domain"
	sharedconfig "github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

func Decode(cfg sharedconfig.Lint) domain.Settings {
	return domain.Settings{
		DDD:    domain.DDD{Root: cfg.DDD.Root, DomainAllow: cfg.DDD.DomainAllow, SrcFiles: cfg.DDD.SrcFiles},
		Shadcn: domain.Shadcn{UIAllow: cfg.Shadcn.UIAllow},
		Observability: domain.Observability{
			Chart:      cfg.Observability.Chart,
			Rules:      cfg.Observability.Rules,
			Dashboards: cfg.Observability.Dashboards,
		},
	}
}
