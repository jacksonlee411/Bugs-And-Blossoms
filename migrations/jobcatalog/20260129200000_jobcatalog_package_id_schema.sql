-- +goose Up
ALTER TABLE jobcatalog.job_family_groups
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_family_group_events
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_family_group_versions
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_families
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_family_events
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_family_versions
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_levels
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_level_events
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_level_versions
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_profiles
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_profile_events
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_profile_versions
  ADD COLUMN package_uuid uuid;
ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD COLUMN package_uuid uuid;


ALTER TABLE jobcatalog.job_family_groups
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_family_groups
  DROP CONSTRAINT IF EXISTS job_family_groups_setid_format_check;
ALTER TABLE jobcatalog.job_family_groups
  DROP CONSTRAINT IF EXISTS job_family_groups_tenant_setid_code_key;
ALTER TABLE jobcatalog.job_family_groups
  DROP CONSTRAINT IF EXISTS job_family_groups_tenant_setid_id_unique CASCADE;
ALTER TABLE jobcatalog.job_family_groups
  ADD CONSTRAINT job_family_groups_tenant_pkg_code_key UNIQUE (tenant_uuid, package_uuid, job_family_group_code);
ALTER TABLE jobcatalog.job_family_groups
  ADD CONSTRAINT job_family_groups_tenant_pkg_id_unique UNIQUE (tenant_uuid, package_uuid, job_family_group_uuid);

ALTER TABLE jobcatalog.job_family_group_events
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_family_group_events
  DROP CONSTRAINT IF EXISTS job_family_group_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_events
  DROP CONSTRAINT IF EXISTS job_family_group_events_one_per_day_unique;
ALTER TABLE jobcatalog.job_family_group_events
  DROP CONSTRAINT IF EXISTS job_family_group_events_group_fk;
ALTER TABLE jobcatalog.job_family_group_events
  ADD CONSTRAINT job_family_group_events_one_per_day_unique UNIQUE (tenant_uuid, package_uuid, job_family_group_uuid, effective_date);
ALTER TABLE jobcatalog.job_family_group_events
  ADD CONSTRAINT job_family_group_events_group_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_group_uuid)
    REFERENCES jobcatalog.job_family_groups(tenant_uuid, package_uuid, job_family_group_uuid) ON DELETE RESTRICT;

DROP INDEX IF EXISTS jobcatalog.job_family_group_events_tenant_effective_idx;
CREATE INDEX IF NOT EXISTS job_family_group_events_tenant_effective_idx
  ON jobcatalog.job_family_group_events (tenant_uuid, package_uuid, job_family_group_uuid, effective_date, id);

ALTER TABLE jobcatalog.job_family_group_versions
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_job_family_group_uuid_fkey;
ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_group_fk;
ALTER TABLE jobcatalog.job_family_group_versions
  DROP CONSTRAINT IF EXISTS job_family_group_versions_no_overlap;
ALTER TABLE jobcatalog.job_family_group_versions
  ADD CONSTRAINT job_family_group_versions_group_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_group_uuid)
    REFERENCES jobcatalog.job_family_groups(tenant_uuid, package_uuid, job_family_group_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_family_group_versions
  ADD CONSTRAINT job_family_group_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      package_uuid gist_uuid_ops WITH =,
      job_family_group_uuid gist_uuid_ops WITH =,
      validity WITH &&
    );

DROP INDEX IF EXISTS jobcatalog.job_family_group_versions_active_day_gist;
CREATE INDEX IF NOT EXISTS job_family_group_versions_active_day_gist
  ON jobcatalog.job_family_group_versions
  USING gist (tenant_uuid gist_uuid_ops, package_uuid gist_uuid_ops, validity)
  WHERE is_active = true;

DROP INDEX IF EXISTS jobcatalog.job_family_group_versions_lookup_btree;
CREATE INDEX IF NOT EXISTS job_family_group_versions_lookup_btree
  ON jobcatalog.job_family_group_versions (tenant_uuid, package_uuid, job_family_group_uuid, lower(validity));

ALTER TABLE jobcatalog.job_families
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_families
  DROP CONSTRAINT IF EXISTS job_families_setid_format_check;
