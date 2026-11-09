package main

import (
	"context"
	"time"

	gatepersistence "github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
	livepersistence "github.com/90p-labs/harness/internal/live/infrastructure/persistence"
	runmodule "github.com/90p-labs/harness/internal/run"
	"github.com/90p-labs/harness/internal/run/application/command/decidestop"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/persistence"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

type stopAdapter struct {
	cfg       config.Config
	commonDir string
	key       domain.RunKey
}

func (s stopAdapter) DecideStop(ctx context.Context, agentRunning bool) (bool, string, error) {
	db, err := openState(ctx, s.cfg, s.commonDir)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = db.Close() }()

	verdict, err := runmodule.NewStopDecider(db, s.cfg.Run, time.Now).Handle(ctx, decidestop.Command{Key: s.key, AgentRunning: agentRunning})
	return verdict.Block, verdict.Reason, err
}

func openState(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
	db, err := dbal.Open(ctx, cfg.State.DSN, commonDir)
	if err != nil {
		return nil, err
	}
	err = db.Migrate(ctx, persistence.Module, persistence.Migrations)
	if err == nil {
		err = db.Migrate(ctx, gatepersistence.Module, gatepersistence.Migrations)
	}
	if err == nil {
		err = db.Migrate(ctx, livepersistence.Module, livepersistence.Migrations)
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func modelsByHost(roles map[string]config.RoleHost) map[shareddomain.Host]map[string][]string {
	models := map[shareddomain.Host]map[string][]string{}
	for role, hosts := range roles {
		for host, list := range hosts {
			h := shareddomain.Host(host)
			if models[h] == nil {
				models[h] = map[string][]string{}
			}
			models[h][role] = list
		}
	}
	return models
}
