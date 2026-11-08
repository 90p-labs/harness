CREATE TABLE runs (
    id           TEXT PRIMARY KEY,
    repo         TEXT NOT NULL,
    worktree     TEXT NOT NULL,
    phase        TEXT NOT NULL,
    review_round INTEGER NOT NULL,
    live_round   INTEGER NOT NULL,
    stop_blocks  INTEGER NOT NULL,
    blocked_from TEXT NOT NULL,
    reason       TEXT NOT NULL,
    version      INTEGER NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX runs_one_active ON runs (repo, worktree)
    WHERE phase NOT IN ('done', 'abandoned');

CREATE TABLE run_transitions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id     TEXT NOT NULL REFERENCES runs (id),
    at         TIMESTAMP NOT NULL,
    from_phase TEXT NOT NULL,
    event      TEXT NOT NULL,
    to_phase   TEXT NOT NULL,
    reason     TEXT NOT NULL
);
