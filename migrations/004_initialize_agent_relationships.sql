-- +goose Up
INSERT INTO agent_relationships(id,game_id,from_agent_id,to_agent_id)
SELECT gen_random_uuid(),a.game_id,a.id,b.id FROM agents a JOIN agents b ON a.game_id=b.game_id AND a.id<>b.id
ON CONFLICT(game_id,from_agent_id,to_agent_id) DO NOTHING;
-- +goose Down
-- Deliberately preserve relationship data. Roll back feature flags instead of deleting history.
SELECT 1;
