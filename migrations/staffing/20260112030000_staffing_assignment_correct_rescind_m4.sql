-- +goose Up
-- create "assignment_event_corrections" table
CREATE TABLE "staffing"."assignment_event_corrections" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "target_effective_date" date NOT NULL,
  "replacement_payload" jsonb NOT NULL,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_event_corrections_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "assignment_event_corrections_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "assignment_event_corrections_target_unique" UNIQUE ("tenant_id", "assignment_id", "target_effective_date"),
  CONSTRAINT "assignment_event_corrections_replacement_payload_obj_check" CHECK (jsonb_typeof(replacement_payload) = 'object'::text)
);
-- create "assignment_event_rescinds" table
CREATE TABLE "staffing"."assignment_event_rescinds" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "target_effective_date" date NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "assignment_event_rescinds_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "assignment_event_rescinds_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "assignment_event_rescinds_target_unique" UNIQUE ("tenant_id", "assignment_id", "target_effective_date"),
  CONSTRAINT "assignment_event_rescinds_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text)
);

-- +goose Down
-- reverse: create "assignment_event_rescinds" table
DROP TABLE "staffing"."assignment_event_rescinds";
-- reverse: create "assignment_event_corrections" table
DROP TABLE "staffing"."assignment_event_corrections";
