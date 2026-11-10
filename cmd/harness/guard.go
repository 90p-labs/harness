package main

import (
	"context"
	"sync"
	"time"

	gatepersistence "github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
	hostpersistence "github.com/90p-labs/harness/internal/host/infrastructure/persistence"
	livepersistence "github.com/90p-labs/harness/internal/live/infrastructure/persistence"
	runmodule "github.com/90p-labs/harness/internal/run"
	"github.com/90p-labs/harness/internal/run/application/command/decidestop"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/persistence"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

// stateHolder opens the state database on first use, so a repository with no use of it gets no file.
type stateHolder struct {
	cfg       config.Config
	commonDir string

	mu sync.Mutex
	db *dbal.DB
}

func newStateHolder(cfg config.Config, commonDir string) *stateHolder {
	return &stateHolder{cfg: cfg, commonDir: commonDir}
}

func (h *stateHolder) DB(ctx context.Context) (*dbal.DB, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db == nil {
		db, err := openStateDB(ctx, h.cfg, h.commonDir)
		if err != nil {
			return nil, err
		}
		h.db = db
	}
	return h.db, nil
}

func (h *stateHolder) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db == nil {
		return nil
	}
	err := h.db.Close()
	h.db = nil
	return err
}

type stopAdapter struct {
	cfg   config.Config
	state *stateHolder
	key   domain.RunKey
}

func (s stopAdapter) DecideStop(ctx context.Context, agentRunning bool) (bool, string, error) {
	db, err := s.state.DB(ctx)
	if err != nil {
		return false, "", err
	}
	verdict, err := runmodule.NewStopDecider(db, s.cfg.Run, time.Now).Handle(ctx, decidestop.Command{Key: s.key, AgentRunning: agentRunning})
	return verdict.Block, verdict.Reason, err
}

var openStateDB = openState

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
	if err == nil {
		err = db.Migrate(ctx, hostpersistence.Module, hostpersistence.Migrations)
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
