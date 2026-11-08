CREATE TABLE releases (
    run_id      TEXT PRIMARY KEY,
    pr_url      TEXT NOT NULL,
    merge_sha   TEXT NOT NULL,
    merged_at   TEXT NOT NULL,
    deployed_at TEXT NOT NULL,
    environment TEXT NOT NULL
);
