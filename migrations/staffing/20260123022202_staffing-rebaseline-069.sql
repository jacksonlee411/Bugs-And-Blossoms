-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;
-- Add new schema named "staffing"
CREATE SCHEMA "staffing";
-- create "assignment_event_corrections" table
CREATE TABLE "staffing"."assignment_event_corrections" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "target_effective_date" date NOT NULL,
  "replacement_payload" jsonb NOT NULL,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_event_corrections_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "assignment_event_corrections_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "assignment_event_corrections_target_unique" UNIQUE ("tenant_id", "assignment_id", "target_effective_date"),
  CONSTRAINT "assignment_event_corrections_replacement_payload_obj_check" CHECK (jsonb_typeof(replacement_payload) = 'object'::text)
);
-- create "assignment_event_rescinds" table
CREATE TABLE "staffing"."assignment_event_rescinds" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "target_effective_date" date NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_event_rescinds_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "assignment_event_rescinds_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "assignment_event_rescinds_target_unique" UNIQUE ("tenant_id", "assignment_id", "target_effective_date"),
  CONSTRAINT "assignment_event_rescinds_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text)
);
-- create "assignments" table
CREATE TABLE "staffing"."assignments" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "person_uuid" uuid NOT NULL,
  "assignment_type" text NOT NULL DEFAULT 'primary',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "assignments_tenant_person_type_unique" UNIQUE ("tenant_id", "person_uuid", "assignment_type"),
  CONSTRAINT "assignments_assignment_type_check" CHECK (assignment_type = 'primary'::text)
);
-- create "assignment_events" table
CREATE TABLE "staffing"."assignment_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "assignment_type" text NOT NULL DEFAULT 'primary',
  "event_type" text NOT NULL,
  "effective_date" date NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "assignment_events_one_per_day_unique" UNIQUE ("tenant_id", "assignment_id", "effective_date"),
  CONSTRAINT "assignment_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "assignment_events_assignment_fk" FOREIGN KEY ("tenant_id", "assignment_id") REFERENCES "staffing"."assignments" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "assignment_events_assignment_type_check" CHECK (assignment_type = 'primary'::text),
  CONSTRAINT "assignment_events_event_type_check" CHECK (event_type = ANY (ARRAY['CREATE'::text, 'UPDATE'::text])),
  CONSTRAINT "assignment_events_payload_allowed_keys_check" CHECK (((((payload - 'position_id'::text) - 'status'::text) - 'allocated_fte'::text) - 'profile'::text) = '{}'::jsonb),
  CONSTRAINT "assignment_events_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text)
);
-- create index "assignment_events_tenant_assignment_effective_idx" to table: "assignment_events"
CREATE INDEX "assignment_events_tenant_assignment_effective_idx" ON "staffing"."assignment_events" ("tenant_id", "assignment_id", "effective_date", "id");
-- create "positions" table
CREATE TABLE "staffing"."positions" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id")
);
-- create "assignment_versions" table
CREATE TABLE "staffing"."assignment_versions" (
  "id" bigserial NOT NULL,
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "position_id" uuid NOT NULL,
  "assignment_type" text NOT NULL DEFAULT 'primary',
  "status" text NOT NULL DEFAULT 'active',
  "allocated_fte" numeric(9,2) NOT NULL DEFAULT 1.0,
  "validity" daterange NOT NULL,
  "last_event_id" bigint NOT NULL,
  "profile" jsonb NOT NULL DEFAULT '{}',
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_versions_no_overlap" EXCLUDE USING gist ("tenant_id" WITH =, "assignment_id" WITH =, "validity" WITH &&),
  CONSTRAINT "assignment_versions_position_no_overlap" EXCLUDE USING gist ("tenant_id" WITH =, "position_id" WITH =, "validity" WITH &&) WHERE (status = 'active'::text),
  CONSTRAINT "assignment_versions_assignment_fk" FOREIGN KEY ("tenant_id", "assignment_id") REFERENCES "staffing"."assignments" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "assignment_versions_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."assignment_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "assignment_versions_position_fk" FOREIGN KEY ("tenant_id", "position_id") REFERENCES "staffing"."positions" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "assignment_versions_allocated_fte_check" CHECK (allocated_fte > (0)::numeric),
  CONSTRAINT "assignment_versions_assignment_type_check" CHECK (assignment_type = 'primary'::text),
  CONSTRAINT "assignment_versions_profile_is_object_check" CHECK (jsonb_typeof(profile) = 'object'::text),
  CONSTRAINT "assignment_versions_status_check" CHECK (status = ANY (ARRAY['active'::text, 'inactive'::text])),
  CONSTRAINT "assignment_versions_validity_bounds_check" CHECK (lower_inc(validity) AND (NOT upper_inc(validity))),
  CONSTRAINT "assignment_versions_validity_check" CHECK (NOT isempty(validity))
);
-- create index "assignment_versions_person_lookup_btree" to table: "assignment_versions"
CREATE INDEX "assignment_versions_person_lookup_btree" ON "staffing"."assignment_versions" ("tenant_id", "person_uuid", (lower(validity)));
-- create "position_events" table
CREATE TABLE "staffing"."position_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "position_id" uuid NOT NULL,
  "event_type" text NOT NULL,
  "effective_date" date NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "position_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "position_events_one_per_day_unique" UNIQUE ("tenant_id", "position_id", "effective_date"),
  CONSTRAINT "position_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "position_events_position_fk" FOREIGN KEY ("tenant_id", "position_id") REFERENCES "staffing"."positions" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "position_events_event_type_check" CHECK (event_type = ANY (ARRAY['CREATE'::text, 'UPDATE'::text])),
  CONSTRAINT "position_events_payload_allowed_keys_check" CHECK ((((((payload - 'org_unit_id'::text) - 'name'::text) - 'reports_to_position_id'::text) - 'job_profile_id'::text) - 'lifecycle_status'::text) - 'capacity_fte'::text) = '{}'::jsonb),
  CONSTRAINT "position_events_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text)
);
-- create index "position_events_tenant_position_effective_idx" to table: "position_events"
CREATE INDEX "position_events_tenant_position_effective_idx" ON "staffing"."position_events" ("tenant_id", "position_id", "effective_date", "id");
-- create "position_versions" table
CREATE TABLE "staffing"."position_versions" (
  "id" bigserial NOT NULL,
  "tenant_id" uuid NOT NULL,
  "position_id" uuid NOT NULL,
  "org_unit_id" uuid NOT NULL,
  "reports_to_position_id" uuid NULL,
  "name" text NULL,
  "lifecycle_status" text NOT NULL DEFAULT 'active',
  "capacity_fte" numeric(9,2) NOT NULL DEFAULT 1.0,
  "profile" jsonb NOT NULL DEFAULT '{}',
  "validity" daterange NOT NULL,
  "last_event_id" bigint NOT NULL,
  "jobcatalog_setid" text NULL,
  "jobcatalog_setid_as_of" date NULL,
  "job_profile_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "position_versions_no_overlap" EXCLUDE USING gist ("tenant_id" WITH =, "position_id" WITH =, "validity" WITH &&),
  CONSTRAINT "position_versions_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."position_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "position_versions_position_fk" FOREIGN KEY ("tenant_id", "position_id") REFERENCES "staffing"."positions" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "position_versions_reports_to_fk" FOREIGN KEY ("tenant_id", "reports_to_position_id") REFERENCES "staffing"."positions" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "position_versions_capacity_fte_check" CHECK (capacity_fte > (0)::numeric),
  CONSTRAINT "position_versions_job_profile_requires_setid_check" CHECK ((job_profile_id IS NULL) OR (jobcatalog_setid IS NOT NULL)),
  CONSTRAINT "position_versions_jobcatalog_setid_format_check" CHECK ((jobcatalog_setid IS NULL) OR (jobcatalog_setid ~ '^[A-Z0-9]{5}$'::text)),
  CONSTRAINT "position_versions_jobcatalog_setid_as_of_check" CHECK ((jobcatalog_setid IS NULL) OR (jobcatalog_setid_as_of IS NOT NULL)),
  CONSTRAINT "position_versions_lifecycle_status_check" CHECK (lifecycle_status = ANY (ARRAY['active'::text, 'disabled'::text])),
  CONSTRAINT "position_versions_profile_is_object_check" CHECK (jsonb_typeof(profile) = 'object'::text),
  CONSTRAINT "position_versions_validity_bounds_check" CHECK (lower_inc(validity) AND (NOT upper_inc(validity))),
  CONSTRAINT "position_versions_validity_check" CHECK (NOT isempty(validity))
);
-- create index "position_versions_lookup_btree" to table: "position_versions"
CREATE INDEX "position_versions_lookup_btree" ON "staffing"."position_versions" ("tenant_id", "position_id", (lower(validity)));

