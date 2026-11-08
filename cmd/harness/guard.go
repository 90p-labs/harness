package main

import (
	"context"
	"time"

	runmodule "github.com/jorge07/harness/internal/run"
	"github.com/jorge07/harness/internal/run/application/command/decidestop"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/infrastructure/persistence"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
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
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func modelsByHost(roles map[string]config.RoleHost) map[shareddomain.Host]map[string]string {
	models := map[shareddomain.Host]map[string]string{}
	for role, hosts := range roles {
		for host, model := range hosts {
			h := shareddomain.Host(host)
			if models[h] == nil {
				models[h] = map[string]string{}
			}
			models[h][role] = model
		}
	}
	return models
}
