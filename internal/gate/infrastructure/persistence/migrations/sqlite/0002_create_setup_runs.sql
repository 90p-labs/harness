CREATE TABLE setup_runs (
    repo       TEXT NOT NULL,
    step_id    TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    at         TIMESTAMP NOT NULL,
    PRIMARY KEY (repo, step_id)
);
