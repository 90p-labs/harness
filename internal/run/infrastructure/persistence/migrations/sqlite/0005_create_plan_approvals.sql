CREATE TABLE plan_approvals (
    run_id      TEXT NOT NULL REFERENCES runs (id),
    plan_sha256 TEXT NOT NULL,
    at          TIMESTAMP NOT NULL,
    PRIMARY KEY (run_id, plan_sha256)
);
