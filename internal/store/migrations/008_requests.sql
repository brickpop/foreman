-- wiki/product/models/request.md
-- What waits for the CEO. Today: a merge request's review.
CREATE TABLE requests (
    seq              INTEGER PRIMARY KEY AUTOINCREMENT, -- the id is req-<seq>
    kind             TEXT NOT NULL,
    project_id       TEXT,
    deployment_id    TEXT,
    instance_id      TEXT,
    summary          TEXT NOT NULL,
    options          TEXT NOT NULL,  -- JSON array
    source           TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'open',
    answer           TEXT,
    created_at       TEXT NOT NULL,
    resolved_at      TEXT,
    merge_request_id TEXT,
    links            TEXT NOT NULL   -- JSON array
);

CREATE INDEX requests_status ON requests (status);
