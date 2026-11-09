CREATE TABLE review_records (
    repo     TEXT NOT NULL,
    head_sha TEXT NOT NULL,
    model    TEXT NOT NULL,
    at       TIMESTAMP NOT NULL,
    PRIMARY KEY (repo, head_sha, model)
);
