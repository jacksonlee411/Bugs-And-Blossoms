-- +goose Up
-- +goose StatementBegin
-- 080C (Phase DDL): add rescind_outcome + table-level presence predicate + deferrable FK.

CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_presence_valid(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT CASE
    -- Transitional allowance for two-step write paths (INSERT first, UPDATE snapshots later).
    WHEN p_before_snapshot IS NULL AND p_after_snapshot IS NULL
      THEN true

    WHEN p_event_type = 'CREATE'
      THEN p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')
      THEN p_before_snapshot IS NOT NULL AND p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN p_before_snapshot IS NOT NULL
           AND (
             -- Transitional allowance: NULL means existing two-step write path has not persisted outcome yet.
             p_rescind_outcome IS NULL
             OR (p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)
             OR (p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)
           )

    ELSE true
  END;
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_org_event_snapshots(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_before_snapshot IS NOT NULL AND jsonb_typeof(p_before_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s before_snapshot_type=%s', p_event_type, jsonb_typeof(p_before_snapshot));
  END IF;

  IF p_after_snapshot IS NOT NULL AND jsonb_typeof(p_after_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s after_snapshot_type=%s', p_event_type, jsonb_typeof(p_after_snapshot));
  END IF;

  IF NOT orgunit.is_org_event_snapshot_presence_valid(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    p_rescind_outcome
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_MISSING',
      DETAIL = format(
        'event_type=%s before=%s after=%s rescind_outcome=%s',
        p_event_type,
        p_before_snapshot IS NOT NULL,
        p_after_snapshot IS NOT NULL,
        COALESCE(p_rescind_outcome, 'NULL')
      );
  END IF;
END;
$$;

-- Backward-compatible wrapper for current 3-arg callers.
CREATE OR REPLACE FUNCTION orgunit.assert_org_event_snapshots(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_org_event_snapshots(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    NULL
  );
END;
$$;

ALTER TABLE orgunit.org_events
  ADD COLUMN IF NOT EXISTS rescind_outcome text NULL;

ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_rescind_outcome_check,
  DROP CONSTRAINT IF EXISTS org_events_snapshot_presence_check;

ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_rescind_outcome_check CHECK (
    (
      event_type NOT IN ('RESCIND_EVENT','RESCIND_ORG')
      AND rescind_outcome IS NULL
    )
    OR (
      event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      AND (rescind_outcome IS NULL OR rescind_outcome IN ('PRESENT','ABSENT'))
    )
  ) NOT VALID,
  ADD CONSTRAINT org_events_snapshot_presence_check CHECK (
    orgunit.is_org_event_snapshot_presence_valid(
      event_type,
      before_snapshot,
      after_snapshot,
      rescind_outcome
    )
  ) NOT VALID;

ALTER TABLE orgunit.org_unit_versions
  DROP CONSTRAINT IF EXISTS org_unit_versions_last_event_id_fkey;

ALTER TABLE orgunit.org_unit_versions
  ADD CONSTRAINT org_unit_versions_last_event_id_fkey
  FOREIGN KEY (last_event_id)
  REFERENCES orgunit.org_events(id)
  DEFERRABLE INITIALLY DEFERRED;

ALTER FUNCTION orgunit.is_org_event_snapshot_presence_valid(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_snapshot_presence_check,
  DROP CONSTRAINT IF EXISTS org_events_rescind_outcome_check;

ALTER TABLE orgunit.org_unit_versions
  DROP CONSTRAINT IF EXISTS org_unit_versions_last_event_id_fkey;

ALTER TABLE orgunit.org_unit_versions
  ADD CONSTRAINT org_unit_versions_last_event_id_fkey
  FOREIGN KEY (last_event_id)
  REFERENCES orgunit.org_events(id);

DROP FUNCTION IF EXISTS orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text);
DROP FUNCTION IF EXISTS orgunit.is_org_event_snapshot_presence_valid(text, jsonb, jsonb, text);

CREATE OR REPLACE FUNCTION orgunit.assert_org_event_snapshots(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_before_snapshot IS NOT NULL AND jsonb_typeof(p_before_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s before_snapshot_type=%s', p_event_type, jsonb_typeof(p_before_snapshot));
  END IF;

  IF p_after_snapshot IS NOT NULL AND jsonb_typeof(p_after_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s after_snapshot_type=%s', p_event_type, jsonb_typeof(p_after_snapshot));
  END IF;

  IF p_event_type = 'CREATE' THEN
    IF p_after_snapshot IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_AUDIT_SNAPSHOT_MISSING',
        DETAIL = format('event_type=%s before=%s after=%s', p_event_type, p_before_snapshot IS NOT NULL, p_after_snapshot IS NOT NULL);
    END IF;
    RETURN;
  END IF;

  IF p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS') THEN
    IF p_before_snapshot IS NULL OR p_after_snapshot IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_AUDIT_SNAPSHOT_MISSING',
        DETAIL = format('event_type=%s before=%s after=%s', p_event_type, p_before_snapshot IS NOT NULL, p_after_snapshot IS NOT NULL);
    END IF;
    RETURN;
  END IF;

  IF p_event_type IN ('RESCIND_EVENT','RESCIND_ORG') THEN
    IF p_before_snapshot IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_AUDIT_SNAPSHOT_MISSING',
        DETAIL = format('event_type=%s before=%s after=%s', p_event_type, p_before_snapshot IS NOT NULL, p_after_snapshot IS NOT NULL);
    END IF;
  END IF;
END;
$$;

ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb)
  SET search_path = pg_catalog, orgunit, public;

ALTER TABLE orgunit.org_events
  DROP COLUMN IF EXISTS rescind_outcome;
-- +goose StatementEnd
