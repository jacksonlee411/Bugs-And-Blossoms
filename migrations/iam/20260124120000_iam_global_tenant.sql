-- +goose Up
-- +goose StatementBegin
INSERT INTO iam.tenants (id, name, is_active)
VALUES ('00000000-0000-0000-0000-000000000000', 'GLOBAL', true)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.tenants WHERE id = '00000000-0000-0000-0000-000000000000'::uuid;
-- +goose StatementEnd
