-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION person.assert_current_tenant(p_tenant_id uuid)
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
      MESSAGE = 'PERSON_INVALID_ARGUMENT',
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

CREATE OR REPLACE FUNCTION person.normalize_pernr(p_pernr text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_pernr IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PERSON_PERNR_INVALID',
      DETAIL = 'pernr is required';
  END IF;

  v := btrim(p_pernr);
  IF v = '' OR v !~ '^[0-9]{1,8}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PERSON_PERNR_INVALID',
      DETAIL = format('pernr=%s', v);
  END IF;

  v := regexp_replace(v, '^0+', '');
  IF v = '' THEN
    v := '0';
  END IF;
  RETURN v;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS person.normalize_pernr(text);
DROP FUNCTION IF EXISTS person.assert_current_tenant(uuid);
-- +goose StatementEnd

