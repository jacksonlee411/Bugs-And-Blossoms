-- +goose Up
-- create "iit_special_additional_deduction_claim_events" table
CREATE TABLE "staffing"."iit_special_additional_deduction_claim_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "tax_year" integer NOT NULL,
  "tax_month" smallint NOT NULL,
  "amount" numeric(15,2) NOT NULL,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "iit_sad_claim_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "iit_sad_claim_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "iit_sad_claim_events_amount_check" CHECK (amount >= (0)::numeric),
  CONSTRAINT "iit_sad_claim_events_tax_month_check" CHECK ((tax_month >= 1) AND (tax_month <= 12)),
  CONSTRAINT "iit_sad_claim_events_tax_year_check" CHECK ((tax_year >= 2000) AND (tax_year <= 9999))
);
-- create index "iit_sad_claim_events_lookup_btree" to table: "iit_special_additional_deduction_claim_events"
CREATE INDEX "iit_sad_claim_events_lookup_btree" ON "staffing"."iit_special_additional_deduction_claim_events" ("tenant_id", "person_uuid", "tax_year", "tax_month", "id");
-- create "iit_special_additional_deduction_claims" table
CREATE TABLE "staffing"."iit_special_additional_deduction_claims" (
  "tenant_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "tax_year" integer NOT NULL,
  "tax_month" smallint NOT NULL,
  "amount" numeric(15,2) NOT NULL DEFAULT 0,
  "last_event_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "person_uuid", "tax_year", "tax_month"),
  CONSTRAINT "iit_special_additional_deduction_claims_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."iit_special_additional_deduction_claim_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "iit_sad_claims_amount_check" CHECK (amount >= (0)::numeric),
  CONSTRAINT "iit_sad_claims_tax_month_check" CHECK ((tax_month >= 1) AND (tax_month <= 12)),
  CONSTRAINT "iit_sad_claims_tax_year_check" CHECK ((tax_year >= 2000) AND (tax_year <= 9999))
);

-- +goose Down
-- reverse: create "iit_special_additional_deduction_claims" table
DROP TABLE "staffing"."iit_special_additional_deduction_claims";
-- reverse: create index "iit_sad_claim_events_lookup_btree" to table: "iit_special_additional_deduction_claim_events"
DROP INDEX "staffing"."iit_sad_claim_events_lookup_btree";
-- reverse: create "iit_special_additional_deduction_claim_events" table
DROP TABLE "staffing"."iit_special_additional_deduction_claim_events";
