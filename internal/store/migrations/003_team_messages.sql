-- wiki/product/models/team-message.md
CREATE TABLE team_messages (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployments (id),
    ref           TEXT NOT NULL,             -- msg-N, per deployment
    from_instance TEXT NOT NULL,             -- an instance of the deployment, or foreman / ceo
    to_instance   TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('ask', 'reply', 'handoff', 'contribute', 'notice')),
    in_reply_to   TEXT,                      -- a reply's ask
    body          TEXT NOT NULL,
    artifact_ref  TEXT,
    state         TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'delivered', 'expired', 'failed')),
    reason        TEXT,
    created_at    TEXT NOT NULL,
    delivered_at  TEXT,
    UNIQUE (deployment_id, ref)
);

-- What waits for a recipient.
CREATE INDEX team_messages_queued ON team_messages (to_instance, state);
