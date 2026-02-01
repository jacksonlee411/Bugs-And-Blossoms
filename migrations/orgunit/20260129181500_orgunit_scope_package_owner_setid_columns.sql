-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.setid_scope_packages
  ADD COLUMN IF NOT EXISTS owner_setid text;

ALTER TABLE orgunit.setid_scope_package_versions
  ADD COLUMN IF NOT EXISTS owner_setid text;
-- +goose StatementEnd
