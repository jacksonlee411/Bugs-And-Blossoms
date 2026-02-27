ALTER TABLE orgunit.setid_strategy_registry
  ADD COLUMN IF NOT EXISTS priority_mode text NOT NULL DEFAULT 'blend_custom_first',
  ADD COLUMN IF NOT EXISTS local_override_mode text NOT NULL DEFAULT 'allow';

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_priority_mode_check,
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_local_override_mode_check,
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_mode_combination_check;

ALTER TABLE orgunit.setid_strategy_registry
  ADD CONSTRAINT setid_strategy_registry_priority_mode_check CHECK (
    priority_mode IN ('blend_custom_first', 'blend_deflt_first', 'deflt_unsubscribed')
  ),
  ADD CONSTRAINT setid_strategy_registry_local_override_mode_check CHECK (
    local_override_mode IN ('allow', 'no_override', 'no_local')
  ),
  ADD CONSTRAINT setid_strategy_registry_mode_combination_check CHECK (
    NOT (priority_mode = 'deflt_unsubscribed' AND local_override_mode = 'no_local')
  );
