-- The hash of the session prompt an instance's current run started with, so
-- Foreman can tell when a team, project or settings change left it behind
-- (wiki/product/models/team-definition.md#reload); '' before it was kept.
ALTER TABLE instances ADD COLUMN prompt_hash TEXT NOT NULL DEFAULT '';
