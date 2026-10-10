-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE accounts (
    id uuid PRIMARY KEY,
    system_role text NOT NULL DEFAULT 'user' CHECK (system_role IN ('user', 'admin')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleting')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE agent_templates (
    id uuid PRIMARY KEY,
    agent_type text NOT NULL CHECK (agent_type IN ('priest', 'scientist', 'soldier', 'historian')),
    version integer NOT NULL CHECK (version > 0),
    display_name text NOT NULL,
    personality_prompt text NOT NULL,
    is_active boolean NOT NULL DEFAULT false,
    updated_by_account_id uuid REFERENCES accounts(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_type, version),
    UNIQUE (id, agent_type)
);
CREATE UNIQUE INDEX agent_templates_one_active_type ON agent_templates(agent_type) WHERE is_active;

CREATE TABLE initial_belief_templates (
    id uuid PRIMARY KEY,
    agent_template_id uuid NOT NULL REFERENCES agent_templates(id) ON DELETE CASCADE,
    claim text NOT NULL,
    initial_state text NOT NULL CHECK (initial_state IN ('forming', 'held', 'questioned', 'abandoned')),
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_template_id, sort_order)
);

CREATE TABLE games (
    id uuid PRIMARY KEY,
    owner_account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    name text NOT NULL,
    player_role text NOT NULL CHECK (player_role IN ('observer', 'god', 'messenger')),
    status text NOT NULL DEFAULT 'starting' CHECK (status IN ('starting', 'active', 'ending_requested', 'ending', 'archived', 'deleting')),
    review_photo_description boolean NOT NULL DEFAULT false,
    last_council_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, owner_account_id)
);
CREATE INDEX games_owner_status_created_idx ON games(owner_account_id, status, created_at DESC);

CREATE TABLE agents (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    source_template_id uuid REFERENCES agent_templates(id) ON DELETE SET NULL,
    agent_type text NOT NULL CHECK (agent_type IN ('priest', 'scientist', 'soldier', 'historian')),
    display_name text NOT NULL,
    personality_prompt text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id),
    UNIQUE (game_id, agent_type)
);
CREATE INDEX agents_source_template_idx ON agents(source_template_id);

CREATE TABLE observations (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    photo_object_path text NOT NULL,
    source_image_sha256 text CHECK (source_image_sha256 IS NULL OR source_image_sha256 ~ '^[0-9a-f]{64}$'),
    mime_type text NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size > 0),
    visual_description text,
    player_correction text,
    player_statement text,
    description_status text NOT NULL DEFAULT 'pending' CHECK (description_status IN ('pending', 'awaiting_review', 'uncertain', 'accepted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    accepted_at timestamptz,
    UNIQUE (game_id, id),
    CHECK (accepted_at IS NULL OR description_status = 'accepted')
);
CREATE INDEX observations_game_hash_idx ON observations(game_id, source_image_sha256) WHERE source_image_sha256 IS NOT NULL;

CREATE TABLE rounds (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    sequence_number bigint NOT NULL CHECK (sequence_number > 0),
    kind text NOT NULL CHECK (kind IN ('discovery', 'council', 'closing')),
    observation_id uuid,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'needs_attention', 'complete', 'failed', 'abandoned')),
    stage text NOT NULL DEFAULT 'queued' CHECK (stage IN ('queued', 'describing', 'awaiting_review', 'awaiting_clarity_choice', 'reacting', 'debating', 'writing', 'complete')),
    idempotency_key text NOT NULL,
    error_code text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id),
    UNIQUE (game_id, sequence_number),
    UNIQUE (game_id, idempotency_key),
    UNIQUE (observation_id),
    FOREIGN KEY (game_id, observation_id) REFERENCES observations(game_id, id) ON DELETE CASCADE,
    CHECK ((kind = 'discovery' AND observation_id IS NOT NULL) OR (kind IN ('council', 'closing') AND observation_id IS NULL)),
    CHECK ((status = 'complete' AND stage = 'complete' AND completed_at IS NOT NULL) OR (status <> 'complete'))
);
CREATE UNIQUE INDEX rounds_one_open_per_game_idx ON rounds(game_id) WHERE status IN ('queued', 'running', 'needs_attention');
CREATE UNIQUE INDEX rounds_one_closing_per_game_idx ON rounds(game_id) WHERE kind = 'closing';
CREATE INDEX rounds_game_sequence_idx ON rounds(game_id, sequence_number DESC);

