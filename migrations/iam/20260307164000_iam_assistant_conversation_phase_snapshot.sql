-- +goose Up
-- +goose StatementBegin
ALTER TABLE iam.assistant_conversations
  ADD COLUMN IF NOT EXISTS current_phase text;

UPDATE iam.assistant_conversations
SET current_phase = CASE
  WHEN state = 'committed' THEN 'committed'
  WHEN state = 'canceled' THEN 'canceled'
  WHEN state = 'expired' THEN 'expired'
  ELSE 'idle'
END
WHERE current_phase IS NULL OR btrim(current_phase) = '';

ALTER TABLE iam.assistant_conversations
  ALTER COLUMN current_phase SET DEFAULT 'idle';
ALTER TABLE iam.assistant_conversations
  ALTER COLUMN current_phase SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_conversations_current_phase_check'
      AND conrelid = 'iam.assistant_conversations'::regclass
  ) THEN
    ALTER TABLE iam.assistant_conversations
      ADD CONSTRAINT assistant_conversations_current_phase_check
      CHECK (current_phase IN ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'));
  END IF;
END $$;

ALTER TABLE iam.assistant_turns
  ADD COLUMN IF NOT EXISTS phase text,
  ADD COLUMN IF NOT EXISTS pending_draft_summary text,
  ADD COLUMN IF NOT EXISTS missing_fields jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS candidate_options jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS selected_candidate_id text,
  ADD COLUMN IF NOT EXISTS commit_reply jsonb,
  ADD COLUMN IF NOT EXISTS error_code text;

UPDATE iam.assistant_turns
SET phase = CASE
  WHEN state = 'committed' THEN 'committed'
  WHEN state = 'canceled' THEN 'canceled'
  WHEN state = 'expired' THEN 'expired'
  WHEN state = 'confirmed' THEN 'await_commit_confirm'
  WHEN EXISTS (
    SELECT 1
    FROM jsonb_array_elements_text(COALESCE(dry_run_json->'validation_errors', '[]'::jsonb)) AS item(code)
    WHERE code IN ('missing_parent_ref_text', 'parent_candidate_not_found', 'missing_entity_name', 'missing_effective_date', 'invalid_effective_date_format')
  ) THEN 'await_missing_fields'
  WHEN EXISTS (
    SELECT 1
    FROM jsonb_array_elements_text(COALESCE(dry_run_json->'validation_errors', '[]'::jsonb)) AS item(code)
    WHERE code = 'candidate_confirmation_required'
  ) AND COALESCE(btrim(resolved_candidate_id), '') = '' THEN 'await_candidate_pick'
  ELSE 'await_commit_confirm'
END
WHERE phase IS NULL OR btrim(phase) = '';

UPDATE iam.assistant_turns
SET candidate_options = candidates_json
WHERE candidate_options = '[]'::jsonb;

UPDATE iam.assistant_turns
SET selected_candidate_id = resolved_candidate_id
WHERE selected_candidate_id IS NULL AND NULLIF(btrim(resolved_candidate_id), '') IS NOT NULL;

ALTER TABLE iam.assistant_turns
  ALTER COLUMN phase SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_turns_phase_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_phase_check
      CHECK (phase IN ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'));
  END IF;
END $$;

ALTER TABLE iam.assistant_state_transitions
  ADD COLUMN IF NOT EXISTS from_phase text,
  ADD COLUMN IF NOT EXISTS to_phase text;

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

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_state_transitions_from_phase_check'
      AND conrelid = 'iam.assistant_state_transitions'::regclass
  ) THEN
    ALTER TABLE iam.assistant_state_transitions
      ADD CONSTRAINT assistant_state_transitions_from_phase_check
      CHECK (from_phase IN ('init', 'idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'assistant_state_transitions_to_phase_check'
      AND conrelid = 'iam.assistant_state_transitions'::regclass
  ) THEN
    ALTER TABLE iam.assistant_state_transitions
      ADD CONSTRAINT assistant_state_transitions_to_phase_check
      CHECK (to_phase IN ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'));
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE iam.assistant_state_transitions DROP CONSTRAINT IF EXISTS assistant_state_transitions_to_phase_check;
ALTER TABLE iam.assistant_state_transitions DROP CONSTRAINT IF EXISTS assistant_state_transitions_from_phase_check;
ALTER TABLE iam.assistant_state_transitions DROP COLUMN IF EXISTS to_phase;
ALTER TABLE iam.assistant_state_transitions DROP COLUMN IF EXISTS from_phase;

ALTER TABLE iam.assistant_turns DROP CONSTRAINT IF EXISTS assistant_turns_phase_check;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS error_code;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS commit_reply;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS selected_candidate_id;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS candidate_options;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS missing_fields;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS pending_draft_summary;
ALTER TABLE iam.assistant_turns DROP COLUMN IF EXISTS phase;

ALTER TABLE iam.assistant_conversations DROP CONSTRAINT IF EXISTS assistant_conversations_current_phase_check;
ALTER TABLE iam.assistant_conversations DROP COLUMN IF EXISTS current_phase;
-- +goose StatementEnd
