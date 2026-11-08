CREATE TABLE gate_evidence (
    repo        TEXT NOT NULL,
    gate_id     TEXT NOT NULL,
    input_hash  TEXT NOT NULL,
    result      TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    tail        TEXT NOT NULL,
    at          TIMESTAMP NOT NULL,
    PRIMARY KEY (repo, gate_id, input_hash)
);

CREATE TABLE commit_checks (
    repo     TEXT NOT NULL,
    head_sha TEXT NOT NULL,
    base     TEXT NOT NULL,
    passed   INTEGER NOT NULL,
    at       TIMESTAMP NOT NULL,
    PRIMARY KEY (repo, head_sha)
);
