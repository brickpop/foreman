-- wiki/product/models/project-config.md#the-project-record
-- An archived project is put away: out of the sidebar's projects and the
-- Dashboard, and not deployable, until it is unarchived.
ALTER TABLE projects ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
