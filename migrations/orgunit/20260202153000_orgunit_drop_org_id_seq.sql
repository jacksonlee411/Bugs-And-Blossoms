-- +goose Up
-- +goose StatementBegin
DROP SEQUENCE IF EXISTS orgunit.org_id_seq;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS orgunit.org_id_seq
  START WITH 10000000
  INCREMENT BY 1
  MINVALUE 10000000
  MAXVALUE 99999999
  NO CYCLE;
-- +goose StatementEnd