CREATE TABLE chronicles (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    round_id uuid NOT NULL UNIQUE,
    historian_agent_id uuid NOT NULL,
    title text NOT NULL,
    outcome text CHECK (outcome IS NULL OR outcome IN ('consensus', 'majority', 'unresolved')),
    verdict text NOT NULL,
    body text NOT NULL,
    suggestion text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object' AND pg_column_size(metadata) <= 8192),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id),
    FOREIGN KEY (game_id, round_id) REFERENCES rounds(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, historian_agent_id) REFERENCES agents(game_id, id) ON DELETE RESTRICT
);
CREATE INDEX chronicles_game_created_idx ON chronicles(game_id, created_at DESC);

CREATE TABLE beliefs (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    agent_id uuid NOT NULL,
    claim text NOT NULL,
    state text NOT NULL CHECK (state IN ('forming', 'held', 'questioned', 'abandoned')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id),
    FOREIGN KEY (game_id, agent_id) REFERENCES agents(game_id, id) ON DELETE CASCADE
);
CREATE INDEX beliefs_game_agent_state_idx ON beliefs(game_id, agent_id, state);

CREATE TABLE traditions (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('myth', 'ritual', 'taboo')),
    title text NOT NULL,
    description text NOT NULL,
    state text NOT NULL CHECK (state IN ('adopted', 'contested', 'retired')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id)
);
CREATE INDEX traditions_game_state_type_idx ON traditions(game_id, state, type);

CREATE TABLE conversation_summaries (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    round_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    summary text NOT NULL CHECK (length(summary) <= 12000),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    last_event_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, id),
    UNIQUE (round_id, agent_id),
    FOREIGN KEY (game_id, round_id) REFERENCES rounds(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, agent_id) REFERENCES agents(game_id, id) ON DELETE CASCADE
);
CREATE INDEX conversation_summaries_game_agent_updated_idx ON conversation_summaries(game_id, agent_id, updated_at DESC);

CREATE TABLE belief_revisions (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    belief_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    event_id uuid NOT NULL,
    cause text NOT NULL CHECK (cause IN ('initial', 'discovery', 'council', 'messenger')),
    change_type text NOT NULL CHECK (change_type IN ('formed', 'strengthened', 'weakened', 'revised', 'abandoned')),
    previous_claim text,
    previous_state text CHECK (previous_state IS NULL OR previous_state IN ('forming', 'held', 'questioned', 'abandoned')),
    new_claim text NOT NULL,
    new_state text NOT NULL CHECK (new_state IN ('forming', 'held', 'questioned', 'abandoned')),
    reason text NOT NULL,
    source_round_id uuid,
    source_conversation_summary_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (belief_id, version),
    UNIQUE (belief_id, event_id),
    FOREIGN KEY (game_id, belief_id) REFERENCES beliefs(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, source_round_id) REFERENCES rounds(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, source_conversation_summary_id) REFERENCES conversation_summaries(game_id, id) ON DELETE CASCADE,
    CHECK ((version = 1 AND previous_claim IS NULL AND previous_state IS NULL) OR (version > 1 AND previous_claim IS NOT NULL AND previous_state IS NOT NULL)),
    CHECK ((cause = 'initial' AND source_round_id IS NULL AND source_conversation_summary_id IS NULL AND version = 1) OR
           (cause IN ('discovery', 'council') AND source_round_id IS NOT NULL AND source_conversation_summary_id IS NULL AND version > 1) OR
           (cause = 'messenger' AND source_round_id IS NULL AND source_conversation_summary_id IS NOT NULL AND version > 1))
);
CREATE INDEX belief_revisions_source_round_idx ON belief_revisions(game_id, source_round_id) WHERE source_round_id IS NOT NULL;

CREATE TABLE tradition_supporters (
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    tradition_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    PRIMARY KEY (tradition_id, agent_id),
    FOREIGN KEY (game_id, tradition_id) REFERENCES traditions(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, agent_id) REFERENCES agents(game_id, id) ON DELETE CASCADE
);

