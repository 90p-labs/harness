package gate

import (
	"os"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/application/command/verifycommits"
	"github.com/jorge07/harness/internal/gate/application/query/evidencestatus"
	"github.com/jorge07/harness/internal/gate/domain"
	gateconfig "github.com/jorge07/harness/internal/gate/infrastructure/config"
	"github.com/jorge07/harness/internal/gate/infrastructure/gittree"
	"github.com/jorge07/harness/internal/gate/infrastructure/persistence"
	"github.com/jorge07/harness/internal/gate/infrastructure/runner"
	"github.com/jorge07/harness/internal/gate/infrastructure/toolchain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
)

const instrumentationName = "github.com/jorge07/harness/internal/gate"

var DecodeGates = gateconfig.Decode

func NewVerify(db *dbal.DB, bus shareddomain.Dispatcher, criteria domain.GateSource, now func() time.Time) verify.Handler {
	return verify.NewHandler(
		gittree.Tree{}, runner.Shell{}, persistence.NewSQLEvidenceRepository(db), bus, criteria,
		os.ReadFile, otel.Tracer(instrumentationName), otel.Meter(instrumentationName), now,
	)
}

func NewVerifyCommits(db *dbal.DB, bus shareddomain.Dispatcher, now func() time.Time) verifycommits.Handler {
	return verifycommits.NewHandler(gittree.Tree{}, NewVerify(db, bus, nil, now), persistence.NewSQLCommitCheckRepository(db), now)
}

func NewEvidenceStatus(db *dbal.DB) evidencestatus.Handler {
	return evidencestatus.NewHandler(gittree.Tree{}, persistence.NewSQLEvidenceRepository(db), otel.Tracer(instrumentationName))
}

func NewCommitChecks(db *dbal.DB) domain.CommitCheckRepository {
	return persistence.NewSQLCommitCheckRepository(db)
}

func NewInit() initgates.Handler {
	writer := gateconfig.Writer{}
	return initgates.NewHandler(toolchain.Detect, writer.HasGates, writer.WriteGates)
}