ALTER TABLE "staffing"."positions" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."positions" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."positions";
CREATE POLICY tenant_isolation ON "staffing"."positions"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."position_events" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."position_events" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."position_events";
CREATE POLICY tenant_isolation ON "staffing"."position_events"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."position_versions" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."position_versions" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."position_versions";
CREATE POLICY tenant_isolation ON "staffing"."position_versions"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."assignments" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."assignments" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."assignments";
CREATE POLICY tenant_isolation ON "staffing"."assignments"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."assignment_events" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."assignment_events" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."assignment_events";
CREATE POLICY tenant_isolation ON "staffing"."assignment_events"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."assignment_event_corrections" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."assignment_event_corrections" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."assignment_event_corrections";
CREATE POLICY tenant_isolation ON "staffing"."assignment_event_corrections"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."assignment_event_rescinds" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."assignment_event_rescinds" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."assignment_event_rescinds";
CREATE POLICY tenant_isolation ON "staffing"."assignment_event_rescinds"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE "staffing"."assignment_versions" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."assignment_versions" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."assignment_versions";
CREATE POLICY tenant_isolation ON "staffing"."assignment_versions"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'tenant_id is required';
  END IF;

  v_ctx_raw := current_setting('app.current_tenant', true);
  IF v_ctx_raw IS NULL OR btrim(v_ctx_raw) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISSING',
      DETAIL = 'app.current_tenant is required';
  END IF;

  BEGIN
    v_ctx_tenant := v_ctx_raw::uuid;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'RLS_TENANT_CONTEXT_INVALID',
        DETAIL = format('app.current_tenant=%s', v_ctx_raw);
  END;

  IF v_ctx_tenant <> p_tenant_id THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, v_ctx_tenant);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_position_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_position_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_reports_to_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.position_events%ROWTYPE;
  v_payload jsonb;
  v_prev_effective_max date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'position_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  IF v_payload ? 'reports_to_position_id' THEN
    v_reports_to_lock_key := format('staffing:position-reports-to:%s', p_tenant_id);
    PERFORM pg_advisory_xact_lock(hashtextextended(v_reports_to_lock_key, 0));
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_id, p_position_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.positions (tenant_id, id)
  VALUES (p_tenant_id, p_position_id)
  ON CONFLICT DO NOTHING;

  INSERT INTO staffing.position_events (
    event_id,
    tenant_id,
    position_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_position_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.position_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.position_id <> p_position_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  IF p_event_type = 'UPDATE' AND v_payload ? 'reports_to_position_id' THEN
    SELECT max(effective_date) INTO v_prev_effective_max
    FROM staffing.position_events
    WHERE tenant_id = p_tenant_id
      AND position_id = p_position_id
      AND id <> v_event_db_id;

    IF v_prev_effective_max IS NOT NULL AND p_effective_date <= v_prev_effective_max THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('reports_to_position_id updates must be forward-only: effective_date=%s last_effective_date=%s', p_effective_date, v_prev_effective_max);
    END IF;
  END IF;

  PERFORM staffing.replay_position_versions(p_tenant_id, p_position_id);

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.replay_position_versions(
  p_tenant_id uuid,
  p_position_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_org_unit_id uuid;
  v_reports_to_position_id uuid;
  v_jobcatalog_setid text;
  v_jobcatalog_setid_as_of date;
  v_job_profile_id uuid;
  v_name text;
  v_lifecycle_status text;
  v_capacity_fte numeric(9,2);
  v_profile jsonb;
  v_tmp_text text;
  v_target_status text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_id is required';
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_id, p_position_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id;

  v_org_unit_id := NULL;
  v_reports_to_position_id := NULL;
  v_jobcatalog_setid := NULL;
  v_jobcatalog_setid_as_of := NULL;
  v_job_profile_id := NULL;
  v_name := NULL;
  v_lifecycle_status := 'active';
  v_capacity_fte := 1.0;
  v_profile := '{}'::jsonb;
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(effective_date) OVER (ORDER BY effective_date ASC, id ASC) AS next_effective
    FROM staffing.position_events e
    WHERE e.tenant_id = p_tenant_id
      AND e.position_id = p_position_id
    ORDER BY effective_date ASC, id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_tmp_text := NULLIF(btrim(v_row.payload->>'org_unit_id'), '');
      IF v_tmp_text IS NULL THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'org_unit_id is required';
      END IF;
      BEGIN
        v_org_unit_id := v_tmp_text::uuid;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('org_unit_id=%s', v_row.payload->>'org_unit_id');
      END;

      v_name := NULLIF(btrim(v_row.payload->>'name'), '');

      IF v_row.payload ? 'reports_to_position_id' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_id'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_id := NULL;
        ELSE
          BEGIN
            v_reports_to_position_id := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_id=%s', v_row.payload->>'reports_to_position_id');
          END;
        END IF;
      ELSE
        v_reports_to_position_id := NULL;
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      ELSE
        v_lifecycle_status := 'active';
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      ELSE
        v_capacity_fte := 1.0;
      END IF;

      IF v_row.payload ? 'job_profile_id' THEN
        IF v_row.payload->'job_profile_id' IS NULL THEN
          v_job_profile_id := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_id'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_id := NULL;
          ELSE
            BEGIN
              v_job_profile_id := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_id=%s', v_row.payload->>'job_profile_id');
            END;
          END IF;
        END IF;
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'org_unit_id' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'org_unit_id'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'org_unit_id is required';
        END IF;
        BEGIN
          v_org_unit_id := v_tmp_text::uuid;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('org_unit_id=%s', v_row.payload->>'org_unit_id');
        END;
      END IF;

      IF v_row.payload ? 'reports_to_position_id' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_id'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_id := NULL;
        ELSE
          BEGIN
            v_reports_to_position_id := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_id=%s', v_row.payload->>'reports_to_position_id');
          END;
        END IF;
      END IF;

      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'job_profile_id' THEN
        IF v_row.payload->'job_profile_id' IS NULL THEN
          v_job_profile_id := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_id'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_id := NULL;
          ELSE
            BEGIN
              v_job_profile_id := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_id=%s', v_row.payload->>'job_profile_id');
            END;
          END IF;
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF v_org_unit_id IS NOT NULL THEN
      v_jobcatalog_setid := orgunit.resolve_setid(p_tenant_id, v_org_unit_id, v_row.effective_date);
      v_jobcatalog_setid_as_of := v_row.effective_date;
    ELSE
      v_jobcatalog_setid := NULL;
      v_jobcatalog_setid_as_of := NULL;
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.org_unit_versions ouv
      WHERE ouv.tenant_id = p_tenant_id
        AND ouv.hierarchy_type = 'OrgUnit'
        AND ouv.org_id = v_org_unit_id
        AND ouv.status = 'active'
        AND ouv.validity @> v_row.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_ORG_UNIT_NOT_FOUND_AS_OF',
        DETAIL = format('org_unit_id=%s as_of=%s', v_org_unit_id, v_row.effective_date);
    END IF;

    IF v_job_profile_id IS NOT NULL THEN
      IF v_jobcatalog_setid IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
          DETAIL = format('job_profile_id=%s', v_job_profile_id);
      END IF;

      IF NOT EXISTS (
        SELECT 1
        FROM jobcatalog.job_profile_versions jpv
        WHERE jpv.tenant_id = p_tenant_id
          AND jpv.setid = v_jobcatalog_setid
          AND jpv.job_profile_id = v_job_profile_id
          AND jpv.is_active = true
          AND jpv.validity @> v_row.effective_date
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
          DETAIL = format('job_profile_id=%s', v_job_profile_id);
      END IF;
    END IF;

    IF v_reports_to_position_id IS NOT NULL THEN
      IF v_reports_to_position_id = p_position_id THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_SELF',
          DETAIL = format('position_id=%s', p_position_id);
      END IF;

      SELECT lifecycle_status INTO v_target_status
      FROM staffing.position_versions pv
      WHERE pv.tenant_id = p_tenant_id
        AND pv.position_id = v_reports_to_position_id
        AND pv.validity @> v_row.effective_date
      ORDER BY lower(pv.validity) DESC
      LIMIT 1;
      IF NOT FOUND THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;
      IF v_target_status <> 'active' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;

      IF EXISTS (
        WITH RECURSIVE chain AS (
          SELECT pv.position_id, pv.reports_to_position_id
          FROM staffing.position_versions pv
          WHERE pv.tenant_id = p_tenant_id
            AND pv.position_id = v_reports_to_position_id
            AND pv.validity @> v_row.effective_date
          UNION ALL
          SELECT pv.position_id, pv.reports_to_position_id
          FROM staffing.position_versions pv
          JOIN chain c ON pv.position_id = c.reports_to_position_id
          WHERE pv.tenant_id = p_tenant_id
            AND pv.validity @> v_row.effective_date
            AND c.reports_to_position_id IS NOT NULL
        )
        SELECT 1
        FROM chain
        WHERE position_id = p_position_id
           OR reports_to_position_id = p_position_id
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_CYCLE',
          DETAIL = format('position_id=%s', p_position_id);
      END IF;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_lifecycle_status = 'disabled' THEN
      IF EXISTS (
        SELECT 1
        FROM staffing.assignment_versions av
        WHERE av.tenant_id = p_tenant_id
          AND av.position_id = p_position_id
          AND av.status = 'active'
          AND av.validity && v_validity
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', p_position_id, v_row.effective_date);
      END IF;
    END IF;

    INSERT INTO staffing.position_versions (
      tenant_id,
      position_id,
      org_unit_id,
      reports_to_position_id,
      name,
      lifecycle_status,
      capacity_fte,
      profile,
      validity,
      last_event_id,
      jobcatalog_setid,
      jobcatalog_setid_as_of,
      job_profile_id
    )
    VALUES (
      p_tenant_id,
      p_position_id,
      v_org_unit_id,
      v_reports_to_position_id,
      v_name,
      v_lifecycle_status,
      v_capacity_fte,
      v_profile,
      v_validity,
      v_row.event_db_id,
      v_jobcatalog_setid,
      v_jobcatalog_setid_as_of,
      v_job_profile_id
    );

    IF v_lifecycle_status = 'active' THEN
      PERFORM staffing.assert_position_capacity(p_tenant_id, p_position_id, v_validity);
    END IF;

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.position_versions
      WHERE tenant_id = p_tenant_id AND position_id = p_position_id
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_GAP',
      DETAIL = 'position_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id
  ORDER BY lower(validity) DESC
  LIMIT 1;
  IF v_last_validity IS NOT NULL AND upper(v_last_validity) IS NOT NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'position_versions must end at infinity';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.assert_position_capacity(
  p_tenant_id uuid,
  p_position_id uuid,
  p_validity daterange
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_id is required';
  END IF;
  IF p_validity IS NULL OR isempty(p_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'validity is required';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.assignment_versions av
    JOIN staffing.position_versions pv
      ON pv.tenant_id = av.tenant_id
     AND pv.position_id = av.position_id
     AND pv.validity && av.validity
    WHERE av.tenant_id = p_tenant_id
      AND av.position_id = p_position_id
      AND av.status = 'active'
      AND av.validity && p_validity
      AND av.allocated_fte > pv.capacity_fte
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_POSITION_CAPACITY_EXCEEDED',
      DETAIL = format('position_id=%s', p_position_id);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.replay_assignment_versions(
  p_tenant_id uuid,
  p_assignment_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_person_uuid uuid;
  v_assignment_type text;
  v_position_id uuid;
  v_status text;
  v_allocated_fte numeric(9,2);
  v_profile jsonb;
  v_tmp_text text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'assignment_id is required';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.assignment_versions
  WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id;

  v_person_uuid := NULL;
  v_assignment_type := NULL;
  v_position_id := NULL;
  v_status := 'active';
  v_allocated_fte := 1.0;
  v_profile := '{}'::jsonb;
  v_prev_effective := NULL;

  FOR v_row IN
    WITH base AS (
      SELECT
        e.id AS event_db_id,
        e.event_type,
        e.effective_date,
        e.person_uuid,
        e.assignment_type,
        COALESCE(c.replacement_payload, e.payload) AS payload,
        (r.id IS NOT NULL) AS is_rescinded
      FROM staffing.assignment_events e
      LEFT JOIN staffing.assignment_event_corrections c
        ON c.tenant_id = e.tenant_id
       AND c.assignment_id = e.assignment_id
       AND c.target_effective_date = e.effective_date
      LEFT JOIN staffing.assignment_event_rescinds r
        ON r.tenant_id = e.tenant_id
       AND r.assignment_id = e.assignment_id
       AND r.target_effective_date = e.effective_date
      WHERE e.tenant_id = p_tenant_id
        AND e.assignment_id = p_assignment_id
    ),
    filtered AS (
      SELECT *
      FROM base
      WHERE NOT is_rescinded
    ),
    ordered AS (
      SELECT
        event_db_id,
        event_type,
        effective_date,
        person_uuid,
        assignment_type,
        payload,
        lead(effective_date) OVER (ORDER BY effective_date ASC, event_db_id ASC) AS next_effective
      FROM filtered
    )
    SELECT *
    FROM ordered
    ORDER BY effective_date ASC, event_db_id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_person_uuid := v_row.person_uuid;
      v_assignment_type := v_row.assignment_type;

      v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
      IF v_position_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'position_id is required';
      END IF;
      v_status := 'active';

      IF v_row.payload ? 'allocated_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'allocated_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = 'allocated_fte is required';
        END IF;
        BEGIN
          v_allocated_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
              DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END;
        IF v_allocated_fte <= 0 OR v_allocated_fte > 1 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'profile' THEN
        IF jsonb_typeof(v_row.payload->'profile') <> 'object' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_PROFILE_INVALID',
            DETAIL = 'profile must be an object';
        END IF;
        v_profile := v_row.payload->'profile';
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'position_id' THEN
        v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
        IF v_position_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'position_id is required';
	      END IF;
      END IF;

      IF v_row.payload ? 'status' THEN
        v_status := NULLIF(btrim(v_row.payload->>'status'), '');
        IF v_status IS NULL OR v_status NOT IN ('active','inactive') THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid status: %s', v_row.payload->>'status');
	        END IF;
      END IF;

      IF v_row.payload ? 'allocated_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'allocated_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = 'allocated_fte is required';
        END IF;
        BEGIN
          v_allocated_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
              DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END;
        IF v_allocated_fte <= 0 OR v_allocated_fte > 1 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'profile' THEN
        IF jsonb_typeof(v_row.payload->'profile') <> 'object' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_PROFILE_INVALID',
            DETAIL = 'profile must be an object';
        END IF;
        v_profile := v_row.payload->'profile';
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_status = 'active' THEN
      IF NOT EXISTS (
        SELECT 1
        FROM staffing.position_versions pv
        WHERE pv.tenant_id = p_tenant_id
          AND pv.position_id = v_position_id
          AND pv.lifecycle_status = 'active'
          AND pv.validity @> v_row.effective_date
        LIMIT 1
      ) THEN
        IF EXISTS (
          SELECT 1
          FROM staffing.position_versions pv
          WHERE pv.tenant_id = p_tenant_id
            AND pv.position_id = v_position_id
            AND pv.validity @> v_row.effective_date
          LIMIT 1
        ) THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
            DETAIL = format('position_id=%s as_of=%s', v_position_id, v_row.effective_date);
        END IF;

        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_position_id, v_row.effective_date);
      END IF;
    END IF;

    INSERT INTO staffing.assignment_versions (
      tenant_id,
      assignment_id,
      person_uuid,
      position_id,
      assignment_type,
      status,
      allocated_fte,
      profile,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_assignment_id,
      v_person_uuid,
      v_position_id,
      v_assignment_type,
      v_status,
      v_allocated_fte,
      v_profile,
      v_validity,
      v_row.event_db_id
    );

    IF v_status = 'active' THEN
      PERFORM staffing.assert_position_capacity(p_tenant_id, v_position_id, v_validity);
    END IF;

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.assignment_versions
      WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_GAP',
      DETAIL = 'assignment_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.assignment_versions
  WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last assignment version validity must be unbounded (infinity)';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_person_uuid uuid,
  p_assignment_type text,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.assignment_events%ROWTYPE;
  v_payload jsonb;
  v_existing_assignment_id uuid;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_assignment_type IS NULL OR btrim(p_assignment_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_type is required';
  END IF;
  IF p_assignment_type <> 'primary' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported assignment_type: %s', p_assignment_type);
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.assignments (tenant_id, id, person_uuid, assignment_type)
  VALUES (p_tenant_id, p_assignment_id, p_person_uuid, p_assignment_type)
  ON CONFLICT (tenant_id, person_uuid, assignment_type) DO NOTHING;

  SELECT id INTO v_existing_assignment_id
  FROM staffing.assignments
  WHERE tenant_id = p_tenant_id AND person_uuid = p_person_uuid AND assignment_type = p_assignment_type;

  IF v_existing_assignment_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'assignment identity missing';
  END IF;
  IF v_existing_assignment_id <> p_assignment_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_ID_MISMATCH',
      DETAIL = format('assignment_id=%s existing_id=%s', p_assignment_id, v_existing_assignment_id);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  INSERT INTO staffing.assignment_events (
    event_id,
    tenant_id,
    assignment_id,
    person_uuid,
    assignment_type,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_person_uuid,
    p_assignment_type,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.assignment_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.assignment_id <> p_assignment_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.assignment_type <> p_assignment_type
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_correction(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_target_effective_date date,
  p_replacement_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target staffing.assignment_events%ROWTYPE;
  v_existing_by_event staffing.assignment_event_corrections%ROWTYPE;
  v_existing_by_request staffing.assignment_event_corrections%ROWTYPE;
  v_existing_by_target staffing.assignment_event_corrections%ROWTYPE;
  v_payload jsonb;
  v_correction_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_replacement_payload IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'replacement_payload is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := p_replacement_payload;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'replacement_payload must be an object';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_target
  FROM staffing.assignment_events
  WHERE tenant_id = p_tenant_id
    AND assignment_id = p_assignment_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.assignment_event_rescinds r
    WHERE r.tenant_id = p_tenant_id
      AND r.assignment_id = p_assignment_id
      AND r.target_effective_date = p_target_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  INSERT INTO staffing.assignment_event_corrections (
    event_id,
    tenant_id,
    assignment_id,
    target_effective_date,
    replacement_payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_correction_db_id;

  IF v_correction_db_id IS NULL THEN
    SELECT * INTO v_existing_by_event
    FROM staffing.assignment_event_corrections
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.assignment_id <> p_assignment_id
        OR v_existing_by_event.target_effective_date <> p_target_effective_date
        OR v_existing_by_event.replacement_payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      v_correction_db_id := v_existing_by_event.id;
    ELSE
      SELECT * INTO v_existing_by_request
      FROM staffing.assignment_event_corrections
      WHERE tenant_id = p_tenant_id
        AND request_id = p_request_id
      LIMIT 1;

      IF FOUND THEN
        IF v_existing_by_request.tenant_id <> p_tenant_id
          OR v_existing_by_request.assignment_id <> p_assignment_id
          OR v_existing_by_request.target_effective_date <> p_target_effective_date
          OR v_existing_by_request.replacement_payload <> v_payload
          OR v_existing_by_request.request_id <> p_request_id
          OR v_existing_by_request.initiator_id <> p_initiator_id
        THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
            DETAIL = format('request_id=%s existing_id=%s', p_request_id, v_existing_by_request.id);
        END IF;
        v_correction_db_id := v_existing_by_request.id;
      ELSE
        SELECT * INTO v_existing_by_target
        FROM staffing.assignment_event_corrections
        WHERE tenant_id = p_tenant_id
          AND assignment_id = p_assignment_id
          AND target_effective_date = p_target_effective_date
        LIMIT 1;

        IF FOUND THEN
          IF v_existing_by_target.replacement_payload = v_payload THEN
            v_correction_db_id := v_existing_by_target.id;
          ELSE
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_CORRECTED',
              DETAIL = format('assignment_id=%s target_effective_date=%s existing_id=%s', p_assignment_id, p_target_effective_date, v_existing_by_target.id);
          END IF;
        ELSE
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'correction insert failed';
        END IF;
      END IF;
    END IF;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  RETURN v_correction_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_rescind(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_target_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target staffing.assignment_events%ROWTYPE;
  v_existing_by_event staffing.assignment_event_rescinds%ROWTYPE;
  v_existing_by_request staffing.assignment_event_rescinds%ROWTYPE;
  v_existing_by_target staffing.assignment_event_rescinds%ROWTYPE;
  v_payload jsonb;
  v_rescind_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_target
  FROM staffing.assignment_events
  WHERE tenant_id = p_tenant_id
    AND assignment_id = p_assignment_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  IF v_target.event_type = 'CREATE' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  INSERT INTO staffing.assignment_event_rescinds (
    event_id,
    tenant_id,
    assignment_id,
    target_effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_rescind_db_id;

  IF v_rescind_db_id IS NULL THEN
    SELECT * INTO v_existing_by_event
    FROM staffing.assignment_event_rescinds
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.assignment_id <> p_assignment_id
        OR v_existing_by_event.target_effective_date <> p_target_effective_date
        OR v_existing_by_event.payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      v_rescind_db_id := v_existing_by_event.id;
    ELSE
      SELECT * INTO v_existing_by_request
      FROM staffing.assignment_event_rescinds
      WHERE tenant_id = p_tenant_id
        AND request_id = p_request_id
      LIMIT 1;

      IF FOUND THEN
        IF v_existing_by_request.tenant_id <> p_tenant_id
          OR v_existing_by_request.assignment_id <> p_assignment_id
          OR v_existing_by_request.target_effective_date <> p_target_effective_date
          OR v_existing_by_request.payload <> v_payload
          OR v_existing_by_request.request_id <> p_request_id
          OR v_existing_by_request.initiator_id <> p_initiator_id
        THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
            DETAIL = format('request_id=%s existing_id=%s', p_request_id, v_existing_by_request.id);
        END IF;
        v_rescind_db_id := v_existing_by_request.id;
      ELSE
        SELECT * INTO v_existing_by_target
        FROM staffing.assignment_event_rescinds
        WHERE tenant_id = p_tenant_id
          AND assignment_id = p_assignment_id
          AND target_effective_date = p_target_effective_date
        LIMIT 1;

        IF FOUND THEN
          v_rescind_db_id := v_existing_by_target.id;
        ELSE
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'rescind insert failed';
        END IF;
      END IF;
    END IF;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  RETURN v_rescind_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.get_position_snapshot(
  p_tenant_id uuid,
  p_query_date date
)
RETURNS TABLE (
  position_id uuid,
  org_unit_id uuid,
  reports_to_position_id uuid,
  jobcatalog_setid text,
  jobcatalog_setid_as_of date,
  job_profile_id uuid,
  job_profile_code text,
  name text,
  lifecycle_status text,
  capacity_fte numeric(9,2),
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

	  RETURN QUERY
	  SELECT
    pv.position_id,
    pv.org_unit_id,
    pv.reports_to_position_id,
    pv.jobcatalog_setid,
    pv.jobcatalog_setid_as_of,
    pv.job_profile_id,
	    jp.code::text AS job_profile_code,
	    pv.name,
	    pv.lifecycle_status,
	    pv.capacity_fte,
	    lower(pv.validity) AS effective_date
	  FROM staffing.position_versions pv
	  LEFT JOIN jobcatalog.job_profiles jp
	    ON jp.tenant_id = pv.tenant_id
   AND jp.setid = pv.jobcatalog_setid
   AND jp.id = pv.job_profile_id
  WHERE pv.tenant_id = p_tenant_id
    AND pv.validity @> p_query_date;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.get_assignment_snapshot(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  assignment_id uuid,
  person_uuid uuid,
  position_id uuid,
  status text,
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'person_uuid is required';
  END IF;
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  RETURN QUERY
  SELECT
    av.assignment_id,
    av.person_uuid,
    av.position_id,
    av.status,
    lower(av.validity) AS effective_date
  FROM staffing.assignment_versions av
  WHERE av.tenant_id = p_tenant_id
    AND av.person_uuid = p_person_uuid
    AND av.validity @> p_query_date;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- reverse: create index "position_versions_lookup_btree" to table: "position_versions"
DROP INDEX "staffing"."position_versions_lookup_btree";
-- reverse: create "position_versions" table
DROP TABLE "staffing"."position_versions";
-- reverse: create index "position_events_tenant_position_effective_idx" to table: "position_events"
DROP INDEX "staffing"."position_events_tenant_position_effective_idx";
-- reverse: create "position_events" table
DROP TABLE "staffing"."position_events";
-- reverse: create index "assignment_versions_person_lookup_btree" to table: "assignment_versions"
DROP INDEX "staffing"."assignment_versions_person_lookup_btree";
-- reverse: create "assignment_versions" table
DROP TABLE "staffing"."assignment_versions";
-- reverse: create "positions" table
DROP TABLE "staffing"."positions";
-- reverse: create index "assignment_events_tenant_assignment_effective_idx" to table: "assignment_events"
DROP INDEX "staffing"."assignment_events_tenant_assignment_effective_idx";
-- reverse: create "assignment_events" table
DROP TABLE "staffing"."assignment_events";
-- reverse: create "assignments" table
DROP TABLE "staffing"."assignments";
-- reverse: create "assignment_event_rescinds" table
DROP TABLE "staffing"."assignment_event_rescinds";
-- reverse: create "assignment_event_corrections" table
DROP TABLE "staffing"."assignment_event_corrections";
-- reverse: Add new schema named "staffing"
DROP SCHEMA "staffing" CASCADE;
