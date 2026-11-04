-- A review's title and body, as the lead's request_merge gives them: a
-- one-line title and, optionally, what the CEO should know about the work
-- (wiki/product/models/review.md). "" for none.
ALTER TABLE reviews ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE reviews ADD COLUMN body TEXT NOT NULL DEFAULT '';
