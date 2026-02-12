-- +goose Up
-- +goose StatementBegin
-- Keep day-slot occupancy aligned with effective event stream: rescinded base events must not block new same-day writes.

DROP INDEX IF EXISTS orgunit.org_events_one_per_day_unique;

CREATE OR REPLACE FUNCTION orgunit.guard_org_events_one_per_day_effective()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.event_type IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
    IF EXISTS (
      SELECT 1
      FROM orgunit.org_events_effective e
      WHERE e.tenant_uuid = NEW.tenant_uuid
        AND e.org_id = NEW.org_id
        AND e.effective_date = NEW.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EVENT_DATE_CONFLICT',
        DETAIL = format('org_id=%s effective_date=%s', NEW.org_id, NEW.effective_date);
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS org_events_guard_one_per_day_effective ON orgunit.org_events;
CREATE TRIGGER org_events_guard_one_per_day_effective
BEFORE INSERT ON orgunit.org_events
FOR EACH ROW
EXECUTE FUNCTION orgunit.guard_org_events_one_per_day_effective();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS org_events_guard_one_per_day_effective ON orgunit.org_events;
DROP FUNCTION IF EXISTS orgunit.guard_org_events_one_per_day_effective();

CREATE UNIQUE INDEX IF NOT EXISTS org_events_one_per_day_unique
  ON orgunit.org_events (tenant_uuid, org_id, effective_date)
  WHERE event_type IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT');
-- +goose StatementEnd
