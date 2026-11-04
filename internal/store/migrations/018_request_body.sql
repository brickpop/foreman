-- A request's body: what it says beyond its summary, its one-line title
-- (wiki/product/models/request.md). "" for none.
ALTER TABLE requests ADD COLUMN body TEXT NOT NULL DEFAULT '';