CREATE TABLE tradition_revisions (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    tradition_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    event_id uuid NOT NULL,
    source_round_id uuid NOT NULL,
    previous_title text,
    previous_description text,
    previous_state text CHECK (previous_state IS NULL OR previous_state IN ('adopted', 'contested', 'retired')),
    new_title text NOT NULL,
    new_description text NOT NULL,
    new_state text NOT NULL CHECK (new_state IN ('adopted', 'contested', 'retired')),
    previous_supporter_ids uuid[] NOT NULL DEFAULT '{}',
    new_supporter_ids uuid[] NOT NULL DEFAULT '{}',
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tradition_id, version),
    UNIQUE (tradition_id, event_id),
    FOREIGN KEY (game_id, tradition_id) REFERENCES traditions(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, source_round_id) REFERENCES rounds(game_id, id) ON DELETE CASCADE,
    CHECK (cardinality(previous_supporter_ids) <= 4 AND cardinality(new_supporter_ids) <= 4),
    CHECK (version = 1 OR (previous_title IS NOT NULL AND previous_description IS NOT NULL AND previous_state IS NOT NULL)),
    CHECK (version > 1 OR (previous_title IS NULL AND previous_description IS NULL AND previous_state IS NULL))
);
CREATE INDEX tradition_revisions_source_round_idx ON tradition_revisions(game_id, source_round_id);

CREATE TABLE search_documents (
    id uuid PRIMARY KEY,
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    agent_id uuid,
    observation_id uuid,
    chronicle_id uuid,
    belief_id uuid,
    conversation_summary_id uuid,
    content text NOT NULL CHECK (length(content) <= 12000),
    embedding vector(768),
    source_version bigint NOT NULL DEFAULT 1 CHECK (source_version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (game_id, agent_id) REFERENCES agents(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, observation_id) REFERENCES observations(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, chronicle_id) REFERENCES chronicles(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, belief_id) REFERENCES beliefs(game_id, id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, conversation_summary_id) REFERENCES conversation_summaries(game_id, id) ON DELETE CASCADE,
    CHECK (num_nonnulls(observation_id, chronicle_id, belief_id, conversation_summary_id) = 1),
    CHECK ((belief_id IS NULL AND conversation_summary_id IS NULL AND agent_id IS NULL) OR
           (belief_id IS NOT NULL AND agent_id IS NOT NULL) OR
           (conversation_summary_id IS NOT NULL AND agent_id IS NOT NULL))
);
CREATE UNIQUE INDEX search_documents_observation_source_idx ON search_documents(observation_id) WHERE observation_id IS NOT NULL;
CREATE UNIQUE INDEX search_documents_chronicle_source_idx ON search_documents(chronicle_id) WHERE chronicle_id IS NOT NULL;
CREATE UNIQUE INDEX search_documents_belief_source_idx ON search_documents(belief_id) WHERE belief_id IS NOT NULL;
CREATE UNIQUE INDEX search_documents_summary_source_idx ON search_documents(conversation_summary_id) WHERE conversation_summary_id IS NOT NULL;
CREATE INDEX search_documents_game_agent_idx ON search_documents(game_id, agent_id);
CREATE INDEX search_documents_embedding_idx ON search_documents USING hnsw (embedding vector_cosine_ops) WHERE embedding IS NOT NULL;
CREATE INDEX search_documents_content_idx ON search_documents USING gin (to_tsvector('english', content));

CREATE TABLE workflow_outbox (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    game_id uuid,
    round_id uuid,
    command_type text NOT NULL,
    idempotency_key text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (pg_column_size(payload) <= 32768),
    result_payload jsonb CHECK (result_payload IS NULL OR pg_column_size(result_payload) <= 8192),
    result_expires_at timestamptz,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'dispatched', 'processed', 'failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    lease_until timestamptz,
    retry_at timestamptz,
    delivered_at timestamptz,
    processed_at timestamptz,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, command_type, idempotency_key),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (game_id, round_id) REFERENCES rounds(game_id, id) ON DELETE CASCADE,
    CHECK ((result_payload IS NULL) = (result_expires_at IS NULL)),
    CHECK (status <> 'processed' OR processed_at IS NOT NULL)
);
CREATE INDEX workflow_outbox_pending_idx ON workflow_outbox(retry_at, created_at) WHERE status IN ('pending', 'failed');
CREATE INDEX workflow_outbox_dispatched_idx ON workflow_outbox(lease_until, created_at) WHERE status = 'dispatched';
CREATE INDEX workflow_outbox_game_created_idx ON workflow_outbox(game_id, created_at DESC);

CREATE TABLE account_deletion_jobs (
    id uuid PRIMARY KEY,
    external_account_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'complete', 'failed')),
    cleanup_stage text NOT NULL DEFAULT 'requested' CHECK (cleanup_stage IN ('requested', 'games', 'auth', 'complete')),
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE UNIQUE INDEX account_deletion_one_unfinished_idx ON account_deletion_jobs(external_account_id) WHERE status IN ('pending', 'running', 'failed');