ALTER TABLE jobcatalog.job_families
  DROP CONSTRAINT IF EXISTS job_families_tenant_setid_code_key;
ALTER TABLE jobcatalog.job_families
  DROP CONSTRAINT IF EXISTS job_families_tenant_setid_id_unique CASCADE;
ALTER TABLE jobcatalog.job_families
  ADD CONSTRAINT job_families_tenant_pkg_code_key UNIQUE (tenant_uuid, package_uuid, job_family_code);
ALTER TABLE jobcatalog.job_families
  ADD CONSTRAINT job_families_tenant_pkg_id_unique UNIQUE (tenant_uuid, package_uuid, job_family_uuid);

ALTER TABLE jobcatalog.job_family_events
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_family_events
  DROP CONSTRAINT IF EXISTS job_family_events_setid_format_check;
ALTER TABLE jobcatalog.job_family_events
  DROP CONSTRAINT IF EXISTS job_family_events_one_per_day_unique;
ALTER TABLE jobcatalog.job_family_events
  DROP CONSTRAINT IF EXISTS job_family_events_family_fk;
ALTER TABLE jobcatalog.job_family_events
  ADD CONSTRAINT job_family_events_one_per_day_unique UNIQUE (tenant_uuid, package_uuid, job_family_uuid, effective_date);
ALTER TABLE jobcatalog.job_family_events
  ADD CONSTRAINT job_family_events_family_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_uuid)
    REFERENCES jobcatalog.job_families(tenant_uuid, package_uuid, job_family_uuid) ON DELETE RESTRICT;

DROP INDEX IF EXISTS jobcatalog.job_family_events_tenant_effective_idx;
CREATE INDEX IF NOT EXISTS job_family_events_tenant_effective_idx
  ON jobcatalog.job_family_events (tenant_uuid, package_uuid, job_family_uuid, effective_date, id);

ALTER TABLE jobcatalog.job_family_versions
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_setid_format_check;
ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_family_fk;
ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_group_fk;
ALTER TABLE jobcatalog.job_family_versions
  DROP CONSTRAINT IF EXISTS job_family_versions_no_overlap;
ALTER TABLE jobcatalog.job_family_versions
  ADD CONSTRAINT job_family_versions_family_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_uuid)
    REFERENCES jobcatalog.job_families(tenant_uuid, package_uuid, job_family_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_family_versions
  ADD CONSTRAINT job_family_versions_group_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_group_uuid)
    REFERENCES jobcatalog.job_family_groups(tenant_uuid, package_uuid, job_family_group_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_family_versions
  ADD CONSTRAINT job_family_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      package_uuid gist_uuid_ops WITH =,
      job_family_uuid gist_uuid_ops WITH =,
      validity WITH &&
    );

DROP INDEX IF EXISTS jobcatalog.job_family_versions_active_day_gist;
CREATE INDEX IF NOT EXISTS job_family_versions_active_day_gist
  ON jobcatalog.job_family_versions
  USING gist (tenant_uuid gist_uuid_ops, package_uuid gist_uuid_ops, validity)
  WHERE is_active = true;

DROP INDEX IF EXISTS jobcatalog.job_family_versions_lookup_btree;
CREATE INDEX IF NOT EXISTS job_family_versions_lookup_btree
  ON jobcatalog.job_family_versions (tenant_uuid, package_uuid, job_family_uuid, lower(validity));

ALTER TABLE jobcatalog.job_levels
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_levels
  DROP CONSTRAINT IF EXISTS job_levels_setid_format_check;
ALTER TABLE jobcatalog.job_levels
  DROP CONSTRAINT IF EXISTS job_levels_tenant_setid_code_key;
ALTER TABLE jobcatalog.job_levels
  DROP CONSTRAINT IF EXISTS job_levels_tenant_setid_id_unique CASCADE;
ALTER TABLE jobcatalog.job_levels
  ADD CONSTRAINT job_levels_tenant_pkg_code_key UNIQUE (tenant_uuid, package_uuid, job_level_code);
ALTER TABLE jobcatalog.job_levels
  ADD CONSTRAINT job_levels_tenant_pkg_id_unique UNIQUE (tenant_uuid, package_uuid, job_level_uuid);

