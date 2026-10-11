-- +goose Up
-- New beliefs have version 1 whether copied at founding or formed during play.
-- Keep exactly one attributable source for every non-initial revision.
SET LOCAL lock_timeout = '5s';
ALTER TABLE belief_revisions DROP CONSTRAINT belief_revisions_check1;
ALTER TABLE belief_revisions ADD CONSTRAINT belief_revisions_source_check CHECK (
    (cause = 'initial' AND source_round_id IS NULL AND source_conversation_summary_id IS NULL AND version = 1) OR
    (cause IN ('discovery', 'council') AND source_round_id IS NOT NULL AND source_conversation_summary_id IS NULL) OR
    (cause = 'messenger' AND source_round_id IS NULL AND source_conversation_summary_id IS NOT NULL)
);

-- +goose Down
-- Rollback intentionally fails if version-1 beliefs formed during play exist.
SET LOCAL lock_timeout = '5s';
ALTER TABLE belief_revisions DROP CONSTRAINT belief_revisions_source_check;
ALTER TABLE belief_revisions ADD CONSTRAINT belief_revisions_check1 CHECK (
    (cause = 'initial' AND source_round_id IS NULL AND source_conversation_summary_id IS NULL AND version = 1) OR
    (cause IN ('discovery', 'council') AND source_round_id IS NOT NULL AND source_conversation_summary_id IS NULL AND version > 1) OR
    (cause = 'messenger' AND source_round_id IS NULL AND source_conversation_summary_id IS NOT NULL AND version > 1)
);
