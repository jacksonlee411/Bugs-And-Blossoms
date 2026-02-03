-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_tenant_id uuid;
  v_global_tenant uuid := '00000000-0000-0000-0000-000000000000';
BEGIN
  IF to_regclass('iam.tenants') IS NOT NULL THEN
    FOR v_tenant_id IN
      SELECT id FROM iam.tenants WHERE id <> v_global_tenant
    LOOP
      PERFORM set_config('app.current_tenant', v_tenant_id::text, true);

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_family_groups
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_family_groups', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_family_group_events
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_family_group_events', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_family_group_versions
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_family_group_versions', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_families
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_families', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_family_events
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_family_events', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_family_versions
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_family_versions', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_levels
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_levels', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_level_events
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_level_events', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_level_versions
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_level_versions', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_profiles
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_profiles', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_profile_events
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_profile_events', v_tenant_id);
    END IF;

    IF EXISTS (
      SELECT 1 FROM jobcatalog.job_profile_versions
      WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_FORMAT_INVALID',
        DETAIL = format('tenant_uuid=%s table=jobcatalog.job_profile_versions', v_tenant_id);
    END IF;

      IF EXISTS (
        SELECT 1 FROM jobcatalog.job_profile_version_job_families
        WHERE tenant_uuid = v_tenant_id AND setid !~ '^[A-Z0-9]{5}$'
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SETID_FORMAT_INVALID',
          DETAIL = format('tenant_uuid=%s table=jobcatalog.job_profile_version_job_families', v_tenant_id);
      END IF;
    END LOOP;
  END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE jobcatalog.job_family_groups
  DROP CONSTRAINT IF EXISTS job_family_groups_setid_format_check;
ALTER TABLE jobcatalog.job_family_groups
  ADD CONSTRAINT job_family_groups_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_family_group_events
  DROP CONSTRAINT IF EXISTS job_family_group_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_events
  ADD CONSTRAINT job_family_group_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_versions
  ADD CONSTRAINT job_family_group_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_families
  DROP CONSTRAINT IF EXISTS job_families_setid_format_check;
ALTER TABLE jobcatalog.job_families
  ADD CONSTRAINT job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_family_events
  DROP CONSTRAINT IF EXISTS job_family_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_events
  ADD CONSTRAINT job_family_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_versions
  ADD CONSTRAINT job_family_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_levels
  DROP CONSTRAINT IF EXISTS job_levels_setid_format_check;
ALTER TABLE jobcatalog.job_levels
  ADD CONSTRAINT job_levels_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_level_events
  DROP CONSTRAINT IF EXISTS job_level_events_setid_format_check;
ALTER TABLE jobcatalog.job_level_events
  ADD CONSTRAINT job_level_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_level_versions
  DROP CONSTRAINT IF EXISTS job_level_versions_setid_format_check;
ALTER TABLE jobcatalog.job_level_versions
  ADD CONSTRAINT job_level_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_profiles
  DROP CONSTRAINT IF EXISTS job_profiles_setid_format_check;
ALTER TABLE jobcatalog.job_profiles
  ADD CONSTRAINT job_profiles_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_profile_events
  DROP CONSTRAINT IF EXISTS job_profile_events_setid_format_check;
ALTER TABLE jobcatalog.job_profile_events
  ADD CONSTRAINT job_profile_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_profile_versions
  DROP CONSTRAINT IF EXISTS job_profile_versions_setid_format_check;
ALTER TABLE jobcatalog.job_profile_versions
  ADD CONSTRAINT job_profile_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP CONSTRAINT IF EXISTS job_profile_version_job_families_setid_format_check;
ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD CONSTRAINT job_profile_version_job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$');

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.normalize_setid(p_setid text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'setid is required';
  END IF;
  v := upper(btrim(p_setid));
  IF v !~ '^[A-Z0-9]{5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = format('setid=%s', v);
  END IF;
  RETURN v;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE jobcatalog.job_family_groups
  DROP CONSTRAINT IF EXISTS job_family_groups_setid_format_check;
ALTER TABLE jobcatalog.job_family_groups
  ADD CONSTRAINT job_family_groups_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_family_group_events
  DROP CONSTRAINT IF EXISTS job_family_group_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_events
  ADD CONSTRAINT job_family_group_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_versions
  ADD CONSTRAINT job_family_group_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_families
  DROP CONSTRAINT IF EXISTS job_families_setid_format_check;
ALTER TABLE jobcatalog.job_families
  ADD CONSTRAINT job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_family_events
  DROP CONSTRAINT IF EXISTS job_family_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_events
  ADD CONSTRAINT job_family_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_versions
  ADD CONSTRAINT job_family_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_levels
  DROP CONSTRAINT IF EXISTS job_levels_setid_format_check;
ALTER TABLE jobcatalog.job_levels
  ADD CONSTRAINT job_levels_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_level_events
  DROP CONSTRAINT IF EXISTS job_level_events_setid_format_check;
ALTER TABLE jobcatalog.job_level_events
  ADD CONSTRAINT job_level_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_level_versions
  DROP CONSTRAINT IF EXISTS job_level_versions_setid_format_check;
ALTER TABLE jobcatalog.job_level_versions
  ADD CONSTRAINT job_level_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_profiles
  DROP CONSTRAINT IF EXISTS job_profiles_setid_format_check;
ALTER TABLE jobcatalog.job_profiles
  ADD CONSTRAINT job_profiles_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_profile_events
  DROP CONSTRAINT IF EXISTS job_profile_events_setid_format_check;
ALTER TABLE jobcatalog.job_profile_events
  ADD CONSTRAINT job_profile_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_profile_versions
  DROP CONSTRAINT IF EXISTS job_profile_versions_setid_format_check;
ALTER TABLE jobcatalog.job_profile_versions
  ADD CONSTRAINT job_profile_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP CONSTRAINT IF EXISTS job_profile_version_job_families_setid_format_check;
ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD CONSTRAINT job_profile_version_job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$');

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.normalize_setid(p_setid text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'setid is required';
  END IF;
  v := upper(btrim(p_setid));
  IF v !~ '^[A-Z0-9]{1,5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = format('setid=%s', v);
  END IF;
  RETURN v;
END;
$$;
-- +goose StatementEnd
