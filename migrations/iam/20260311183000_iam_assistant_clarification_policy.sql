-- +goose Up
-- +goose StatementBegin
ALTER TABLE iam.assistant_turns
  ADD COLUMN IF NOT EXISTS clarification_json jsonb NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'assistant_turns_clarification_object_check'
      AND conrelid = 'iam.assistant_turns'::regclass
  ) THEN
    ALTER TABLE iam.assistant_turns
      ADD CONSTRAINT assistant_turns_clarification_object_check
      CHECK (jsonb_typeof(clarification_json) = 'object');
  END IF;
END $$;

ALTER TABLE iam.assistant_conversations
  DROP CONSTRAINT IF EXISTS assistant_conversations_current_phase_check;
ALTER TABLE iam.assistant_conversations
  ADD CONSTRAINT assistant_conversations_current_phase_check
  CHECK (
    current_phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_turns
  DROP CONSTRAINT IF EXISTS assistant_turns_phase_check;
ALTER TABLE iam.assistant_turns
  ADD CONSTRAINT assistant_turns_phase_check
  CHECK (
    phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_state_transitions
  DROP CONSTRAINT IF EXISTS assistant_state_transitions_from_phase_check;
ALTER TABLE iam.assistant_state_transitions
  ADD CONSTRAINT assistant_state_transitions_from_phase_check
  CHECK (
    from_phase IN (
      'init',
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_state_transitions
  DROP CONSTRAINT IF EXISTS assistant_state_transitions_to_phase_check;
ALTER TABLE iam.assistant_state_transitions
  ADD CONSTRAINT assistant_state_transitions_to_phase_check
  CHECK (
    to_phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE iam.assistant_state_transitions
  DROP CONSTRAINT IF EXISTS assistant_state_transitions_to_phase_check;
ALTER TABLE iam.assistant_state_transitions
  ADD CONSTRAINT assistant_state_transitions_to_phase_check
  CHECK (
    to_phase IN (
      'idle',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_state_transitions
  DROP CONSTRAINT IF EXISTS assistant_state_transitions_from_phase_check;
ALTER TABLE iam.assistant_state_transitions
  ADD CONSTRAINT assistant_state_transitions_from_phase_check
  CHECK (
    from_phase IN (
      'init',
      'idle',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_turns
  DROP CONSTRAINT IF EXISTS assistant_turns_phase_check;
ALTER TABLE iam.assistant_turns
  ADD CONSTRAINT assistant_turns_phase_check
  CHECK (
    phase IN (
      'idle',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_conversations
  DROP CONSTRAINT IF EXISTS assistant_conversations_current_phase_check;
ALTER TABLE iam.assistant_conversations
  ADD CONSTRAINT assistant_conversations_current_phase_check
  CHECK (
    current_phase IN (
      'idle',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  );

ALTER TABLE iam.assistant_turns
  DROP CONSTRAINT IF EXISTS assistant_turns_clarification_object_check;
ALTER TABLE iam.assistant_turns
  DROP COLUMN IF EXISTS clarification_json;
-- +goose StatementEnd
