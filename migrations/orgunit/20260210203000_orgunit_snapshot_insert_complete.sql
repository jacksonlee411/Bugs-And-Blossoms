-- +goose Up
-- +goose StatementBegin
-- 080C follow-up: enforce INSERT-complete audit snapshots and strict rescind outcome semantics.

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
    WHEN p_event_type = 'CREATE'
      THEN p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')
      THEN p_before_snapshot IS NOT NULL AND p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN p_before_snapshot IS NOT NULL
           AND (
             (p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)
             OR (p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)
           )

    ELSE true
  END;
$$;

UPDATE orgunit.org_events
SET rescind_outcome = CASE WHEN after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END
WHERE event_type IN ('RESCIND_EVENT','RESCIND_ORG')
  AND rescind_outcome IS NULL;

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
      AND rescind_outcome IN ('PRESENT','ABSENT')
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

CREATE OR REPLACE FUNCTION orgunit.org_events_effective_for_replay(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_id text,
  p_pending_initiator_uuid uuid,
  p_pending_tx_time timestamptz,
  p_pending_transaction_time timestamptz,
  p_pending_created_at timestamptz
)
RETURNS TABLE (
  id bigint,
  event_uuid uuid,
  tenant_uuid uuid,
  org_id int,
  event_type text,
  effective_date date,
  payload jsonb,
  request_id text,
  initiator_uuid uuid,
  transaction_time timestamptz,
  created_at timestamptz
)
LANGUAGE sql
STABLE
AS $$
  WITH source_events AS (
    SELECT
      e.id,
      e.event_uuid,
      e.tenant_uuid,
      e.org_id,
      e.event_type,
      e.effective_date,
      COALESCE(e.payload, '{}'::jsonb) AS payload,
      e.request_id,
      e.initiator_uuid,
      e.tx_time,
      e.transaction_time,
      e.created_at
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id

    UNION ALL

    SELECT
      p_pending_event_id,
      p_pending_event_uuid,
      p_tenant_uuid,
      p_org_id,
      p_pending_event_type,
      p_pending_effective_date,
      COALESCE(p_pending_payload, '{}'::jsonb),
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    WHERE p_pending_event_id IS NOT NULL
  ),
  correction_events AS (
    SELECT
      se.*,
      (se.payload->>'target_event_uuid')::uuid AS target_event_uuid
    FROM source_events se
    WHERE se.event_type IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
      AND se.payload ? 'target_event_uuid'
  ),
  latest_corrections AS (
    SELECT DISTINCT ON (tenant_uuid, target_event_uuid)
      tenant_uuid,
      target_event_uuid,
      event_type AS correction_type,
      payload AS correction_payload,
      tx_time,
      id
    FROM correction_events
    ORDER BY tenant_uuid, target_event_uuid, tx_time DESC, id DESC
  )
  SELECT
    se.id,
    se.event_uuid,
    se.tenant_uuid,
    se.org_id,
    CASE
      WHEN lc.correction_type = 'CORRECT_STATUS'
        AND COALESCE(lc.correction_payload->>'target_status', '') = 'active'
        THEN 'ENABLE'
      WHEN lc.correction_type = 'CORRECT_STATUS'
        AND COALESCE(lc.correction_payload->>'target_status', '') = 'disabled'
        THEN 'DISABLE'
      ELSE se.event_type
    END AS event_type,
    CASE
      WHEN lc.correction_type = 'CORRECT_EVENT'
        AND lc.correction_payload ? 'effective_date'
        THEN NULLIF(btrim(lc.correction_payload->>'effective_date'), '')::date
      ELSE se.effective_date
    END AS effective_date,
    CASE
      WHEN lc.correction_type = 'CORRECT_EVENT'
        THEN se.payload || (lc.correction_payload - 'effective_date' - 'target_event_uuid' - 'op')
      ELSE se.payload
    END AS payload,
    se.request_id,
    se.initiator_uuid,
    se.transaction_time,
    se.created_at
  FROM source_events se
  LEFT JOIN latest_corrections lc
    ON lc.tenant_uuid = se.tenant_uuid
   AND lc.target_event_uuid = se.event_uuid
  WHERE se.event_type IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
    AND COALESCE(lc.correction_type, '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG')
  ORDER BY effective_date, id;
