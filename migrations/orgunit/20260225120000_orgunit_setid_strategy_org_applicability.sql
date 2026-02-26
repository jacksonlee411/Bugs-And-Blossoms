-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.setid_strategy_registry
  RENAME COLUMN org_level TO org_applicability;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME CONSTRAINT setid_strategy_registry_org_level_check
  TO setid_strategy_registry_org_applicability_check;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME CONSTRAINT setid_strategy_registry_business_unit_check
  TO setid_strategy_registry_business_unit_applicability_check;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orgunit.setid_strategy_registry
  RENAME CONSTRAINT setid_strategy_registry_org_applicability_check
  TO setid_strategy_registry_org_level_check;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME CONSTRAINT setid_strategy_registry_business_unit_applicability_check
  TO setid_strategy_registry_business_unit_check;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME COLUMN org_applicability TO org_level;
-- +goose StatementEnd
