-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.setid_scope_packages
  ADD COLUMN IF NOT EXISTS owner_setid text;

ALTER TABLE orgunit.setid_scope_packages
  ADD CONSTRAINT setid_scope_packages_owner_format_check
    CHECK (owner_setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE orgunit.setid_scope_packages
  ADD CONSTRAINT setid_scope_packages_owner_fk
    FOREIGN KEY (tenant_uuid, owner_setid)
    REFERENCES orgunit.setids (tenant_uuid, setid);

CREATE INDEX IF NOT EXISTS setid_scope_packages_owner_lookup_idx
  ON orgunit.setid_scope_packages (tenant_uuid, scope_code, owner_setid, status);

ALTER TABLE orgunit.setid_scope_package_versions
  ADD COLUMN IF NOT EXISTS owner_setid text;

ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_format_check
    CHECK (owner_setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_fk
    FOREIGN KEY (tenant_uuid, owner_setid)
    REFERENCES orgunit.setids (tenant_uuid, setid);

CREATE INDEX IF NOT EXISTS setid_scope_package_versions_owner_lookup_idx
  ON orgunit.setid_scope_package_versions (tenant_uuid, scope_code, owner_setid, lower(validity));

ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_scope_no_overlap
  EXCLUDE USING gist (
    tenant_uuid WITH =,
    scope_code gist_text_ops WITH =,
    owner_setid gist_text_ops WITH =,
    validity WITH &&
  )
  WHERE (status = 'active');
-- +goose StatementEnd