-- Product identity and cross-table invariants that ordinary CHECK/FK constraints cannot express.
-- +goose StatementBegin
CREATE FUNCTION mythborn_guard_game_role() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.player_role IS DISTINCT FROM OLD.player_role THEN
        RAISE EXCEPTION 'game player_role is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER games_player_role_immutable BEFORE UPDATE OF player_role ON games
FOR EACH ROW EXECUTE FUNCTION mythborn_guard_game_role();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_agent_template_type() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE template_type text;
BEGIN
    IF NEW.source_template_id IS NOT NULL THEN
        SELECT agent_type INTO template_type FROM agent_templates WHERE id = NEW.source_template_id;
        IF template_type IS DISTINCT FROM NEW.agent_type THEN
            RAISE EXCEPTION 'agent source template type must match agent type' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER agents_source_template_type BEFORE INSERT OR UPDATE OF source_template_id, agent_type ON agents
FOR EACH ROW EXECUTE FUNCTION mythborn_validate_agent_template_type();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_observation_role() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE role_name text;
BEGIN
    SELECT player_role INTO role_name FROM games WHERE id = NEW.game_id;
    IF role_name = 'observer' AND NEW.player_statement IS NOT NULL THEN
        RAISE EXCEPTION 'observer observations cannot include a player statement' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER observations_player_role BEFORE INSERT OR UPDATE OF game_id, player_statement ON observations
FOR EACH ROW EXECUTE FUNCTION mythborn_validate_observation_role();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_chronicle() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE round_kind text; round_status text; author_type text;
BEGIN
    SELECT kind, status INTO round_kind, round_status FROM rounds WHERE (game_id, id) = (NEW.game_id, NEW.round_id);
    SELECT agent_type INTO author_type FROM agents WHERE (game_id, id) = (NEW.game_id, NEW.historian_agent_id);
    IF author_type IS DISTINCT FROM 'historian' THEN
        RAISE EXCEPTION 'chronicle author must be this game historian' USING ERRCODE = '23514';
    END IF;
    IF (round_kind = 'closing') <> (NEW.outcome IS NULL) THEN
        RAISE EXCEPTION 'only closing chronicles may omit outcome' USING ERRCODE = '23514';
    END IF;
    IF NEW.suggestion IS NOT NULL AND round_kind <> 'discovery' THEN
        RAISE EXCEPTION 'only discovery chronicles may have a suggestion' USING ERRCODE = '23514';
    END IF;
    IF round_status <> 'complete' AND pg_trigger_depth() = 0 THEN
        RAISE EXCEPTION 'chronicle round must be complete at commit' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER chronicles_validate_round AFTER INSERT OR UPDATE ON chronicles
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_chronicle();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_round_chronicle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'complete' AND NOT EXISTS (SELECT 1 FROM chronicles WHERE game_id = NEW.game_id AND round_id = NEW.id) THEN
        RAISE EXCEPTION 'completed round requires a chronicle' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER rounds_require_chronicle AFTER INSERT OR UPDATE ON rounds
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_round_chronicle();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_round_source_kind() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE round_kind text;
BEGIN
    IF NEW.source_round_id IS NOT NULL THEN
        SELECT kind INTO round_kind FROM rounds WHERE (game_id, id) = (NEW.game_id, NEW.source_round_id);
        IF (NEW.cause = 'discovery' AND round_kind <> 'discovery') OR (NEW.cause = 'council' AND round_kind <> 'council') THEN
            RAISE EXCEPTION 'belief revision source round kind does not match cause' USING ERRCODE = '23514';
        END IF;
    END IF;
    IF NEW.source_conversation_summary_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM conversation_summaries s JOIN games g ON g.id = s.game_id
        WHERE (s.game_id, s.id) = (NEW.game_id, NEW.source_conversation_summary_id) AND g.player_role = 'messenger'
    ) THEN
        RAISE EXCEPTION 'messenger belief revision requires a Messenger game summary' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER belief_revisions_validate_source BEFORE INSERT OR UPDATE ON belief_revisions
