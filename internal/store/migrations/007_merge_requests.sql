-- wiki/product/models/merge-request.md
-- A lead's request to merge one of its lead branches into a repo's main:
-- at most one open per (project, repo, branch).
CREATE TABLE merge_requests (
    seq             INTEGER PRIMARY KEY AUTOINCREMENT, -- the id is mr-<seq>
    project_id      TEXT,
    deployment_id   TEXT REFERENCES deployments (id),
    author_instance TEXT NOT NULL,
    repo            TEXT NOT NULL,
    branch          TEXT NOT NULL,
    entry           TEXT,
    state           TEXT NOT NULL DEFAULT 'open',
    checks_status   TEXT,
    conflicting     INTEGER NOT NULL DEFAULT 0,
    forge_pr_url    TEXT,
    forge_pr_number INTEGER NOT NULL DEFAULT 0,
    note            TEXT,
    created_at      TEXT NOT NULL,
    merged_at       TEXT,
    rejected_at     TEXT
);

CREATE UNIQUE INDEX merge_requests_one_open ON merge_requests (project_id, repo, branch) WHERE state = 'open';
