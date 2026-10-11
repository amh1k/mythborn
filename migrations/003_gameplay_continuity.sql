-- +goose Up
CREATE TABLE mysteries (
 id uuid PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
 title text NOT NULL CHECK(length(title) BETWEEN 1 AND 180), question text NOT NULL CHECK(length(question) BETWEEN 1 AND 600),
 state text NOT NULL CHECK(state IN ('open','resolved')),
 explanations jsonb NOT NULL CHECK(jsonb_typeof(explanations)='array' AND jsonb_array_length(explanations) BETWEEN 1 AND 3 AND pg_column_size(explanations)<=8192),
 evidence_summary text NOT NULL CHECK(length(evidence_summary) BETWEEN 1 AND 1500), next_observation text NOT NULL DEFAULT '' CHECK(length(next_observation)<=500),
 origin_round_id uuid NOT NULL, latest_round_id uuid NOT NULL, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(game_id,id),
 FOREIGN KEY(game_id,origin_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,latest_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE
);
CREATE INDEX mysteries_game_state_updated_idx ON mysteries(game_id,state,updated_at DESC,id);
CREATE INDEX mysteries_origin_idx ON mysteries(game_id,origin_round_id);
CREATE INDEX mysteries_latest_idx ON mysteries(game_id,latest_round_id);
CREATE TABLE mystery_revisions (
 id uuid PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE, mystery_id uuid NOT NULL, source_round_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version>0), kind text NOT NULL CHECK(kind IN ('opened','advanced','resolved','reopened')),
 before_snapshot jsonb, after_snapshot jsonb NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 1000), created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(before_snapshot IS NULL OR (jsonb_typeof(before_snapshot)='object' AND pg_column_size(before_snapshot)<=16384)),
 CHECK(jsonb_typeof(after_snapshot)='object' AND pg_column_size(after_snapshot)<=16384),
 UNIQUE(mystery_id,version), UNIQUE(mystery_id,source_round_id),
 FOREIGN KEY(game_id,mystery_id) REFERENCES mysteries(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,source_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE
);
CREATE INDEX mystery_revisions_source_idx ON mystery_revisions(game_id,source_round_id);
CREATE TABLE agent_relationships (
 id uuid PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE, from_agent_id uuid NOT NULL, to_agent_id uuid NOT NULL,
 trust text NOT NULL DEFAULT 'neutral' CHECK(trust IN ('cautious','neutral','trusting')),
 rapport text NOT NULL DEFAULT 'neutral' CHECK(rapport IN ('tense','neutral','warm')),
 summary text NOT NULL DEFAULT '' CHECK(length(summary)<=800), latest_round_id uuid, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(game_id,id), UNIQUE(game_id,from_agent_id,to_agent_id), CHECK(from_agent_id<>to_agent_id),
 FOREIGN KEY(game_id,from_agent_id) REFERENCES agents(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,to_agent_id) REFERENCES agents(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,latest_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE
);
CREATE INDEX relationships_to_idx ON agent_relationships(game_id,to_agent_id);
CREATE INDEX relationships_latest_idx ON agent_relationships(game_id,latest_round_id);
CREATE TABLE agent_relationship_revisions (
 id uuid PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE, relationship_id uuid NOT NULL, source_round_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version>1), before_snapshot jsonb NOT NULL, after_snapshot jsonb NOT NULL,
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 1000), created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(jsonb_typeof(before_snapshot)='object' AND pg_column_size(before_snapshot)<=8192), CHECK(jsonb_typeof(after_snapshot)='object' AND pg_column_size(after_snapshot)<=8192),
 UNIQUE(relationship_id,version), UNIQUE(relationship_id,source_round_id),
 FOREIGN KEY(game_id,relationship_id) REFERENCES agent_relationships(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,source_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE
);
CREATE INDEX relationship_revisions_source_idx ON agent_relationship_revisions(game_id,source_round_id);
CREATE TABLE tradition_influences (
 id uuid PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE, tradition_id uuid NOT NULL, source_round_id uuid NOT NULL, agent_id uuid NOT NULL,
 tradition_version bigint NOT NULL CHECK(tradition_version>0), tradition_state text NOT NULL CHECK(tradition_state IN ('adopted','contested','retired')),
 category text NOT NULL CHECK(category IN ('interpretation','precaution','challenge','suggestion')),
 explanation text NOT NULL CHECK(length(explanation) BETWEEN 1 AND 1000), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tradition_id,agent_id,source_round_id,category),
 FOREIGN KEY(game_id,tradition_id) REFERENCES traditions(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,source_round_id) REFERENCES rounds(game_id,id) ON DELETE CASCADE,
 FOREIGN KEY(game_id,agent_id) REFERENCES agents(game_id,id) ON DELETE CASCADE
);
CREATE INDEX tradition_influences_history_idx ON tradition_influences(game_id,tradition_id,created_at DESC,id);
CREATE INDEX tradition_influences_round_idx ON tradition_influences(game_id,source_round_id);
CREATE INDEX tradition_influences_agent_idx ON tradition_influences(game_id,agent_id);

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_gameplay_source() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE source_kind text; source_status text;
BEGIN
 SELECT kind,status INTO source_kind,source_status FROM rounds WHERE game_id=NEW.game_id AND id=NEW.source_round_id;
 IF source_status IS DISTINCT FROM 'complete' THEN
  RAISE EXCEPTION 'gameplay revision requires completed source round' USING ERRCODE='23514';
 END IF;
 IF (TG_TABLE_NAME='mystery_revisions' AND source_kind IS DISTINCT FROM 'discovery') OR
    (TG_TABLE_NAME<>'mystery_revisions' AND source_kind NOT IN ('discovery','council')) THEN
  RAISE EXCEPTION 'invalid gameplay source kind' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER mystery_revision_source AFTER INSERT OR UPDATE ON mystery_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_gameplay_source();
CREATE CONSTRAINT TRIGGER relationship_revision_source AFTER INSERT OR UPDATE ON agent_relationship_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_gameplay_source();
CREATE CONSTRAINT TRIGGER tradition_influence_source AFTER INSERT OR UPDATE ON tradition_influences DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_gameplay_source();

-- +goose Down
DROP TABLE tradition_influences;
DROP TABLE agent_relationship_revisions;
DROP TABLE agent_relationships;
DROP TABLE mystery_revisions;
DROP TABLE mysteries;
DROP FUNCTION IF EXISTS mythborn_validate_gameplay_source();
