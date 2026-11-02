-- Whether an instance's next start starts its conversation fresh: set at
-- deploy unless the agent's last conversation on the project was with the same
-- team and harness, cleared once it starts
-- (wiki/backlog/continue-conversations-on-redeploy.md). A row that never
-- started was a deployment's first start until now: fresh.
ALTER TABLE instances ADD COLUMN fresh INTEGER NOT NULL DEFAULT 0;
UPDATE instances SET fresh = 1 WHERE started_at IS NULL;