$$;

CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_id text,
  p_pending_initiator_uuid uuid,
  p_pending_tx_time timestamptz,
  p_pending_transaction_time timestamptz,
  p_pending_created_at timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_event record;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
  v_root_org_id int;
  v_has_create boolean;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;

  IF p_pending_event_id IS NOT NULL THEN
    IF p_pending_event_uuid IS NULL
      OR p_pending_event_type IS NULL
      OR p_pending_effective_date IS NULL
      OR p_pending_request_id IS NULL
      OR p_pending_initiator_uuid IS NULL
      OR p_pending_tx_time IS NULL
      OR p_pending_transaction_time IS NULL
      OR p_pending_created_at IS NULL
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = 'pending event metadata is incomplete';
    END IF;
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  SELECT EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective_for_replay(
      p_tenant_uuid,
      p_org_id,
      p_pending_event_id,
      p_pending_event_uuid,
      p_pending_event_type,
      p_pending_effective_date,
      p_pending_payload,
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    ) e
    WHERE e.event_type = 'CREATE'
  ) INTO v_has_create;

  IF v_has_create AND v_root_org_id = p_org_id THEN
    DELETE FROM orgunit.org_trees
    WHERE tenant_uuid = p_tenant_uuid;
  ELSIF v_root_org_id = p_org_id AND NOT v_has_create THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('root org missing create event: org_id=%s', p_org_id);
  END IF;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events_effective_for_replay(
      p_tenant_uuid,
      p_org_id,
      p_pending_event_id,
      p_pending_event_uuid,
      p_pending_event_type,
      p_pending_effective_date,
      p_pending_payload,
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    )
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
      v_org_code := NULLIF(v_payload->>'org_code', '');
      v_is_business_unit := NULL;
      IF v_payload ? 'is_business_unit' THEN
        BEGIN
          v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_INVALID_ARGUMENT',
              DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
        END;
      END IF;
      PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_event.org_id, v_org_code, v_parent_id, v_event.effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'ENABLE' THEN
      PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'SET_BUSINESS_UNIT' THEN
      IF NOT (v_payload ? 'is_business_unit') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = 'is_business_unit is required';
      END IF;
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
      PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_is_business_unit, v_event.id);
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_event.event_type);
    END IF;
  END LOOP;

  SELECT v.node_path INTO v_root_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_root_path IS NULL THEN
    RETURN;
  END IF;

  SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.node_path <@ v_root_path;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org(
  p_tenant_uuid uuid,
  p_org_id int
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_org_id int;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF p_event_type = 'SET_BUSINESS_UNIT' THEN
    IF NOT (v_payload ? 'is_business_unit') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = 'is_business_unit is required';
    END IF;
    BEGIN
      PERFORM (v_payload->>'is_business_unit')::boolean;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
    END;
  END IF;

  IF p_event_type = 'CREATE' AND p_org_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF FOUND THEN
      IF v_existing.tenant_uuid <> p_tenant_uuid
        OR v_existing.event_type <> p_event_type
        OR v_existing.effective_date <> p_effective_date
        OR v_existing.payload <> v_payload
        OR v_existing.request_id <> p_request_id
        OR v_existing.initiator_uuid <> p_initiator_uuid
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
          DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
      END IF;

      RETURN v_existing.id;
    END IF;

    v_org_id := orgunit.allocate_org_id(p_tenant_uuid);
  ELSE
    IF p_org_id IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
    END IF;
    v_org_id := p_org_id;
  END IF;

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_uuid <> p_event_uuid
      OR v_existing_request.org_id <> v_org_id
      OR v_existing_request.event_type <> p_event_type
      OR v_existing_request.effective_date <> p_effective_date
      OR v_existing_request.payload <> v_payload
      OR v_existing_request.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.id;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);

  SELECT * INTO v_existing
  FROM orgunit.org_events
  WHERE event_uuid = p_event_uuid;

  IF FOUND THEN
    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  IF p_event_type = 'CREATE' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
    v_name := NULLIF(btrim(v_payload->>'name'), '');
    v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
    v_org_code := NULLIF(v_payload->>'org_code', '');
    v_is_business_unit := NULL;
    IF v_payload ? 'is_business_unit' THEN
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
    END IF;
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_org_id, v_org_code, v_parent_id, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'MOVE' THEN
    v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_org_id, v_new_parent_id, p_effective_date, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.node_path <@ v_root_path;
  ELSIF p_event_type = 'RENAME' THEN
    v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_org_id, p_effective_date, v_new_name, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'DISABLE' THEN
    PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'ENABLE' THEN
    PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  END IF;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);
  PERFORM orgunit.assert_org_event_snapshots(p_event_type, v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    before_snapshot,
    after_snapshot
  )
  VALUES (
    v_event_db_id,
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot
  );

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_reason text;
  v_event_uuid uuid;
  v_payload jsonb;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_rescind_outcome text;
  v_pending_time timestamptz;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  v_reason := btrim(COALESCE(p_reason, ''));
  IF v_reason = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'reason is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'RESCIND_EVENT'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.effective_date <> p_target_effective_date
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RETURN v_existing_rescind.event_uuid;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'RESCIND_EVENT',
    'reason', v_reason,
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  );

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'RESCIND_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_rescind_outcome := CASE WHEN v_after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END;
  PERFORM orgunit.assert_org_event_snapshots('RESCIND_EVENT', v_before_snapshot, v_after_snapshot, v_rescind_outcome);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    reason,
    before_snapshot,
    after_snapshot,
    rescind_outcome,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'RESCIND_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_reason,
    v_before_snapshot,
    v_after_snapshot,
    v_rescind_outcome,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_reason text;
  v_root_org_id int;
  v_node_path ltree;
  v_event_count int;
  v_existing_batch_count int;
  v_need_apply boolean;
  v_request_id_seq text;
  v_payload jsonb;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_rescind_event_uuid uuid;
  v_rescind_event_id bigint;
  v_rescind_outcome text;
  v_pending_time timestamptz;
  rec record;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  v_reason := btrim(COALESCE(p_reason, ''));
  IF v_reason = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'reason is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_id=%s', p_request_id);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid
  LIMIT 1;

  IF v_root_org_id = p_org_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_DELETE_FORBIDDEN',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT v.node_path INTO v_node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_node_path IS NOT NULL AND EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions c
    WHERE c.tenant_uuid = p_tenant_uuid
      AND c.node_path <@ v_node_path
      AND c.org_id <> p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_CHILDREN_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions b
    WHERE b.tenant_uuid = p_tenant_uuid
      AND b.org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_DEPENDENCIES_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_event_count
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id;

  IF v_event_count = 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_existing_batch_count
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.request_id LIKE p_request_id || '#%';

  IF v_existing_batch_count > 0 AND v_existing_batch_count <> v_event_count THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_id=%s', p_request_id);
  END IF;

  v_need_apply := false;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    SELECT * INTO v_existing_request
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.request_id = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      IF v_existing_request.event_type <> 'RESCIND_ORG'
        OR v_existing_request.org_id <> p_org_id
        OR COALESCE(v_existing_request.payload->>'target_event_uuid', '') <> rec.event_uuid::text
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
          DETAIL = format('request_id=%s', p_request_id);
      END IF;
      CONTINUE;
    END IF;

    v_need_apply := true;
  END LOOP;

  IF NOT v_need_apply THEN
    RETURN v_event_count;
  END IF;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    SELECT * INTO v_existing_request
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.request_id = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      CONTINUE;
    END IF;

    SELECT * INTO v_existing_rescind
    FROM orgunit.org_events r
    WHERE r.tenant_uuid = p_tenant_uuid
      AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      AND r.payload->>'target_event_uuid' = rec.event_uuid::text
    LIMIT 1;

    IF FOUND THEN
      CONTINUE;
    END IF;

    v_payload := jsonb_build_object(
      'op', 'RESCIND_ORG',
      'reason', v_reason,
      'batch_request_id', p_request_id,
      'target_event_uuid', rec.event_uuid,
      'target_effective_date', rec.target_effective_date
    );

    v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, rec.target_effective_date);
    v_rescind_event_uuid := gen_random_uuid();
    v_pending_time := now();
    SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_rescind_event_id;

    PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
      p_tenant_uuid,
      p_org_id,
      v_rescind_event_id,
      v_rescind_event_uuid,
      'RESCIND_ORG',
      rec.target_effective_date,
      v_payload,
      v_request_id_seq,
      p_initiator_uuid,
      v_pending_time,
      v_pending_time,
      v_pending_time
    );

    v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, rec.target_effective_date);
    v_rescind_outcome := CASE WHEN v_after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END;
    PERFORM orgunit.assert_org_event_snapshots('RESCIND_ORG', v_before_snapshot, v_after_snapshot, v_rescind_outcome);

    INSERT INTO orgunit.org_events (
      id,
      tenant_uuid,
      org_id,
      event_uuid,
      event_type,
      effective_date,
      payload,
      request_id,
      initiator_uuid,
      reason,
      before_snapshot,
      after_snapshot,
      rescind_outcome,
      tx_time,
      transaction_time,
      created_at
    )
    VALUES (
      v_rescind_event_id,
      p_tenant_uuid,
      p_org_id,
      v_rescind_event_uuid,
      'RESCIND_ORG',
      rec.target_effective_date,
      v_payload,
      v_request_id_seq,
      p_initiator_uuid,
      v_reason,
      v_before_snapshot,
      v_after_snapshot,
      v_rescind_outcome,
      v_pending_time,
      v_pending_time,
      v_pending_time
    );
  END LOOP;

  RETURN v_event_count;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_patch jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_effective_payload jsonb;
  v_target_effective date;
  v_prev_effective date;
  v_new_effective date;
  v_next_effective date;
  v_parent_id int;
  v_target_path ltree;
  v_descendant_min_create date;
  v_event_uuid uuid;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_pending_time timestamptz;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_patch IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_payload := p_patch;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch must be an object';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
  END IF;

  SELECT ee.effective_date, ee.payload
  INTO v_target_effective, v_effective_payload
  FROM orgunit.org_events_effective ee
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.event_uuid = v_target.event_uuid
  LIMIT 1;

  v_payload := COALESCE(v_effective_payload, '{}'::jsonb) || v_payload;
  v_new_effective := v_target_effective;

  IF v_payload ? 'effective_date' THEN
    BEGIN
      v_new_effective := NULLIF(btrim(v_payload->>'effective_date'), '')::date;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'EFFECTIVE_DATE_INVALID',
          DETAIL = format('effective_date=%s', v_payload->>'effective_date');
    END;
    IF v_new_effective IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EFFECTIVE_DATE_INVALID',
        DETAIL = 'effective_date is required';
    END IF;
    -- keep effective_date in payload for audit and correction view
  END IF;

  SELECT MAX(e.effective_date) INTO v_prev_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date < v_target_effective;

  SELECT MIN(e.effective_date) INTO v_next_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date > v_target_effective;

  IF v_prev_effective IS NOT NULL AND v_new_effective <= v_prev_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('prev=%s new=%s', v_prev_effective, v_new_effective);
  END IF;
  IF v_next_effective IS NOT NULL AND v_new_effective >= v_next_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('next=%s new=%s', v_next_effective, v_new_effective);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND e.effective_date = v_new_effective
      AND e.event_uuid <> v_target.event_uuid
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EVENT_DATE_CONFLICT',
      DETAIL = format('org_id=%s effective_date=%s', p_org_id, v_new_effective);
  END IF;

  IF v_payload ? 'parent_id' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
    IF v_parent_id IS NOT NULL THEN
      IF v_parent_id = p_org_id THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_CYCLE_DETECTED',
          DETAIL = format('org_id=%s parent_id=%s', p_org_id, v_parent_id);
      END IF;
      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.org_unit_versions v
        WHERE v.tenant_uuid = p_tenant_uuid
          AND v.org_id = v_parent_id
          AND v.status = 'active'
          AND v.validity @> v_new_effective
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
          DETAIL = format('parent_id=%s as_of=%s', v_parent_id, v_new_effective);
      END IF;
    END IF;
  END IF;

  -- Guard high-risk create reordering that would force replay to fail after full-table churn.
  IF v_target.event_type = 'CREATE' AND v_new_effective > v_target_effective THEN
    SELECT v.node_path INTO v_target_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> v_target_effective
    ORDER BY lower(v.validity) DESC
    LIMIT 1;

    IF v_target_path IS NOT NULL THEN
      SELECT MIN(e.effective_date) INTO v_descendant_min_create
      FROM orgunit.org_events_effective e
      WHERE e.tenant_uuid = p_tenant_uuid
        AND e.event_type = 'CREATE'
        AND e.org_id <> p_org_id
        AND EXISTS (
          SELECT 1
          FROM orgunit.org_unit_versions dv
          WHERE dv.tenant_uuid = p_tenant_uuid
            AND dv.org_id = e.org_id
            AND dv.node_path <@ v_target_path
          LIMIT 1
        );

      IF v_descendant_min_create IS NOT NULL AND v_descendant_min_create < v_new_effective THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_HIGH_RISK_REORDER_FORBIDDEN',
          DETAIL = format(
            'org_id=%s target_effective=%s new_effective=%s descendant_create=%s',
            p_org_id,
            v_target_effective,
            v_new_effective,
            v_descendant_min_create
          );
      END IF;
    END IF;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'CORRECT_EVENT',
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  ) || v_payload;

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'CORRECT_EVENT'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.payload <> v_payload
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'CORRECT_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  PERFORM orgunit.assert_org_event_snapshots('CORRECT_EVENT', v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    before_snapshot,
    after_snapshot,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'CORRECT_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_status_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_target_status text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_target_status text;
  v_target_effective date;
  v_effective_payload jsonb;
  v_event_uuid uuid;
  v_payload jsonb;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_pending_time timestamptz;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_target_status := lower(btrim(COALESCE(p_target_status, '')));
  IF v_target_status IN ('enabled', '有效') THEN
    v_target_status := 'active';
  ELSIF v_target_status IN ('inactive', '无效') THEN
    v_target_status := 'disabled';
  END IF;
  IF v_target_status NOT IN ('active', 'disabled') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_status invalid';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  IF v_target.event_type NOT IN ('ENABLE', 'DISABLE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET',
      DETAIL = format('event_type=%s', v_target.event_type);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
  END IF;

  SELECT ee.effective_date, ee.payload
  INTO v_target_effective, v_effective_payload
  FROM orgunit.org_events_effective ee
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF v_target_effective IS NULL THEN
    v_target_effective := v_target.effective_date;
    v_effective_payload := v_target.payload;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'CORRECT_STATUS',
    'target_status', v_target_status,
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  );

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'CORRECT_STATUS'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.payload <> v_payload
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'CORRECT_STATUS',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  PERFORM orgunit.assert_org_event_snapshots('CORRECT_STATUS', v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    before_snapshot,
    after_snapshot,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'CORRECT_STATUS',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;

ALTER FUNCTION orgunit.is_org_event_snapshot_presence_valid(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.org_events_effective_for_replay(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.org_events_effective_for_replay(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) SET search_path = pg_catalog, orgunit, public;

REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
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
    WHEN p_before_snapshot IS NULL AND p_after_snapshot IS NULL
      THEN true

    WHEN p_event_type = 'CREATE'
      THEN p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')
      THEN p_before_snapshot IS NOT NULL AND p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN p_before_snapshot IS NOT NULL
           AND (
             p_rescind_outcome IS NULL
             OR (p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)
             OR (p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)
           )

    ELSE true
  END;
$$;

ALTER FUNCTION orgunit.is_org_event_snapshot_presence_valid(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;

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
-- +goose StatementEnd
