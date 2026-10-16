-- wiki/product/architecture/protocol.md#team-actions
-- The git tools agents call (switch, commit, merge, abort_merge), with what
-- they did, for the Team traffic panel.
CREATE TABLE team_actions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployments (id),
    instance      TEXT NOT NULL,
    tool          TEXT NOT NULL,
    branch        TEXT,             -- switch to, merge from
    message       TEXT,             -- commit
    repos         TEXT NOT NULL,    -- JSON: what happened in each repo
    error         TEXT,             -- the refusal the agent got, if any
    created_at    TEXT NOT NULL
);

CREATE INDEX team_actions_deployment ON team_actions (deployment_id);