ALTER TABLE jobcatalog.job_level_events
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_level_events
  DROP CONSTRAINT IF EXISTS job_level_events_setid_format_check;
ALTER TABLE jobcatalog.job_level_events
  DROP CONSTRAINT IF EXISTS job_level_events_one_per_day_unique;
ALTER TABLE jobcatalog.job_level_events
  DROP CONSTRAINT IF EXISTS job_level_events_level_fk;
ALTER TABLE jobcatalog.job_level_events
  ADD CONSTRAINT job_level_events_one_per_day_unique UNIQUE (tenant_uuid, package_uuid, job_level_uuid, effective_date);
ALTER TABLE jobcatalog.job_level_events
  ADD CONSTRAINT job_level_events_level_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_level_uuid)
    REFERENCES jobcatalog.job_levels(tenant_uuid, package_uuid, job_level_uuid) ON DELETE RESTRICT;

DROP INDEX IF EXISTS jobcatalog.job_level_events_tenant_effective_idx;
CREATE INDEX IF NOT EXISTS job_level_events_tenant_effective_idx
  ON jobcatalog.job_level_events (tenant_uuid, package_uuid, job_level_uuid, effective_date, id);

ALTER TABLE jobcatalog.job_level_versions
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_level_versions
  DROP CONSTRAINT IF EXISTS job_level_versions_setid_format_check;
ALTER TABLE jobcatalog.job_level_versions
  DROP CONSTRAINT IF EXISTS job_level_versions_level_fk;
ALTER TABLE jobcatalog.job_level_versions
  DROP CONSTRAINT IF EXISTS job_level_versions_no_overlap;
ALTER TABLE jobcatalog.job_level_versions
  ADD CONSTRAINT job_level_versions_level_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_level_uuid)
    REFERENCES jobcatalog.job_levels(tenant_uuid, package_uuid, job_level_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_level_versions
  ADD CONSTRAINT job_level_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      package_uuid gist_uuid_ops WITH =,
      job_level_uuid gist_uuid_ops WITH =,
      validity WITH &&
    );

DROP INDEX IF EXISTS jobcatalog.job_level_versions_active_day_gist;
CREATE INDEX IF NOT EXISTS job_level_versions_active_day_gist
  ON jobcatalog.job_level_versions
  USING gist (tenant_uuid gist_uuid_ops, package_uuid gist_uuid_ops, validity)
  WHERE is_active = true;

DROP INDEX IF EXISTS jobcatalog.job_level_versions_lookup_btree;
CREATE INDEX IF NOT EXISTS job_level_versions_lookup_btree
  ON jobcatalog.job_level_versions (tenant_uuid, package_uuid, job_level_uuid, lower(validity));

ALTER TABLE jobcatalog.job_profiles
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_profiles
  DROP CONSTRAINT IF EXISTS job_profiles_setid_format_check;
ALTER TABLE jobcatalog.job_profiles
  DROP CONSTRAINT IF EXISTS job_profiles_tenant_setid_code_key;
ALTER TABLE jobcatalog.job_profiles
  DROP CONSTRAINT IF EXISTS job_profiles_tenant_setid_id_unique CASCADE;
ALTER TABLE jobcatalog.job_profiles
  ADD CONSTRAINT job_profiles_tenant_pkg_code_key UNIQUE (tenant_uuid, package_uuid, job_profile_code);
ALTER TABLE jobcatalog.job_profiles
  ADD CONSTRAINT job_profiles_tenant_pkg_id_unique UNIQUE (tenant_uuid, package_uuid, job_profile_uuid);

ALTER TABLE jobcatalog.job_profile_events
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_profile_events
  DROP CONSTRAINT IF EXISTS job_profile_events_setid_format_check;
ALTER TABLE jobcatalog.job_profile_events
  DROP CONSTRAINT IF EXISTS job_profile_events_one_per_day_unique;
ALTER TABLE jobcatalog.job_profile_events
  DROP CONSTRAINT IF EXISTS job_profile_events_profile_fk;
ALTER TABLE jobcatalog.job_profile_events
  ADD CONSTRAINT job_profile_events_one_per_day_unique UNIQUE (tenant_uuid, package_uuid, job_profile_uuid, effective_date);
