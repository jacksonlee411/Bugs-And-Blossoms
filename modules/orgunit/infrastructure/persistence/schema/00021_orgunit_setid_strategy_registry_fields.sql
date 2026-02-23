ALTER TABLE orgunit.setid_strategy_registry
  ADD COLUMN IF NOT EXISTS maintainable boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS allowed_value_codes jsonb NULL;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_allowed_value_codes_array_check,
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_default_value_in_allowed_check;

ALTER TABLE orgunit.setid_strategy_registry
  ADD CONSTRAINT setid_strategy_registry_allowed_value_codes_array_check CHECK (
    allowed_value_codes IS NULL OR jsonb_typeof(allowed_value_codes) = 'array'
  ),
  ADD CONSTRAINT setid_strategy_registry_default_value_in_allowed_check CHECK (
    allowed_value_codes IS NULL
    OR default_value IS NULL
    OR btrim(default_value) = ''
    OR allowed_value_codes ? btrim(default_value)
  );
