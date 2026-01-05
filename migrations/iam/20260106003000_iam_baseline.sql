-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS iam;

CREATE OR REPLACE FUNCTION public.current_tenant_id()
RETURNS uuid
LANGUAGE sql
STABLE
AS $$
  SELECT current_setting('app.current_tenant')::uuid;
$$;

CREATE OR REPLACE FUNCTION public.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_tenant_id <> public.current_tenant_id() THEN
    RAISE EXCEPTION 'RLS_TENANT_MISMATCH'
      USING
        ERRCODE = 'P0001',
        DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, public.current_tenant_id());
  END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS public.assert_current_tenant(uuid);
DROP FUNCTION IF EXISTS public.current_tenant_id();
DROP SCHEMA IF EXISTS iam;
DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd

