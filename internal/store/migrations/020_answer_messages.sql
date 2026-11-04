-- The CEO's answer to an escalation is a message of kind 'answer'
-- (wiki/product/models/team-message.md), which the kind's CHECK never
-- allowed: every answer failed to be saved, and so never reached the lead
-- (found 2026-11-04). SQLite can't change a CHECK, so the table is made again,
-- the same but for 'answer'. Nothing references it.
CREATE TABLE team_messages_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployments (id),
    ref           TEXT NOT NULL,             -- msg-N, per deployment
    from_instance TEXT NOT NULL,             -- an instance of the deployment, or foreman / ceo
    to_instance   TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('ask', 'reply', 'handoff', 'contribute', 'notice', 'answer')),
    in_reply_to   TEXT,                      -- a reply's ask, an answer's request
    body          TEXT NOT NULL,
    artifact_ref  TEXT,
    state         TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'delivered', 'expired', 'failed')),
    reason        TEXT,
    created_at    TEXT NOT NULL,
    delivered_at  TEXT,
    UNIQUE (deployment_id, ref)
);
INSERT INTO team_messages_new SELECT * FROM team_messages;
DROP TABLE team_messages;
ALTER TABLE team_messages_new RENAME TO team_messages;
CREATE INDEX team_messages_queued ON team_messages (to_instance, state);
