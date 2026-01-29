-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_group_versions(
  p_tenant_id uuid,
  p_package_id uuid,
  p_job_family_group_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_family_group_versions(p_tenant_id, p_package_id::text, p_job_family_group_id);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_versions(
  p_tenant_id uuid,
  p_package_id uuid,
  p_job_family_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_family_versions(p_tenant_id, p_package_id::text, p_job_family_id);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_level_versions(
  p_tenant_id uuid,
  p_package_id uuid,
  p_job_level_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_level_versions(p_tenant_id, p_package_id::text, p_job_level_id);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_profile_versions(
  p_tenant_id uuid,
  p_package_id uuid,
  p_job_profile_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_profile_versions(p_tenant_id, p_package_id::text, p_job_profile_id);
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
