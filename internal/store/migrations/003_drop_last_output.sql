-- Never written: a stuck agent is found by the hooks it last sent, kept in
-- memory (wiki/product/architecture/agent-runtime.md#stuck-agents).
ALTER TABLE instances DROP COLUMN last_output_at;
