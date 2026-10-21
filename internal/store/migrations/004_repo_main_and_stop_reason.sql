-- Each code repo's own main branch, read from its checkout when it is added
-- (wiki/product/models/project-config.md#the-project-record); repos added
-- before were on the project's, which defaulted to main.
ALTER TABLE project_repos ADD COLUMN main TEXT NOT NULL DEFAULT 'main';

-- Why Foreman stopped an instance on its own (a start that failed, a crash
-- loop, wiki/product/models/instance.md); '' otherwise, cleared on each start.
ALTER TABLE instances ADD COLUMN stop_reason TEXT NOT NULL DEFAULT '';
