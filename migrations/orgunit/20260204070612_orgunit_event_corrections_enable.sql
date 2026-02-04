-- +goose Up
-- modify "org_events" table
ALTER TABLE "orgunit"."org_events" DROP CONSTRAINT "org_events_event_type_check", ADD CONSTRAINT "org_events_event_type_check" CHECK (event_type = ANY (ARRAY['CREATE'::text, 'MOVE'::text, 'RENAME'::text, 'DISABLE'::text, 'ENABLE'::text, 'SET_BUSINESS_UNIT'::text]));
-- create "org_event_corrections_current" table
CREATE TABLE "orgunit"."org_event_corrections_current" (
  "event_uuid" uuid NOT NULL,
  "tenant_uuid" uuid NOT NULL,
  "org_id" integer NOT NULL,
  "target_effective_date" date NOT NULL,
  "corrected_effective_date" date NOT NULL,
  "original_event" jsonb NOT NULL,
  "replacement_payload" jsonb NOT NULL,
  "initiator_uuid" uuid NOT NULL,
  "request_id" text NOT NULL,
  "request_hash" text NOT NULL,
  "corrected_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("event_uuid"),
  CONSTRAINT "org_event_corrections_current_target_unique" UNIQUE ("tenant_uuid", "org_id", "target_effective_date"),
  CONSTRAINT "org_event_corrections_current_org_id_check" CHECK ((org_id >= 10000000) AND (org_id <= 99999999)),
  CONSTRAINT "org_event_corrections_current_original_event_obj_check" CHECK (jsonb_typeof(original_event) = 'object'::text),
  CONSTRAINT "org_event_corrections_current_replacement_payload_obj_check" CHECK (jsonb_typeof(replacement_payload) = 'object'::text)
);
-- create index "org_event_corrections_current_tenant_org_corrected_idx" to table: "org_event_corrections_current"
CREATE INDEX "org_event_corrections_current_tenant_org_corrected_idx" ON "orgunit"."org_event_corrections_current" ("tenant_uuid", "org_id", "corrected_effective_date");
-- create index "org_event_corrections_current_tenant_org_target_idx" to table: "org_event_corrections_current"
CREATE INDEX "org_event_corrections_current_tenant_org_target_idx" ON "orgunit"."org_event_corrections_current" ("tenant_uuid", "org_id", "target_effective_date");
-- create "org_event_corrections_history" table
CREATE TABLE "orgunit"."org_event_corrections_history" (
  "correction_uuid" uuid NOT NULL,
  "event_uuid" uuid NOT NULL,
  "tenant_uuid" uuid NOT NULL,
  "org_id" integer NOT NULL,
  "target_effective_date" date NOT NULL,
  "corrected_effective_date" date NOT NULL,
  "original_event" jsonb NOT NULL,
  "replacement_payload" jsonb NOT NULL,
  "initiator_uuid" uuid NOT NULL,
  "request_id" text NOT NULL,
  "request_hash" text NOT NULL,
  "corrected_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("correction_uuid"),
  CONSTRAINT "org_event_corrections_history_request_unique" UNIQUE ("tenant_uuid", "request_id"),
  CONSTRAINT "org_event_corrections_history_org_id_check" CHECK ((org_id >= 10000000) AND (org_id <= 99999999)),
  CONSTRAINT "org_event_corrections_history_original_event_obj_check" CHECK (jsonb_typeof(original_event) = 'object'::text),
  CONSTRAINT "org_event_corrections_history_replacement_payload_obj_check" CHECK (jsonb_typeof(replacement_payload) = 'object'::text)
);

-- +goose StatementBegin
CREATE OR REPLACE VIEW orgunit.org_events_effective AS
SELECT
  e.id,
  e.event_uuid,
  e.tenant_uuid,
  e.org_id,
  e.event_type,
  COALESCE(c.corrected_effective_date, e.effective_date) AS effective_date,
  COALESCE(c.replacement_payload, e.payload) AS payload,
  e.request_code,
  e.initiator_uuid,
  e.transaction_time,
  e.created_at
FROM orgunit.org_events e
LEFT JOIN orgunit.org_event_corrections_current c
  ON c.event_uuid = e.event_uuid
 AND c.tenant_uuid = e.tenant_uuid
 AND c.org_id = e.org_id;

