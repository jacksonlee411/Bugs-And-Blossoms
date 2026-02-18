-- +goose Up
-- +goose StatementBegin
-- DEV-PLAN-106B: sticky effective_date for CORRECT_EVENT chains.

CREATE OR REPLACE VIEW orgunit.org_events_effective AS
WITH correction_events AS (
  SELECT
    e.*,
    (e.payload->>'target_event_uuid')::uuid AS target_event_uuid
  FROM orgunit.org_events e
  WHERE e.event_type IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
    AND e.payload ? 'target_event_uuid'
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
),
latest_effective_date_corrections AS (
  -- Sticky effective_date: take the latest CORRECT_EVENT that explicitly carries effective_date,
  -- regardless of later corrections that don't include effective_date.
  SELECT DISTINCT ON (tenant_uuid, target_event_uuid)
    tenant_uuid,
    target_event_uuid,
    NULLIF(btrim(payload->>'effective_date'), '')::date AS sticky_effective_date,
    tx_time,
    id
  FROM correction_events
  WHERE event_type = 'CORRECT_EVENT'
    AND payload ? 'effective_date'
  ORDER BY tenant_uuid, target_event_uuid, tx_time DESC, id DESC
)
SELECT
  e.id,
  e.event_uuid,
  e.tenant_uuid,
  e.org_id,
  CASE
    WHEN lc.correction_type = 'CORRECT_STATUS'
      AND COALESCE(lc.correction_payload->>'target_status', '') = 'active'
      THEN 'ENABLE'
    WHEN lc.correction_type = 'CORRECT_STATUS'
      AND COALESCE(lc.correction_payload->>'target_status', '') = 'disabled'
      THEN 'DISABLE'
    ELSE e.event_type
  END AS event_type,
  COALESCE(lec.sticky_effective_date, e.effective_date) AS effective_date,
  CASE
    WHEN lc.correction_type = 'CORRECT_EVENT'
      THEN orgunit.merge_org_event_payload_with_correction(e.payload, lc.correction_payload)
    ELSE e.payload
  END AS payload,
  e.request_code,
  e.initiator_uuid,
  e.transaction_time,
  e.created_at
FROM orgunit.org_events e
LEFT JOIN latest_corrections lc
  ON lc.tenant_uuid = e.tenant_uuid
 AND lc.target_event_uuid = e.event_uuid
LEFT JOIN latest_effective_date_corrections lec
  ON lec.tenant_uuid = e.tenant_uuid
 AND lec.target_event_uuid = e.event_uuid
WHERE e.event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
  AND COALESCE(lc.correction_type, '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG');

CREATE OR REPLACE FUNCTION orgunit.org_events_effective_for_replay(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_code text,
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
  request_code text,
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
      e.request_code,
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
      p_pending_request_code,
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
  ),
  latest_effective_date_corrections AS (
    -- Sticky effective_date: take the latest CORRECT_EVENT that explicitly carries effective_date,
    -- regardless of later corrections that don't include effective_date.
    SELECT DISTINCT ON (tenant_uuid, target_event_uuid)
      tenant_uuid,
      target_event_uuid,
      NULLIF(btrim(payload->>'effective_date'), '')::date AS sticky_effective_date,
      tx_time,
      id
    FROM correction_events
    WHERE event_type = 'CORRECT_EVENT'
      AND payload ? 'effective_date'
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
      WHEN lc.correction_type = 'CORRECT_EVENT'
        AND se.event_type <> 'CREATE'
        AND (
          orgunit.merge_org_event_payload_with_correction(se.payload, lc.correction_payload) ?| ARRAY[
            'name',
            'parent_id',
            'status',
            'is_business_unit',
            'manager_uuid',
            'manager_pernr',
            'ext',
            'new_name',
            'new_parent_id'
          ]
        )
        THEN 'UPDATE'
      ELSE se.event_type
    END AS event_type,
    COALESCE(lec.sticky_effective_date, se.effective_date) AS effective_date,
    CASE
      WHEN lc.correction_type = 'CORRECT_EVENT'
        THEN orgunit.merge_org_event_payload_with_correction(se.payload, lc.correction_payload)
      ELSE se.payload
    END AS payload,
    se.request_code,
    se.initiator_uuid,
    se.transaction_time,
    se.created_at
  FROM source_events se
  LEFT JOIN latest_corrections lc
    ON lc.tenant_uuid = se.tenant_uuid
   AND lc.target_event_uuid = se.event_uuid
  LEFT JOIN latest_effective_date_corrections lec
    ON lec.tenant_uuid = se.tenant_uuid
   AND lec.target_event_uuid = se.event_uuid
  WHERE se.event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
    AND COALESCE(lc.correction_type, '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG')
  ORDER BY effective_date, id;
$$;

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No down migration (DEV-PLAN-106B sticky effective_date)
-- +goose StatementEnd

