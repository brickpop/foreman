-- Where a merge request's branch stood when it left open, so its review still
-- shows the change (wiki/product/models/merge-request.md).
ALTER TABLE merge_requests ADD COLUMN head TEXT;
ALTER TABLE merge_requests ADD COLUMN base TEXT;
