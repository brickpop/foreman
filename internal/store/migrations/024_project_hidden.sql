-- wiki/product/models/project-config.md#the-project-record
-- Archived becomes hidden (the CEO, 2026-11-13): visibility only, out of the
-- sidebar's projects (into More) and the Dashboard; deployable, removable.
ALTER TABLE projects RENAME COLUMN archived TO hidden;
