-- The asleep status (wiki/product/models/instance.md#statuses): SQLite can't
-- change a CHECK, so the table is rebuilt. Nothing references instances.
CREATE TABLE instances_new (
    id                 TEXT PRIMARY KEY,   -- agent@project
    deployment_id      TEXT NOT NULL REFERENCES deployments (id),
    agent_id           TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'starting'
                       CHECK (status IN ('asleep', 'queued', 'starting', 'running', 'idle', 'awaiting', 'done', 'stopped')),
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

INSERT INTO instances_new SELECT * FROM instances;
DROP TABLE instances;
ALTER TABLE instances_new RENAME TO instances;
CREATE INDEX instances_by_deployment ON instances (deployment_id);
