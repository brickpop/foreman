-- wiki/product/models/review.md
-- A review the lead withdrew, or Foreman did when the CEO merged another of
-- the lead's reviews holding all of it (superseded_by, that review's id).
ALTER TABLE reviews ADD COLUMN withdrawn_at TEXT;
ALTER TABLE reviews ADD COLUMN withdrawn_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE reviews ADD COLUMN superseded_by TEXT NOT NULL DEFAULT '';
