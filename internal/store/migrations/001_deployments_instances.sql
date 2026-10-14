-- wiki/product/models/deployment.md
CREATE TABLE deployments (
    id         TEXT PRIMARY KEY,           -- team@project
    team_id    TEXT NOT NULL,
    project_id TEXT NOT NULL,
    goal       TEXT NOT NULL CHECK (trim(goal) <> ''),
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'stopped')),
    created_at TEXT NOT NULL,
    stopped_at TEXT,
    UNIQUE (team_id, project_id)           -- one row per pair: re-deploying reactivates it
);

-- wiki/product/models/instance.md
CREATE TABLE instances (
    id                 TEXT PRIMARY KEY,   -- agent@project
    deployment_id      TEXT NOT NULL REFERENCES deployments (id),
    agent_id           TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'starting'
                       CHECK (status IN ('queued', 'starting', 'running', 'idle', 'awaiting', 'done', 'stopped')),
    task               TEXT,
    pid                INTEGER,
    pty_id             TEXT,
    sandbox_path       TEXT NOT NULL,
    harness            TEXT NOT NULL,
    harness_session_id TEXT,
    started_at         TEXT,
    last_output_at     TEXT,
    restarts           INTEGER NOT NULL DEFAULT 0 CHECK (restarts >= 0)
);

CREATE INDEX instances_by_deployment ON instances (deployment_id);
