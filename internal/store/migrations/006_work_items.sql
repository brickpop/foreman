-- wiki/product/models/work-item.md
-- One agent's branch on its way to the lead: one row per branch of a
-- deployment, its stage moved by hand-offs, deliveries and integration.
CREATE TABLE work_items (
    seq              INTEGER PRIMARY KEY AUTOINCREMENT, -- the id is wi-<seq>
    deployment_id    TEXT NOT NULL REFERENCES deployments (id),
    author_instance  TEXT NOT NULL,
    branch           TEXT NOT NULL,
    card_ref         TEXT,
    stage            INTEGER NOT NULL DEFAULT 0,
    note             TEXT,
    checks_status    TEXT,
    merge_request_id TEXT,
    UNIQUE (deployment_id, branch)
);
