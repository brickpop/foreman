-- wiki/product/models/review.md
-- The CEO's note with the last Request changes, for the lead's reviews() to
-- read again once its session lost the message.
ALTER TABLE reviews ADD COLUMN changes_note TEXT NOT NULL DEFAULT '';
