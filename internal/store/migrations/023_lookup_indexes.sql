-- Lookups run on every list of reviews (once per review) and of a project's
-- pipeline: without these, each scanned the whole table, and listing every
-- review grew with the square of their number.
CREATE INDEX merge_requests_of_review ON merge_requests (review_id);
CREATE INDEX merge_requests_of_project ON merge_requests (project_id);
