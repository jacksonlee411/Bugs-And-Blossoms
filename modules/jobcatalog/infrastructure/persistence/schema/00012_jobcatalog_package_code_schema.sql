ALTER TABLE jobcatalog.job_family_groups
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_family_group_events
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_family_group_versions
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_families
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_family_events
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_family_versions
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_levels
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_level_events
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_level_versions
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_profiles
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_profile_events
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_profile_versions
  ADD COLUMN IF NOT EXISTS package_code text;

ALTER TABLE jobcatalog.job_profile_version_job_families
  ADD COLUMN IF NOT EXISTS package_code text;
