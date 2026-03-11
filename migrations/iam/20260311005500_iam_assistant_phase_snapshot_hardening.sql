-- +goose Up
-- +goose StatementBegin
UPDATE iam.assistant_state_transitions
SET from_phase = CASE
  WHEN from_state = 'init' THEN 'init'
  WHEN from_state = 'committed' THEN 'committed'
  WHEN from_state = 'canceled' THEN 'canceled'
  WHEN from_state = 'expired' THEN 'expired'
  WHEN from_state = 'confirmed' THEN 'await_commit_confirm'
  ELSE 'idle'
END
WHERE from_phase IS NULL OR btrim(from_phase) = '';

UPDATE iam.assistant_state_transitions
SET to_phase = CASE
  WHEN reason_code = 'conversation_created' THEN 'idle'
  WHEN reason_code = 'turn_created' THEN 'await_commit_confirm'
  WHEN reason_code = 'committed' THEN 'committed'
  WHEN to_state = 'committed' THEN 'committed'
  WHEN to_state = 'canceled' THEN 'canceled'
  WHEN to_state = 'expired' THEN 'expired'
  WHEN to_state = 'confirmed' THEN 'await_commit_confirm'
  ELSE 'await_commit_confirm'
END
WHERE to_phase IS NULL OR btrim(to_phase) = '';

ALTER TABLE iam.assistant_state_transitions
  ALTER COLUMN from_phase SET NOT NULL;
ALTER TABLE iam.assistant_state_transitions
  ALTER COLUMN to_phase SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_turns_candidate_options_array_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_candidate_options_array_check
      CHECK (jsonb_typeof(candidate_options) = 'array');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_turns_missing_fields_array_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_missing_fields_array_check
      CHECK (jsonb_typeof(missing_fields) = 'array');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_turns_commit_reply_object_or_null_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_commit_reply_object_or_null_check
      CHECK (commit_reply IS NULL OR jsonb_typeof(commit_reply) = 'object');
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE iam.assistant_turns DROP CONSTRAINT IF EXISTS assistant_turns_commit_reply_object_or_null_check;
ALTER TABLE iam.assistant_turns DROP CONSTRAINT IF EXISTS assistant_turns_missing_fields_array_check;
ALTER TABLE iam.assistant_turns DROP CONSTRAINT IF EXISTS assistant_turns_candidate_options_array_check;
ALTER TABLE iam.assistant_state_transitions ALTER COLUMN to_phase DROP NOT NULL;
ALTER TABLE iam.assistant_state_transitions ALTER COLUMN from_phase DROP NOT NULL;
-- +goose StatementEnd
