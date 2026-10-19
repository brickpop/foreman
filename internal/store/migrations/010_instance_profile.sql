-- The profile an instance's current run started on
-- (wiki/product/models/instance.md): Project settings says which running
-- agents still run on another one. '' for a run from before this column.
ALTER TABLE instances ADD COLUMN profile TEXT NOT NULL DEFAULT '';
