-- +goose Up
-- +goose StatementBegin
ALTER TABLE iam.assistant_turns
  ADD COLUMN IF NOT EXISTS route_decision_json jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'assistant_turns_route_decision_object_or_null_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_route_decision_object_or_null_check
      CHECK (
        route_decision_json IS NULL OR jsonb_typeof(route_decision_json) = 'object'
      );
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE iam.assistant_turns
  DROP CONSTRAINT IF EXISTS assistant_turns_route_decision_object_or_null_check;
ALTER TABLE iam.assistant_turns
  DROP COLUMN IF EXISTS route_decision_json;
-- +goose StatementEnd
