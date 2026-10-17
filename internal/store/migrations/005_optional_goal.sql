-- A deployment's goal is optional (wiki/product/models/deployment.md): the CEO
-- may give it in the lead's session. SQLite can't change a CHECK, and
-- rebuilding deployments would trip the foreign keys that point at it, so
-- the column is replaced; its own CHECK goes with it.
ALTER TABLE deployments ADD COLUMN goal_new TEXT NOT NULL DEFAULT '';
UPDATE deployments SET goal_new = goal;
ALTER TABLE deployments DROP COLUMN goal;
ALTER TABLE deployments RENAME COLUMN goal_new TO goal;
