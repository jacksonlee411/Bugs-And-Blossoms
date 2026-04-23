-- +goose Up
CREATE TABLE IF NOT EXISTS iam.cubebox_conversations (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  conversation_id text NOT NULL,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  title text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  archived boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz NULL,
  PRIMARY KEY (tenant_uuid, conversation_id),
  CONSTRAINT cubebox_conversations_id_nonempty_check CHECK (btrim(conversation_id) <> ''),
  CONSTRAINT cubebox_conversations_title_nonempty_check CHECK (btrim(title) <> ''),
  CONSTRAINT cubebox_conversations_status_check CHECK (status IN ('active', 'archived')),
  CONSTRAINT cubebox_conversations_archive_consistent_check CHECK (
    (archived = true AND status = 'archived')
    OR (archived = false AND status = 'active' AND archived_at IS NULL)
  )
);

CREATE INDEX IF NOT EXISTS cubebox_conversations_tenant_principal_updated_idx
  ON iam.cubebox_conversations (tenant_uuid, principal_id, updated_at DESC, conversation_id DESC);

CREATE INDEX IF NOT EXISTS cubebox_conversations_tenant_updated_idx
  ON iam.cubebox_conversations (tenant_uuid, updated_at DESC, conversation_id DESC);

CREATE TABLE IF NOT EXISTS iam.cubebox_conversation_events (
  tenant_uuid uuid NOT NULL,
  conversation_id text NOT NULL,
  event_id text NOT NULL,
  sequence integer NOT NULL,
  turn_id text NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, conversation_id, event_id),
  CONSTRAINT cubebox_conversation_events_conversation_fk FOREIGN KEY (tenant_uuid, conversation_id)
    REFERENCES iam.cubebox_conversations(tenant_uuid, conversation_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_conversation_events_sequence_positive_check CHECK (sequence > 0),
  CONSTRAINT cubebox_conversation_events_type_nonempty_check CHECK (btrim(event_type) <> ''),
  CONSTRAINT cubebox_conversation_events_payload_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT cubebox_conversation_events_turn_id_nonempty_or_null_check CHECK (
    turn_id IS NULL OR btrim(turn_id) <> ''
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS cubebox_conversation_events_sequence_unique
  ON iam.cubebox_conversation_events (tenant_uuid, conversation_id, sequence);

CREATE INDEX IF NOT EXISTS cubebox_conversation_events_lookup_idx
  ON iam.cubebox_conversation_events (tenant_uuid, conversation_id, sequence ASC);

-- +goose Down
DROP INDEX IF EXISTS iam.cubebox_conversation_events_lookup_idx;
DROP INDEX IF EXISTS iam.cubebox_conversation_events_sequence_unique;
DROP TABLE IF EXISTS iam.cubebox_conversation_events;
DROP INDEX IF EXISTS iam.cubebox_conversations_tenant_updated_idx;
DROP INDEX IF EXISTS iam.cubebox_conversations_tenant_principal_updated_idx;
DROP TABLE IF EXISTS iam.cubebox_conversations;
