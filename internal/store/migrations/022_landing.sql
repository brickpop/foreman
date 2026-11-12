-- Where a merge request lands, when not on its target (the CEO, 2026-11-11):
-- a branch made at the target's head on Merge, named by the lead (under the
-- project's landing prefixes) or by the CEO; '' lands on the target itself
-- (wiki/product/features/pipeline-and-merges.md#where-a-merge-goes).
ALTER TABLE merge_requests ADD COLUMN landing TEXT NOT NULL DEFAULT '';
