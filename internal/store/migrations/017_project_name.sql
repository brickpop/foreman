-- A project's display name, which the CEO can change at any time; "" shows it
-- by its id. The id stays: instance ids, branches and folders are built from
-- it (wiki/product/models/project-config.md#the-project-record).
ALTER TABLE projects ADD COLUMN name TEXT NOT NULL DEFAULT '';
