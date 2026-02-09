-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.org_events
  ADD COLUMN IF NOT EXISTS initiator_name text,
  ADD COLUMN IF NOT EXISTS initiator_employee_id text;

ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_target_event_uuid_required;
ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_target_event_uuid_required CHECK (
    event_type NOT IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
    OR (
      payload ? 'target_event_uuid'
      AND NULLIF(btrim(payload->>'target_event_uuid'), '') IS NOT NULL
    )
  );

DROP INDEX IF EXISTS orgunit.org_events_tenant_tx_time_idx;
CREATE INDEX IF NOT EXISTS org_events_tenant_tx_time_idx
  ON orgunit.org_events (tenant_uuid, tx_time DESC, id DESC);

CREATE OR REPLACE FUNCTION orgunit.fill_org_event_audit_snapshot()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_name text;
  v_employee text;
BEGIN
  IF NEW.tx_time IS NULL THEN
    NEW.tx_time := COALESCE(NEW.transaction_time, now());
  END IF;

  IF NULLIF(btrim(COALESCE(NEW.initiator_name, '')), '') IS NOT NULL
    AND NULLIF(btrim(COALESCE(NEW.initiator_employee_id, '')), '') IS NOT NULL
  THEN
    RETURN NEW;
  END IF;

  IF to_regclass('iam.principals') IS NOT NULL THEN
    SELECT
      COALESCE(NULLIF(btrim(p.display_name), ''), NULLIF(btrim(p.email), ''), NEW.initiator_uuid::text),
      COALESCE(NULLIF(btrim(p.email), ''), NEW.initiator_uuid::text)
    INTO v_name, v_employee
    FROM iam.principals p
    WHERE p.tenant_uuid = NEW.tenant_uuid
      AND p.id = NEW.initiator_uuid
    LIMIT 1;
  END IF;

  NEW.initiator_name := COALESCE(NULLIF(btrim(COALESCE(NEW.initiator_name, '')), ''), v_name, NEW.initiator_uuid::text);
  NEW.initiator_employee_id := COALESCE(NULLIF(btrim(COALESCE(NEW.initiator_employee_id, '')), ''), v_employee, NEW.initiator_uuid::text);

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS org_events_fill_audit_snapshot ON orgunit.org_events;
CREATE TRIGGER org_events_fill_audit_snapshot
BEFORE INSERT ON orgunit.org_events
FOR EACH ROW
EXECUTE FUNCTION orgunit.fill_org_event_audit_snapshot();

DO $$
BEGIN
  IF to_regclass('iam.principals') IS NOT NULL THEN
    UPDATE orgunit.org_events e
    SET initiator_name = COALESCE(NULLIF(btrim(e.initiator_name), ''), p.display_name, p.email, e.initiator_uuid::text),
        initiator_employee_id = COALESCE(NULLIF(btrim(e.initiator_employee_id), ''), p.email, e.initiator_uuid::text)
    FROM iam.principals p
    WHERE p.tenant_uuid = e.tenant_uuid
      AND p.id = e.initiator_uuid
      AND (
        NULLIF(btrim(COALESCE(e.initiator_name, '')), '') IS NULL
        OR NULLIF(btrim(COALESCE(e.initiator_employee_id, '')), '') IS NULL
      );
  END IF;
END $$;

UPDATE orgunit.org_events e
SET initiator_name = COALESCE(NULLIF(btrim(e.initiator_name), ''), e.initiator_uuid::text),
    initiator_employee_id = COALESCE(NULLIF(btrim(e.initiator_employee_id), ''), e.initiator_uuid::text)
WHERE NULLIF(btrim(COALESCE(e.initiator_name, '')), '') IS NULL
   OR NULLIF(btrim(COALESCE(e.initiator_employee_id, '')), '') IS NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    IF to_regnamespace('iam') IS NOT NULL THEN
      GRANT USAGE ON SCHEMA iam TO orgunit_kernel;
    END IF;
    IF to_regclass('iam.principals') IS NOT NULL THEN
      GRANT SELECT ON TABLE iam.principals TO orgunit_kernel;
    END IF;
    ALTER FUNCTION orgunit.fill_org_event_audit_snapshot() OWNER TO orgunit_kernel;
    ALTER FUNCTION orgunit.fill_org_event_audit_snapshot() SECURITY DEFINER;
    ALTER FUNCTION orgunit.fill_org_event_audit_snapshot() SET search_path = pg_catalog, orgunit, iam, public;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
