-- Foreman never talks to a forge (the CEO, 2026-11-15): merges stay local and
-- the CEO pushes code with their own git (wiki/backlog/remove-forge.md).
ALTER TABLE merge_requests DROP COLUMN forge_pr_url;
ALTER TABLE merge_requests DROP COLUMN forge_pr_number;
