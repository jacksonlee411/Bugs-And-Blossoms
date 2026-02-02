-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS orgunit.org_id_allocators OWNER TO orgunit_kernel;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE orgunit.org_id_allocators TO orgunit_kernel;

ALTER FUNCTION orgunit.allocate_org_id(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SECURITY DEFINER;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER FUNCTION orgunit.allocate_org_id(uuid) SECURITY INVOKER;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SET search_path = pg_catalog, public;
REVOKE ALL ON TABLE orgunit.org_id_allocators FROM orgunit_kernel;
-- +goose StatementEnd
