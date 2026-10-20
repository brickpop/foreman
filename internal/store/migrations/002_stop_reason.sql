-- Why a deployment stopped when Foreman stopped it on its own (its project
-- gone, wiki/product/models/deployment.md#lifecycle); '' when the CEO did.
ALTER TABLE deployments ADD COLUMN stop_reason TEXT NOT NULL DEFAULT '';
