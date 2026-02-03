-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_group_versions(
  p_tenant_uuid uuid,
  p_package_uuid uuid,
  p_job_family_group_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_family_group_versions(p_tenant_uuid, p_package_uuid::text, p_job_family_group_uuid);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_versions(
  p_tenant_uuid uuid,
  p_package_uuid uuid,
  p_job_family_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_family_versions(p_tenant_uuid, p_package_uuid::text, p_job_family_uuid);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_level_versions(
  p_tenant_uuid uuid,
  p_package_uuid uuid,
  p_job_level_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_level_versions(p_tenant_uuid, p_package_uuid::text, p_job_level_uuid);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_profile_versions(
  p_tenant_uuid uuid,
  p_package_uuid uuid,
  p_job_profile_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_profile_versions(p_tenant_uuid, p_package_uuid::text, p_job_profile_uuid);
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