FOR EACH ROW EXECUTE FUNCTION mythborn_validate_round_source_kind();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_tradition_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE round_kind text; agent_count integer;
BEGIN
    SELECT kind INTO round_kind FROM rounds WHERE (game_id, id) = (NEW.game_id, NEW.source_round_id);
    IF round_kind NOT IN ('discovery', 'council') THEN
        RAISE EXCEPTION 'tradition revision must come from discovery or council' USING ERRCODE = '23514';
    END IF;
    SELECT count(*) INTO agent_count FROM agents WHERE game_id = NEW.game_id;
    IF cardinality(NEW.previous_supporter_ids) > agent_count OR cardinality(NEW.new_supporter_ids) > agent_count OR
       cardinality(NEW.previous_supporter_ids) <> (SELECT count(DISTINCT x) FROM unnest(NEW.previous_supporter_ids) x) OR
       cardinality(NEW.new_supporter_ids) <> (SELECT count(DISTINCT x) FROM unnest(NEW.new_supporter_ids) x) THEN
        RAISE EXCEPTION 'tradition revision supporter snapshots must be distinct and valid' USING ERRCODE = '23514';
    END IF;
    IF EXISTS (SELECT 1 FROM unnest(NEW.previous_supporter_ids) x WHERE NOT EXISTS (SELECT 1 FROM agents a WHERE a.game_id = NEW.game_id AND a.id = x)) OR
       EXISTS (SELECT 1 FROM unnest(NEW.new_supporter_ids) x WHERE NOT EXISTS (SELECT 1 FROM agents a WHERE a.game_id = NEW.game_id AND a.id = x)) THEN
        RAISE EXCEPTION 'tradition revision supporter must belong to game' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER tradition_revisions_validate BEFORE INSERT OR UPDATE ON tradition_revisions
FOR EACH ROW EXECUTE FUNCTION mythborn_validate_tradition_revision();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_conversation_summary() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE game_role text; round_kind text; round_status text;
BEGIN
    SELECT player_role INTO game_role FROM games WHERE id = NEW.game_id;
    SELECT kind, status INTO round_kind, round_status FROM rounds WHERE (game_id, id) = (NEW.game_id, NEW.round_id);
    IF game_role <> 'messenger' OR round_kind <> 'discovery' OR round_status <> 'complete' THEN
        RAISE EXCEPTION 'conversation summary requires a completed discovery in a Messenger game' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER conversation_summaries_validate AFTER INSERT OR UPDATE ON conversation_summaries
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_conversation_summary();

-- +goose StatementBegin
CREATE FUNCTION mythborn_validate_game_agent_set() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE gid uuid; agent_count integer;
BEGIN
    IF TG_TABLE_NAME = 'games' THEN
        gid := NEW.id;
    ELSE
        gid := COALESCE(NEW.game_id, OLD.game_id);
    END IF;
    IF EXISTS (SELECT 1 FROM games WHERE id = gid AND status <> 'deleting') THEN
        SELECT count(*) INTO agent_count FROM agents WHERE game_id = gid;
        IF agent_count <> 4 OR (SELECT count(DISTINCT agent_type) FROM agents WHERE game_id = gid) <> 4 THEN
            RAISE EXCEPTION 'a playable game must have exactly four founding agents' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER agents_exactly_four_after_change AFTER INSERT OR UPDATE OR DELETE ON agents
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_game_agent_set();
CREATE CONSTRAINT TRIGGER games_require_four_agents AFTER INSERT ON games
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mythborn_validate_game_agent_set();

-- +goose Down

DROP TABLE IF EXISTS account_deletion_jobs CASCADE;
DROP TABLE IF EXISTS workflow_outbox CASCADE;
DROP TABLE IF EXISTS search_documents CASCADE;
DROP TABLE IF EXISTS tradition_revisions CASCADE;
DROP TABLE IF EXISTS tradition_supporters CASCADE;
DROP TABLE IF EXISTS belief_revisions CASCADE;
DROP TABLE IF EXISTS conversation_summaries CASCADE;
DROP TABLE IF EXISTS traditions CASCADE;
DROP TABLE IF EXISTS beliefs CASCADE;
DROP TABLE IF EXISTS chronicles CASCADE;
DROP TABLE IF EXISTS rounds CASCADE;
DROP TABLE IF EXISTS observations CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS games CASCADE;
DROP TABLE IF EXISTS initial_belief_templates CASCADE;
DROP TABLE IF EXISTS agent_templates CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_game_agent_set() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_conversation_summary() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_tradition_revision() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_round_source_kind() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_round_chronicle() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_chronicle() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_observation_role() CASCADE;
DROP FUNCTION IF EXISTS mythborn_validate_agent_template_type() CASCADE;
DROP FUNCTION IF EXISTS mythborn_guard_game_role() CASCADE;