CREATE OR REPLACE FUNCTION orgunit.apply_enable_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  UPDATE orgunit.org_unit_versions
  SET status = 'active', last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_rename_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_new_name text,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_new_name IS NULL OR btrim(p_new_name) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'new_name is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'RENAME'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET name = p_new_name, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_set_business_unit_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_is_business_unit boolean,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
  v_status text;
  v_root_org_id int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_is_business_unit IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'is_business_unit is required';
  END IF;

  SELECT v.status INTO v_status
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_status IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;
  IF v_status <> 'active' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  IF v_root_org_id IS NOT NULL AND v_root_org_id = p_org_id AND p_is_business_unit = false THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_BUSINESS_UNIT_REQUIRED',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'SET_BUSINESS_UNIT'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET is_business_unit = p_is_business_unit, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
  p_tenant_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_trees
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events_effective
    WHERE tenant_uuid = p_tenant_uuid
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
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
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

  IF EXISTS (
    WITH ordered AS (
      SELECT
        org_id,
        validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_GAP',
      DETAIL = 'org_unit_versions must be gapless';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_NOT_INFINITE',
      DETAIL = 'last version validity must be unbounded (infinity)';
  END IF;

  UPDATE orgunit.org_unit_versions v
  SET full_name_path = (
    SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
    FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
    JOIN orgunit.org_unit_versions a
      ON a.tenant_uuid = v.tenant_uuid
     AND a.org_id = t.uid
     AND a.validity @> lower(v.validity)
  )
  WHERE v.tenant_uuid = p_tenant_uuid
;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
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
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
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
        OR v_existing.request_code <> p_request_code
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

  INSERT INTO orgunit.org_events (
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_code,
    p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_code <> p_request_code
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

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

  RETURN v_event_db_id;
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
  v_existing orgunit.org_event_corrections_history%ROWTYPE;
  v_request_hash text;
  v_payload jsonb;
  v_new_effective date;
  v_next_effective date;
  v_correction_uuid uuid;
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

  v_request_hash := encode(
    digest(format('%s|%s|%s', p_org_id, p_target_effective_date, v_payload::text), 'sha256'),
    'hex'
  );

  SELECT * INTO v_existing
  FROM orgunit.org_event_corrections_history
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.request_hash <> v_request_hash THEN
      RAISE EXCEPTION USING
        MESSAGE = 'REQUEST_DUPLICATE',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing.correction_uuid;
  END IF;

  SELECT * INTO v_target
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  v_payload := COALESCE(v_target.payload, '{}'::jsonb) || v_payload;
  v_new_effective := v_target.effective_date;

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
    v_payload := v_payload - 'effective_date';
  END IF;

  SELECT MIN(e.effective_date) INTO v_next_effective
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date > v_target.effective_date;

  IF v_new_effective < v_target.effective_date THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('target=%s new=%s', v_target.effective_date, v_new_effective);
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

  v_correction_uuid := gen_random_uuid();

  INSERT INTO orgunit.org_event_corrections_history (
    correction_uuid,
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_correction_uuid,
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  );

  INSERT INTO orgunit.org_event_corrections_current (
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  )
  ON CONFLICT (event_uuid) DO UPDATE SET
    tenant_uuid = EXCLUDED.tenant_uuid,
    org_id = EXCLUDED.org_id,
    target_effective_date = EXCLUDED.target_effective_date,
    corrected_effective_date = EXCLUDED.corrected_effective_date,
    original_event = EXCLUDED.original_event,
    replacement_payload = EXCLUDED.replacement_payload,
    initiator_uuid = EXCLUDED.initiator_uuid,
    request_id = EXCLUDED.request_id,
    request_hash = EXCLUDED.request_hash,
    corrected_at = EXCLUDED.corrected_at;

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_correction_uuid;
END;
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.org_event_corrections_current,
  orgunit.org_event_corrections_history
TO orgunit_kernel;

GRANT SELECT ON TABLE orgunit.org_events_effective TO orgunit_kernel;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.apply_rename_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_new_name text,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_new_name IS NULL OR btrim(p_new_name) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'new_name is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'RENAME'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET name = p_new_name, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_set_business_unit_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_is_business_unit boolean,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
  v_status text;
  v_root_org_id int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_is_business_unit IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'is_business_unit is required';
  END IF;

  SELECT v.status INTO v_status
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_status IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;
  IF v_status <> 'active' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  IF v_root_org_id IS NOT NULL AND v_root_org_id = p_org_id AND p_is_business_unit = false THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_BUSINESS_UNIT_REQUIRED',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'SET_BUSINESS_UNIT'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET is_business_unit = p_is_business_unit, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
  p_tenant_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_trees
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events
    WHERE tenant_uuid = p_tenant_uuid
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
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
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

  IF EXISTS (
    WITH ordered AS (
      SELECT
        org_id,
        validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_GAP',
      DETAIL = 'org_unit_versions must be gapless';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_NOT_INFINITE',
      DETAIL = 'last version validity must be unbounded (infinity)';
  END IF;

  UPDATE orgunit.org_unit_versions v
  SET full_name_path = (
    SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
    FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
    JOIN orgunit.org_unit_versions a
      ON a.tenant_uuid = v.tenant_uuid
     AND a.org_id = t.uid
     AND a.validity @> lower(v.validity)
  )
  WHERE v.tenant_uuid = p_tenant_uuid
;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
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
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','MOVE','RENAME','DISABLE','SET_BUSINESS_UNIT') THEN
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
        OR v_existing.request_code <> p_request_code
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

  INSERT INTO orgunit.org_events (
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_code,
    p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_code <> p_request_code
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

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
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  END IF;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  RETURN v_event_db_id;
END;
$$;

DROP FUNCTION IF EXISTS orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS orgunit.apply_enable_logic(uuid, int, date, bigint);
DROP VIEW IF EXISTS orgunit.org_events_effective;
-- +goose StatementEnd
-- reverse: create "org_event_corrections_history" table
DROP TABLE "orgunit"."org_event_corrections_history";
-- reverse: create index "org_event_corrections_current_tenant_org_target_idx" to table: "org_event_corrections_current"
DROP INDEX "orgunit"."org_event_corrections_current_tenant_org_target_idx";
-- reverse: create index "org_event_corrections_current_tenant_org_corrected_idx" to table: "org_event_corrections_current"
DROP INDEX "orgunit"."org_event_corrections_current_tenant_org_corrected_idx";
-- reverse: create "org_event_corrections_current" table
DROP TABLE "orgunit"."org_event_corrections_current";
-- reverse: modify "org_events" table
ALTER TABLE "orgunit"."org_events" DROP CONSTRAINT "org_events_event_type_check", ADD CONSTRAINT "org_events_event_type_check" CHECK (event_type = ANY (ARRAY['CREATE'::text, 'MOVE'::text, 'RENAME'::text, 'DISABLE'::text, 'SET_BUSINESS_UNIT'::text]));
