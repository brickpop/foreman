-- wiki/product/models/deployment.md#invariants
-- A project has one team at most: one active deployment per project.
CREATE UNIQUE INDEX deployments_one_active_per_project ON deployments (project_id) WHERE status = 'active';
