-- commit names its repos, each with its own message
-- (wiki/product/architecture/protocol.md#commit): the message moves into each
-- repo of the action. A commit refused before reaching any repo (nothing to
-- commit, no message) keeps no message: there is no repo to hold it.
UPDATE team_actions
SET repos = (SELECT json_group_array(json_set(r.value, '$.message', team_actions.message)) FROM json_each(team_actions.repos) AS r)
WHERE tool = 'commit' AND message IS NOT NULL AND json_array_length(repos) > 0;
ALTER TABLE team_actions DROP COLUMN message;