ALTER TABLE jobcatalog.job_profile_events
  ADD CONSTRAINT job_profile_events_profile_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_profile_uuid)
    REFERENCES jobcatalog.job_profiles(tenant_uuid, package_uuid, job_profile_uuid) ON DELETE RESTRICT;

DROP INDEX IF EXISTS jobcatalog.job_profile_events_tenant_effective_idx;
CREATE INDEX IF NOT EXISTS job_profile_events_tenant_effective_idx
  ON jobcatalog.job_profile_events (tenant_uuid, package_uuid, job_profile_uuid, effective_date, id);

ALTER TABLE jobcatalog.job_profile_versions
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_profile_versions
  DROP CONSTRAINT IF EXISTS job_profile_versions_setid_format_check;
ALTER TABLE jobcatalog.job_profile_versions
  DROP CONSTRAINT IF EXISTS job_profile_versions_profile_fk;
ALTER TABLE jobcatalog.job_profile_versions
  DROP CONSTRAINT IF EXISTS job_profile_versions_no_overlap;
ALTER TABLE jobcatalog.job_profile_versions
  ADD CONSTRAINT job_profile_versions_profile_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_profile_uuid)
    REFERENCES jobcatalog.job_profiles(tenant_uuid, package_uuid, job_profile_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_profile_versions
  ADD CONSTRAINT job_profile_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      package_uuid gist_uuid_ops WITH =,
      job_profile_uuid gist_uuid_ops WITH =,
      validity WITH &&
    );

DROP INDEX IF EXISTS jobcatalog.job_profile_versions_active_day_gist;
CREATE INDEX IF NOT EXISTS job_profile_versions_active_day_gist
  ON jobcatalog.job_profile_versions
  USING gist (tenant_uuid gist_uuid_ops, package_uuid gist_uuid_ops, validity)
  WHERE is_active = true;

DROP INDEX IF EXISTS jobcatalog.job_profile_versions_lookup_btree;
CREATE INDEX IF NOT EXISTS job_profile_versions_lookup_btree
  ON jobcatalog.job_profile_versions (tenant_uuid, package_uuid, job_profile_uuid, lower(validity));

ALTER TABLE jobcatalog.job_profile_version_job_families
  ALTER COLUMN setid DROP NOT NULL;
ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP CONSTRAINT IF EXISTS job_profile_version_job_families_setid_format_check;
ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP CONSTRAINT IF EXISTS job_profile_version_job_families_family_fk;
ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP CONSTRAINT IF EXISTS job_profile_version_job_families_unique;
ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD CONSTRAINT job_profile_version_job_families_family_fk
    FOREIGN KEY (tenant_uuid, package_uuid, job_family_uuid)
    REFERENCES jobcatalog.job_families(tenant_uuid, package_uuid, job_family_uuid) ON DELETE RESTRICT;
ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD CONSTRAINT job_profile_version_job_families_unique
    UNIQUE (tenant_uuid, package_uuid, job_profile_version_id, job_family_uuid);

DROP INDEX IF EXISTS jobcatalog.job_profile_version_job_families_one_primary_unique;
CREATE UNIQUE INDEX IF NOT EXISTS job_profile_version_job_families_one_primary_unique
  ON jobcatalog.job_profile_version_job_families (tenant_uuid, package_uuid, job_profile_version_id)
  WHERE is_primary = true;

DROP INDEX IF EXISTS jobcatalog.job_profile_version_job_families_family_lookup_btree;
CREATE INDEX IF NOT EXISTS job_profile_version_job_families_family_lookup_btree
  ON jobcatalog.job_profile_version_job_families (tenant_uuid, package_uuid, job_family_uuid);

-- +goose Down
ALTER TABLE jobcatalog.job_profile_version_job_families
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_profile_versions
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_profile_events
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_profiles
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_level_versions
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_level_events
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_levels
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_family_versions
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_family_events
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_families
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_family_group_versions
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_family_group_events
  DROP COLUMN IF EXISTS package_uuid;
ALTER TABLE jobcatalog.job_family_groups
  DROP COLUMN IF EXISTS package_uuid;
