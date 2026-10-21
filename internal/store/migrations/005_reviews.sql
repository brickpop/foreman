-- wiki/product/models/review.md
-- A review groups the merge requests one lead branch opened, one per repo,
-- for the CEO to read and decide at once.
CREATE TABLE reviews (
    seq                  INTEGER PRIMARY KEY AUTOINCREMENT, -- the id is rev-<seq>
    project_id           TEXT NOT NULL,
    branch               TEXT NOT NULL,
    created_at           TEXT NOT NULL,
    changes_requested_at TEXT -- set by Request changes, cleared by the lead's next request_merge
);

ALTER TABLE merge_requests ADD COLUMN review_id TEXT;
-- The branch's head when the CEO last requested changes: "since your last look".
ALTER TABLE merge_requests ADD COLUMN reviewed_head TEXT;

CREATE TABLE review_comments (
    seq        INTEGER PRIMARY KEY AUTOINCREMENT, -- the id is com-<seq>
    review_id  TEXT NOT NULL,
    repo       TEXT NOT NULL DEFAULT '', -- '' for a general comment
    path       TEXT NOT NULL DEFAULT '',
    side       TEXT NOT NULL DEFAULT '', -- new (the branch) or old (main), on a line
    line       INTEGER NOT NULL DEFAULT 0,
    commit_sha TEXT NOT NULL DEFAULT '', -- the branch head it was written on
    snippet    TEXT NOT NULL DEFAULT '', -- the line's text then
    body       TEXT NOT NULL,
    state      TEXT NOT NULL DEFAULT 'draft', -- draft, then sent
    created_at TEXT NOT NULL,
    sent_at    TEXT
);

CREATE INDEX review_comments_of ON review_comments (review_id);

-- The open merge requests from before: one review per branch. Closed ones
-- keep none: there is nothing left to review.
INSERT INTO reviews (project_id, branch, created_at)
    SELECT project_id, branch, MIN(created_at) FROM merge_requests
    WHERE state = 'open' AND project_id IS NOT NULL
    GROUP BY project_id, branch ORDER BY MIN(seq);
UPDATE merge_requests SET review_id = (
    SELECT 'rev-' || r.seq FROM reviews r
    WHERE r.project_id = merge_requests.project_id AND r.branch = merge_requests.branch)
    WHERE state = 'open';

-- A review request is the review's, no longer one merge request's: one open
-- per review, the oldest kept.
ALTER TABLE requests ADD COLUMN review_id TEXT;
UPDATE requests SET review_id = (
    SELECT m.review_id FROM merge_requests m WHERE 'mr-' || m.seq = requests.merge_request_id)
    WHERE kind = 'review';
UPDATE requests SET status = 'resolved', answer = 'one review per branch now',
    resolved_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
    WHERE kind = 'review' AND status = 'open' AND seq NOT IN (
        SELECT MIN(seq) FROM requests WHERE kind = 'review' AND status = 'open' GROUP BY review_id);
ALTER TABLE requests DROP COLUMN merge_request_id;
