-- +goose Up
-- +goose StatementBegin
-- 080D: enforce rescind payload/snapshot completeness and backfill historical sparse rescind rows.

CREATE OR REPLACE FUNCTION orgunit.is_orgunit_snapshot_complete(p_snapshot jsonb)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT p_snapshot IS NOT NULL
    AND jsonb_typeof(p_snapshot) = 'object'
    AND p_snapshot ?& ARRAY[
      'org_id',
      'name',
      'status',
      'parent_id',
      'node_path',
      'validity',
      'full_name_path',
      'is_business_unit'
    ];
$$;

CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_content_valid(
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
    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN orgunit.is_orgunit_snapshot_complete(p_before_snapshot)
           AND (
             p_rescind_outcome = 'ABSENT'
             OR (
               p_rescind_outcome = 'PRESENT'
               AND orgunit.is_orgunit_snapshot_complete(p_after_snapshot)
             )
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

  IF NOT orgunit.is_org_event_snapshot_content_valid(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    p_rescind_outcome
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format(
        'event_type=%s incomplete_snapshot_content=true rescind_outcome=%s',
        p_event_type,
        COALESCE(p_rescind_outcome, 'NULL')
      );
  END IF;
END;
$$;

ALTER FUNCTION orgunit.is_orgunit_snapshot_complete(jsonb)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.is_org_event_snapshot_content_valid(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SET search_path = pg_catalog, orgunit, public;

-- Backfill sparse rescind payload keys to canonical shape.
UPDATE orgunit.org_events e
SET payload = COALESCE(e.payload, '{}'::jsonb)
              || jsonb_build_object(
                   'op', COALESCE(NULLIF(btrim(e.payload->>'op'), ''), e.event_type),
                   'reason', COALESCE(NULLIF(btrim(e.payload->>'reason'), ''), NULLIF(btrim(e.reason), ''), '历史数据补齐'),
                   'target_effective_date', COALESCE(NULLIF(btrim(e.payload->>'target_effective_date'), ''), to_char(e.effective_date, 'YYYY-MM-DD'))
                 )
WHERE e.event_type IN ('RESCIND_EVENT', 'RESCIND_ORG');

-- Backfill sparse rescind snapshots from target event snapshots when available.
WITH target AS (
  SELECT
    r.id AS rescind_id,
    t.before_snapshot AS target_before_snapshot,
    t.after_snapshot AS target_after_snapshot
  FROM orgunit.org_events r
  LEFT JOIN orgunit.org_events t
    ON t.tenant_uuid = r.tenant_uuid
   AND t.org_id = r.org_id
   AND t.event_uuid::text = r.payload->>'target_event_uuid'
  WHERE r.event_type IN ('RESCIND_EVENT', 'RESCIND_ORG')
)
UPDATE orgunit.org_events r
SET
  before_snapshot = CASE
    WHEN r.before_snapshot IS NULL OR r.before_snapshot = '{}'::jsonb THEN
      CASE
        WHEN target.target_after_snapshot IS NOT NULL AND target.target_after_snapshot <> '{}'::jsonb THEN target.target_after_snapshot
        WHEN target.target_before_snapshot IS NOT NULL AND target.target_before_snapshot <> '{}'::jsonb THEN target.target_before_snapshot
        ELSE r.before_snapshot
      END
    ELSE r.before_snapshot
  END,
  after_snapshot = CASE
    WHEN r.rescind_outcome = 'ABSENT' THEN NULL
    WHEN r.after_snapshot IS NULL OR r.after_snapshot = '{}'::jsonb THEN
      CASE
        WHEN target.target_before_snapshot IS NOT NULL AND target.target_before_snapshot <> '{}'::jsonb THEN target.target_before_snapshot
        WHEN target.target_after_snapshot IS NOT NULL AND target.target_after_snapshot <> '{}'::jsonb THEN target.target_after_snapshot
        ELSE r.after_snapshot
      END
    ELSE r.after_snapshot
  END
FROM target
WHERE r.id = target.rescind_id;

-- Normalize rescind outcome to the final snapshot state.
UPDATE orgunit.org_events
SET rescind_outcome = CASE WHEN after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END
WHERE event_type IN ('RESCIND_EVENT', 'RESCIND_ORG');

ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_rescind_payload_required,
  DROP CONSTRAINT IF EXISTS org_events_snapshot_content_check;

ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_rescind_payload_required CHECK (
    event_type NOT IN ('RESCIND_EVENT','RESCIND_ORG')
    OR (
      COALESCE(NULLIF(btrim(payload->>'op'), ''), '') = event_type
      AND COALESCE(NULLIF(btrim(payload->>'reason'), ''), '') <> ''
      AND COALESCE(NULLIF(btrim(payload->>'target_event_uuid'), ''), '') <> ''
      AND COALESCE(NULLIF(btrim(payload->>'target_effective_date'), ''), '') = to_char(effective_date, 'YYYY-MM-DD')
    )
  ) NOT VALID,
  ADD CONSTRAINT org_events_snapshot_content_check CHECK (
    orgunit.is_org_event_snapshot_content_valid(
      event_type,
      before_snapshot,
      after_snapshot,
      rescind_outcome
    )
  ) NOT VALID;

ALTER TABLE orgunit.org_events VALIDATE CONSTRAINT org_events_rescind_payload_required;
ALTER TABLE orgunit.org_events VALIDATE CONSTRAINT org_events_snapshot_content_check;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_snapshot_content_check,
  DROP CONSTRAINT IF EXISTS org_events_rescind_payload_required;

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

ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_org_event_snapshots(text, jsonb, jsonb, text)
  SET search_path = pg_catalog, orgunit, public;

DROP FUNCTION IF EXISTS orgunit.is_org_event_snapshot_content_valid(text, jsonb, jsonb, text);
DROP FUNCTION IF EXISTS orgunit.is_orgunit_snapshot_complete(jsonb);
-- +goose StatementEnd
