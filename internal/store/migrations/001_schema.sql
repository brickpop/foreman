-- The schema as of the projects anywhere epic
-- (wiki/backlog/epics/projects-anywhere.md); later changes are migrations
-- after this one.

-- wiki/product/models/project-config.md#the-project-record: a project is a record, its repos anywhere
-- on disk. Names are unique in any case: a Mac's filesystem takes two
-- spellings for one folder (the project repo's, under projects/).
CREATE TABLE projects (
    id         TEXT PRIMARY KEY COLLATE NOCASE,
    profile    TEXT NOT NULL DEFAULT '',   -- '' is the default profile
    created_at TEXT NOT NULL
);

CREATE TABLE project_repos (
    project_id  TEXT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name        TEXT NOT NULL COLLATE NOCASE,
    path        TEXT NOT NULL,              -- absolute, as the CEO gave it
    root_commit TEXT NOT NULL,              -- Locate accepts only a folder holding it
    PRIMARY KEY (project_id, name),
    UNIQUE (project_id, path)
);

-- wiki/product/models/deployment.md
CREATE TABLE deployments (
    id         TEXT PRIMARY KEY,           -- team@project
    team_id    TEXT NOT NULL,
    project_id TEXT NOT NULL,
    goal       TEXT NOT NULL DEFAULT '',   -- optional: the CEO may give it in the lead's session
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
                       CHECK (status IN ('asleep', 'queued', 'starting', 'running', 'idle', 'awaiting', 'done', 'stopped')),
    task               TEXT,
    pid                INTEGER,
    pty_id             TEXT,
    sandbox_path       TEXT NOT NULL,
    harness            TEXT NOT NULL,
    profile            TEXT NOT NULL DEFAULT '', -- the one its current run started on
    harness_session_id TEXT,
    started_at         TEXT,
    last_output_at     TEXT,
    restarts           INTEGER NOT NULL DEFAULT 0 CHECK (restarts >= 0)
);

CREATE INDEX instances_by_deployment ON instances (deployment_id);

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

-- wiki/product/architecture/protocol.md#team-actions
-- The git tools agents call (switch, commit, merge, abort_merge), with what
-- they did, for the Team traffic panel.
CREATE TABLE team_actions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployments (id),
    instance      TEXT NOT NULL,
    tool          TEXT NOT NULL,
    branch        TEXT,             -- switch to, merge from
    repos         TEXT NOT NULL,    -- JSON: what happened in each repo (a commit's message included)
    error         TEXT,             -- the refusal the agent got, if any
    created_at    TEXT NOT NULL
);

CREATE INDEX team_actions_deployment ON team_actions (deployment_id);

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
