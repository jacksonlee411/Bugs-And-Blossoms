ALTER TABLE orgunit.setid_scope_packages
  ALTER COLUMN owner_setid SET NOT NULL;

ALTER TABLE orgunit.setid_scope_package_versions
  ALTER COLUMN owner_setid SET NOT NULL;
