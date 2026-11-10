CREATE TABLE agent_marks (
    run_id     TEXT NOT NULL,
    kind       TEXT NOT NULL,
    id         TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    PRIMARY KEY (run_id, kind, id)
);
