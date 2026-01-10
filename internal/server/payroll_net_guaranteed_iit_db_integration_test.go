package server

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPayrollDB_NetGuaranteedIIT(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	adminConn, adminDSN, ok := connectTestPostgres(ctx, t)
	if !ok {
		return
	}
	t.Cleanup(func() { _ = adminConn.Close(context.Background()) })

	if err := ensurePayrollNetGuaranteedIITSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	runtimeDSN, err := withUserPassword(adminDSN, "bb_test_runtime", "bb_test_runtime")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatalf("connect runtime role: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	t.Run("rls fail-closed", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM staffing.payslip_item_inputs;`).Scan(&n); err == nil {
			t.Fatal("expected RLS fail-closed error when app.current_tenant is missing (payslip_item_inputs)")
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM staffing.payslip_item_input_events;`).Scan(&n); err == nil {
			t.Fatal("expected RLS fail-closed error when app.current_tenant is missing (payslip_item_input_events)")
		}
	})

	t.Run("kernel errors and finalized readonly", func(t *testing.T) {
		const (
			tenantID    = "00000000-0000-0000-0000-0000000000a1"
			payPeriodID = "00000000-0000-0000-0000-0000000000b1"
			runID       = "00000000-0000-0000-0000-0000000000c1"
			payslipID   = "00000000-0000-0000-0000-0000000000d1"
			personUUID  = "00000000-0000-0000-0000-0000000000e1"
			asmtID      = "00000000-0000-0000-0000-0000000000f1"
			initiatorID = "00000000-0000-0000-0000-000000000101"
		)

		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		setup := func(t *testing.T, runState string) pgx.Tx {
			t.Helper()

			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

			if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
				t.Fatal(err)
			}

			var payPeriodEventDBID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO staffing.pay_period_events (
				  tenant_id,
				  pay_period_id,
				  event_type,
				  pay_group,
				  period,
				  request_id,
				  initiator_id,
				  transaction_time,
				  created_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  'CREATE',
				  'monthly',
				  daterange('2026-01-01'::date, '2026-02-01'::date, '[)'),
				  'req-pay-period',
				  $3::uuid,
				  $4::timestamptz,
				  $4::timestamptz
				)
				RETURNING id;
			`, tenantID, payPeriodID, initiatorID, now).Scan(&payPeriodEventDBID); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.pay_periods (tenant_id, id, pay_group, period, status, last_event_id, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, 'monthly', daterange('2026-01-01'::date, '2026-02-01'::date, '[)'), 'open', $3::bigint, $4::timestamptz, $4::timestamptz);
			`, tenantID, payPeriodID, payPeriodEventDBID, now); err != nil {
				t.Fatal(err)
			}

			var runEventDBID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO staffing.payroll_run_events (
				  tenant_id,
				  run_id,
				  pay_period_id,
				  event_type,
				  run_state,
				  payload,
				  request_id,
				  initiator_id,
				  transaction_time,
				  created_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'CREATE',
				  $4::text,
				  '{}'::jsonb,
				  'req-run',
				  $5::uuid,
				  $6::timestamptz,
				  $6::timestamptz
				)
				RETURNING id;
			`, tenantID, runID, payPeriodID, runState, initiatorID, now).Scan(&runEventDBID); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.payroll_runs (
				  tenant_id,
				  id,
				  pay_period_id,
				  run_state,
				  needs_recalc,
				  calc_started_at,
				  calc_finished_at,
				  finalized_at,
				  last_event_id,
				  created_at,
				  updated_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::text,
				  false,
				  NULL,
				  NULL,
				  NULL,
				  $5::bigint,
				  $6::timestamptz,
				  $6::timestamptz
				);
			`, tenantID, runID, payPeriodID, runState, runEventDBID, now); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.payslips (
				  tenant_id,
				  id,
				  run_id,
				  pay_period_id,
				  person_uuid,
				  assignment_id,
				  currency,
				  gross_pay,
				  net_pay,
				  employer_total,
				  last_run_event_id,
				  created_at,
				  updated_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  $5::uuid,
				  $6::uuid,
				  'CNY'::char(3),
				  10000.00,
				  10000.00,
				  0.00,
				  $7::bigint,
				  $8::timestamptz,
				  $8::timestamptz
				);
			`, tenantID, payslipID, runID, payPeriodID, personUUID, asmtID, runEventDBID, now); err != nil {
				t.Fatal(err)
			}

			return tx
		}

		t.Run("amount missing => 422 code", func(t *testing.T) {
			tx := setup(t, "draft")

			var eventDBID int64
			err := tx.QueryRow(ctx, `
				SELECT staffing.submit_payslip_item_input_event(
				  '00000000-0000-0000-0000-000000000111'::uuid,
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'UPSERT',
				  'EARNING_LONG_SERVICE_AWARD',
				  'earning',
				  'CNY'::char(3),
				  'net_guaranteed_iit',
				  'employer',
				  NULL::numeric(15,2),
				  'req-1',
				  $5::uuid
				);
			`, tenantID, runID, personUUID, asmtID, initiatorID).Scan(&eventDBID)
			if err == nil {
				t.Fatal("expected error")
			}
			if msg := pgErrorMessage(err); msg != "STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT" {
				t.Fatalf("pg_message=%q", msg)
			}
		})

		t.Run("currency mismatch => 422 code", func(t *testing.T) {
			tx := setup(t, "draft")

			var eventDBID int64
			err := tx.QueryRow(ctx, `
				SELECT staffing.submit_payslip_item_input_event(
				  '00000000-0000-0000-0000-000000000112'::uuid,
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'UPSERT',
				  'EARNING_LONG_SERVICE_AWARD',
				  'earning',
				  'USD'::char(3),
				  'net_guaranteed_iit',
				  'employer',
				  20000.00,
				  'req-1',
				  $5::uuid
				);
			`, tenantID, runID, personUUID, asmtID, initiatorID).Scan(&eventDBID)
			if err == nil {
				t.Fatal("expected error")
			}
			if msg := pgErrorMessage(err); msg != "STAFFING_PAYROLL_NET_GUARANTEED_IIT_CURRENCY_MISMATCH" {
				t.Fatalf("pg_message=%q", msg)
			}
		})

		t.Run("finalized readonly", func(t *testing.T) {
			tx := setup(t, "finalized")

			var eventDBID int64
			err := tx.QueryRow(ctx, `
				SELECT staffing.submit_payslip_item_input_event(
				  '00000000-0000-0000-0000-000000000113'::uuid,
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'UPSERT',
				  'EARNING_LONG_SERVICE_AWARD',
				  'earning',
				  'CNY'::char(3),
				  'net_guaranteed_iit',
				  'employer',
				  20000.00,
				  'req-1',
				  $5::uuid
				);
			`, tenantID, runID, personUUID, asmtID, initiatorID).Scan(&eventDBID)
			if err == nil {
				t.Fatal("expected error")
			}
			if msg := pgErrorMessage(err); msg != "STAFFING_PAYROLL_RUN_FINALIZED_READONLY" {
				t.Fatalf("pg_message=%q", msg)
			}
		})
	})

	t.Run("solver and allocation: multi-item, sum(iit_delta)=ΔIIT, deterministic", func(t *testing.T) {
		const (
			tenantID    = "00000000-0000-0000-0000-0000000001a1"
			payPeriodID = "00000000-0000-0000-0000-0000000001b1"
			runID       = "00000000-0000-0000-0000-0000000001c1"
			payslipID   = "00000000-0000-0000-0000-0000000001d1"
			personUUID  = "00000000-0000-0000-0000-0000000001e1"
			asmtID      = "00000000-0000-0000-0000-0000000001f1"
			initiatorID = "00000000-0000-0000-0000-000000000201"
		)

		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		type itemOut struct {
			itemCode  string
			amount    string
			targetNet string
			iitDelta  string
		}
		type runOut struct {
			items     []itemOut
			sumDelta  int64
			deltaIIT  int64
			grossDiff int64
		}

		runOnce := func(t *testing.T) runOut {
			t.Helper()

			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()

			if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
				t.Fatal(err)
			}

			var payPeriodEventDBID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO staffing.pay_period_events (
				  tenant_id,
				  pay_period_id,
				  event_type,
				  pay_group,
				  period,
				  request_id,
				  initiator_id,
				  transaction_time,
				  created_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  'CREATE',
				  'monthly',
				  daterange('2026-01-01'::date, '2026-02-01'::date, '[)'),
				  'req-pay-period',
				  $3::uuid,
				  $4::timestamptz,
				  $4::timestamptz
				)
				RETURNING id;
			`, tenantID, payPeriodID, initiatorID, now).Scan(&payPeriodEventDBID); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.pay_periods (tenant_id, id, pay_group, period, status, last_event_id, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, 'monthly', daterange('2026-01-01'::date, '2026-02-01'::date, '[)'), 'open', $3::bigint, $4::timestamptz, $4::timestamptz);
			`, tenantID, payPeriodID, payPeriodEventDBID, now); err != nil {
				t.Fatal(err)
			}

			var runEventDBID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO staffing.payroll_run_events (
				  tenant_id,
				  run_id,
				  pay_period_id,
				  event_type,
				  run_state,
				  payload,
				  request_id,
				  initiator_id,
				  transaction_time,
				  created_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'CREATE',
				  'draft',
				  '{}'::jsonb,
				  'req-run',
				  $4::uuid,
				  $5::timestamptz,
				  $5::timestamptz
				)
				RETURNING id;
			`, tenantID, runID, payPeriodID, initiatorID, now).Scan(&runEventDBID); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.payroll_runs (
				  tenant_id,
				  id,
				  pay_period_id,
				  run_state,
				  needs_recalc,
				  calc_started_at,
				  calc_finished_at,
				  finalized_at,
				  last_event_id,
				  created_at,
				  updated_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'draft',
				  false,
				  NULL,
				  NULL,
				  NULL,
				  $4::bigint,
				  $5::timestamptz,
				  $5::timestamptz
				);
			`, tenantID, runID, payPeriodID, runEventDBID, now); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO staffing.payslips (
				  tenant_id,
				  id,
				  run_id,
				  pay_period_id,
				  person_uuid,
				  assignment_id,
				  currency,
				  gross_pay,
				  net_pay,
				  employer_total,
				  last_run_event_id,
				  created_at,
				  updated_at
				)
				VALUES (
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  $5::uuid,
				  $6::uuid,
				  'CNY'::char(3),
				  10000.00,
				  10000.00,
				  0.00,
				  $7::bigint,
				  $8::timestamptz,
				  $8::timestamptz
				);
			`, tenantID, payslipID, runID, payPeriodID, personUUID, asmtID, runEventDBID, now); err != nil {
				t.Fatal(err)
			}

			for _, in := range []struct {
				eventID string
				code    string
				net     string
			}{
				{"00000000-0000-0000-0000-000000000211", "EARNING_LONG_SERVICE_AWARD", "20000.00"},
				{"00000000-0000-0000-0000-000000000212", "EARNING_SIGN_ON_BONUS", "5000.00"},
			} {
				var eventDBID int64
				if err := tx.QueryRow(ctx, `
					SELECT staffing.submit_payslip_item_input_event(
					  $1::uuid,
					  $2::uuid,
					  $3::uuid,
					  $4::uuid,
					  $5::uuid,
					  'UPSERT',
					  $6::text,
					  'earning',
					  'CNY'::char(3),
					  'net_guaranteed_iit',
					  'employer',
					  $7::numeric(15,2),
					  $8::text,
					  $9::uuid
					);
				`, in.eventID, tenantID, runID, personUUID, asmtID, in.code, in.net, "req-"+in.eventID, initiatorID).Scan(&eventDBID); err != nil {
					t.Fatal(err)
				}
			}

			var baseWithholdCents int64
			if err := tx.QueryRow(ctx, `
				SELECT staffing.iit_withhold_this_month_cents(
				  10000 * 100,
				  0,
				  5000 * 100,
				  0,
				  0,
				  0
				);
			`).Scan(&baseWithholdCents); err != nil {
				t.Fatal(err)
			}

			if _, err := tx.Exec(ctx, `
					SELECT staffing.payroll_apply_iit(
					  $1::uuid,
					  $2::uuid,
					  $3::uuid,
					  $4::bigint,
					  $5::timestamptz
					);
				`, tenantID, runID, payPeriodID, runEventDBID, now); err != nil {
				t.Fatal(err)
			}

			rows, err := tx.Query(ctx, `
				SELECT
				  item_code,
				  amount::text,
				  target_net::text,
				  iit_delta::text
				FROM staffing.payslip_items
				WHERE tenant_id = $1::uuid
				  AND payslip_id = $2::uuid
				  AND calc_mode = 'net_guaranteed_iit'
				ORDER BY item_code ASC;
			`, tenantID, payslipID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()

			var out runOut
			for rows.Next() {
				var it itemOut
				if err := rows.Scan(&it.itemCode, &it.amount, &it.targetNet, &it.iitDelta); err != nil {
					t.Fatal(err)
				}
				out.items = append(out.items, it)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if len(out.items) != 2 {
				t.Fatalf("expected 2 net-guaranteed items, got=%d", len(out.items))
			}

			if err := tx.QueryRow(ctx, `
				SELECT round(COALESCE(sum(iit_delta), 0) * 100, 0)::bigint
				FROM staffing.payslip_items
				WHERE tenant_id = $1::uuid
				  AND payslip_id = $2::uuid
				  AND calc_mode = 'net_guaranteed_iit';
			`, tenantID, payslipID).Scan(&out.sumDelta); err != nil {
				t.Fatal(err)
			}
			if out.sumDelta < 0 {
				t.Fatalf("expected sum(iit_delta) >= 0, got=%d", out.sumDelta)
			}

			var withholdTotalCents int64
			if err := tx.QueryRow(ctx, `
				SELECT round(amount * 100, 0)::bigint
				FROM staffing.payslip_items
				WHERE tenant_id = $1::uuid
				  AND payslip_id = $2::uuid
				  AND item_code = 'DEDUCTION_IIT_WITHHOLDING';
			`, tenantID, payslipID).Scan(&withholdTotalCents); err != nil {
				t.Fatal(err)
			}
			out.deltaIIT = withholdTotalCents - baseWithholdCents
			if out.deltaIIT < 0 {
				t.Fatalf("expected ΔIIT >= 0, got=%d", out.deltaIIT)
			}
			if out.sumDelta != out.deltaIIT {
				t.Fatalf("expected sum(iit_delta_cents) == ΔIIT; sum=%d Δ=%d", out.sumDelta, out.deltaIIT)
			}

			if err := tx.QueryRow(ctx, `
				SELECT round((gross_pay - 10000.00) * 100, 0)::bigint
				FROM staffing.payslips
				WHERE tenant_id = $1::uuid AND id = $2::uuid;
			`, tenantID, payslipID).Scan(&out.grossDiff); err != nil {
				t.Fatal(err)
			}
			if out.grossDiff <= 0 {
				t.Fatalf("expected gross_pay increased, got=%d", out.grossDiff)
			}

			var sumAmounts int64
			if err := tx.QueryRow(ctx, `
				SELECT round(COALESCE(sum(amount), 0) * 100, 0)::bigint
				FROM staffing.payslip_items
				WHERE tenant_id = $1::uuid
				  AND payslip_id = $2::uuid
				  AND calc_mode = 'net_guaranteed_iit';
			`, tenantID, payslipID).Scan(&sumAmounts); err != nil {
				t.Fatal(err)
			}
			if sumAmounts != out.grossDiff {
				t.Fatalf("expected Σ(amount)=gross_pay_increment; sum=%d inc=%d", sumAmounts, out.grossDiff)
			}

			var bad int
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				FROM staffing.payslip_items
				WHERE tenant_id = $1::uuid
				  AND payslip_id = $2::uuid
				  AND calc_mode = 'net_guaranteed_iit'
				  AND amount <> target_net + iit_delta;
			`, tenantID, payslipID).Scan(&bad); err != nil {
				t.Fatal(err)
			}
			if bad != 0 {
				t.Fatalf("expected amount = target_net + iit_delta for all items; bad=%d", bad)
			}

			return out
		}

		first := runOnce(t)
		second := runOnce(t)

		if first.sumDelta != second.sumDelta || first.deltaIIT != second.deltaIIT || first.grossDiff != second.grossDiff {
			t.Fatalf("non-deterministic: first=%+v second=%+v", first, second)
		}
		for i := range first.items {
			if first.items[i] != second.items[i] {
				t.Fatalf("non-deterministic items: first=%+v second=%+v", first.items, second.items)
			}
		}
	})

	t.Run("allocation tie-breaker: item_code asc when residual ties", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		var codeWithExtra string
		if err := tx.QueryRow(ctx, `
			WITH
			inputs AS (
			  SELECT * FROM (VALUES
			    ('EARNING_A', 10000::bigint),
			    ('EARNING_B', 10000::bigint)
			  ) AS v(item_code, target_net_cents)
			),
			params AS (
			  SELECT 1::bigint AS delta_iit_cents, 20000::bigint AS group_target_net_cents
			),
			alloc_base AS (
			  SELECT
			    i.*,
			    (p.delta_iit_cents::numeric * i.target_net_cents::numeric) AS mul,
			    floor((p.delta_iit_cents::numeric * i.target_net_cents::numeric) / p.group_target_net_cents::numeric)::bigint AS q
			  FROM inputs i CROSS JOIN params p
			),
			alloc AS (
			  SELECT
			    a.*,
			    (a.mul - (a.q::numeric * (SELECT group_target_net_cents::numeric FROM params)))::bigint AS r
			  FROM alloc_base a
			),
			residual AS (
			  SELECT (SELECT delta_iit_cents FROM params) - sum(a.q) AS residual FROM alloc a
			),
			ranked AS (
			  SELECT
			    a.*,
			    row_number() OVER (ORDER BY a.r DESC, a.item_code ASC) AS rn,
			    (SELECT residual FROM residual) AS residual
			  FROM alloc a
			)
			SELECT item_code
			FROM ranked
			WHERE (q + CASE WHEN rn <= residual THEN 1 ELSE 0 END) = 1
			ORDER BY item_code
			LIMIT 1;
		`).Scan(&codeWithExtra); err != nil {
			t.Fatal(err)
		}
		if codeWithExtra != "EARNING_A" {
			t.Fatalf("expected tie-breaker to pick item_code asc, got=%q", codeWithExtra)
		}
	})
}

func ensurePayrollNetGuaranteedIITSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
	const runtimeRole = "bb_test_runtime"

	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,
		`CREATE SCHEMA IF NOT EXISTS staffing;`,
		`
CREATE OR REPLACE FUNCTION staffing.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('app.current_tenant', true) IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_TENANT_MISSING';
  END IF;
  IF current_setting('app.current_tenant')::uuid <> p_tenant_id THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_TENANT_MISMATCH';
  END IF;
END;
$$;
`,
		`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + runtimeRole + `') THEN
    CREATE ROLE ` + runtimeRole + ` LOGIN PASSWORD '` + runtimeRole + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
  END IF;
END;
$$;
`,
		`
CREATE TABLE IF NOT EXISTS staffing.pay_period_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  event_type text NOT NULL,
  pay_group text NOT NULL,
  period daterange NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pay_period_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT pay_period_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.pay_periods (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL,
  pay_group text NOT NULL,
  period daterange NOT NULL,
  status text NOT NULL DEFAULT 'open',
  closed_at timestamptz NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.pay_period_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id)
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payroll_run_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  event_type text NOT NULL,
  run_state text NOT NULL DEFAULT 'draft',
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payroll_run_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT payroll_run_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payroll_runs (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  run_state text NOT NULL DEFAULT 'draft',
  needs_recalc boolean NOT NULL DEFAULT false,
  calc_started_at timestamptz NULL,
  calc_finished_at timestamptz NULL,
  finalized_at timestamptz NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payroll_runs_pay_period_fk FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payslips (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL,
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  gross_pay numeric(15,2) NOT NULL DEFAULT 0,
  net_pay numeric(15,2) NOT NULL DEFAULT 0,
  employer_total numeric(15,2) NOT NULL DEFAULT 0,
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payslips_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslips_pay_period_fk FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payslip_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  amount numeric(15,2) NOT NULL,
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  calc_mode text NOT NULL DEFAULT 'amount',
  tax_bearer text NOT NULL DEFAULT 'employee',
  target_net numeric(15,2) NULL,
  iit_delta numeric(15,2) NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_items_payslip_fk FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE,
  CONSTRAINT payslip_items_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_items_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_items_iit_delta_nonneg_check CHECK (iit_delta IS NULL OR iit_delta >= 0),
  CONSTRAINT payslip_items_target_net_positive_check CHECK (target_net IS NULL OR target_net > 0),
  CONSTRAINT payslip_items_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      tax_bearer = 'employer'
      AND item_kind = 'earning'
      AND target_net IS NOT NULL
      AND iit_delta IS NOT NULL
      AND amount = target_net + iit_delta
    )
  )
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payslip_social_insurance_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  run_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  employee_amount numeric(15,2) NOT NULL DEFAULT 0,
  employer_amount numeric(15,2) NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_social_insurance_items_payslip_fk FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.iit_special_additional_deduction_claims (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  tax_month smallint NOT NULL,
  amount numeric(15,2) NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, person_uuid, tax_year, tax_month)
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payroll_balances (
  tenant_id uuid NOT NULL,
  tax_entity_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  first_tax_month smallint NOT NULL,
  last_tax_month smallint NOT NULL,
  ytd_income numeric NOT NULL DEFAULT 0,
  ytd_tax_exempt_income numeric NOT NULL DEFAULT 0,
  ytd_standard_deduction numeric NOT NULL DEFAULT 0,
  ytd_special_deduction numeric NOT NULL DEFAULT 0,
  ytd_special_additional_deduction numeric NOT NULL DEFAULT 0,
  ytd_taxable_income numeric NOT NULL DEFAULT 0,
  ytd_iit_tax_liability numeric NOT NULL DEFAULT 0,
  ytd_iit_withheld numeric NOT NULL DEFAULT 0,
  ytd_iit_credit numeric NOT NULL DEFAULT 0,
  last_pay_period_id uuid NOT NULL,
  last_run_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, tax_entity_id, person_uuid, tax_year)
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payslip_item_input_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  event_type text NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,
  tax_bearer text NOT NULL,
  amount numeric(15,2) NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_item_input_events_event_type_check CHECK (event_type IN ('UPSERT','DELETE')),
  CONSTRAINT payslip_item_input_events_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_input_events_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_input_events_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_input_events_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_input_events_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_input_events_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_input_events_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      item_kind = 'earning'
      AND tax_bearer = 'employer'
      AND currency = 'CNY'
    )
  ),
  CONSTRAINT payslip_item_input_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT payslip_item_input_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT payslip_item_input_events_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.payslip_item_inputs (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,
  tax_bearer text NOT NULL,
  amount numeric(15,2) NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.payslip_item_input_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payslip_item_inputs_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_inputs_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_inputs_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_inputs_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_inputs_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_inputs_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_inputs_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      item_kind = 'earning'
      AND tax_bearer = 'employer'
      AND currency = 'CNY'
    )
  ),
  CONSTRAINT payslip_item_inputs_natural_unique UNIQUE (tenant_id, run_id, person_uuid, assignment_id, item_code),
  CONSTRAINT payslip_item_inputs_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);
`,
		`
CREATE OR REPLACE FUNCTION staffing.iit_compute_cumulative_withholding(
  p_ytd_income numeric,
  p_ytd_tax_exempt_income numeric,
  p_ytd_standard_deduction numeric,
  p_ytd_special_deduction numeric,
  p_ytd_special_additional_deduction numeric,
  p_effective_withheld numeric
)
RETURNS TABLE (
  taxable_income numeric,
  tax_liability numeric,
  delta numeric,
  withhold_this_month numeric,
  credit numeric,
  rate numeric,
  quick_deduction numeric
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_taxable numeric;
  v_rate numeric;
  v_quick numeric;
  v_tax_liability numeric;
  v_delta numeric;
BEGIN
  v_taxable := greatest(
    0,
    coalesce(p_ytd_income, 0)
      - coalesce(p_ytd_tax_exempt_income, 0)
      - coalesce(p_ytd_standard_deduction, 0)
      - coalesce(p_ytd_special_deduction, 0)
      - coalesce(p_ytd_special_additional_deduction, 0)
  );

  IF v_taxable <= 36000 THEN
    v_rate := 0.03;
    v_quick := 0;
  ELSIF v_taxable <= 144000 THEN
    v_rate := 0.10;
    v_quick := 2520;
  ELSIF v_taxable <= 300000 THEN
    v_rate := 0.20;
    v_quick := 16920;
  ELSIF v_taxable <= 420000 THEN
    v_rate := 0.25;
    v_quick := 31920;
  ELSIF v_taxable <= 660000 THEN
    v_rate := 0.30;
    v_quick := 52920;
  ELSIF v_taxable <= 960000 THEN
    v_rate := 0.35;
    v_quick := 85920;
  ELSE
    v_rate := 0.45;
    v_quick := 181920;
  END IF;

  v_tax_liability := round(v_taxable * v_rate - v_quick, 2);
  IF v_tax_liability < 0 THEN
    v_tax_liability := 0;
  END IF;

  v_delta := v_tax_liability - coalesce(p_effective_withheld, 0);

  taxable_income := round(v_taxable, 2);
  tax_liability := v_tax_liability;
  delta := round(v_delta, 2);
  rate := v_rate;
  quick_deduction := v_quick;

  IF v_delta > 0 THEN
    withhold_this_month := round(v_delta, 2);
    credit := 0;
  ELSE
    withhold_this_month := 0;
    credit := round(-v_delta, 2);
  END IF;

  RETURN NEXT;
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION staffing.iit_withhold_this_month_cents(
  p_ytd_income_cents bigint,
  p_ytd_tax_exempt_income_cents bigint,
  p_ytd_standard_deduction_cents bigint,
  p_ytd_special_deduction_cents bigint,
  p_ytd_special_additional_deduction_cents bigint,
  p_effective_withheld_cents bigint
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_withhold_this_month numeric;
BEGIN
  SELECT t.withhold_this_month INTO v_withhold_this_month
  FROM staffing.iit_compute_cumulative_withholding(
    round(coalesce(p_ytd_income_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_tax_exempt_income_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_standard_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_special_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_special_additional_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_effective_withheld_cents, 0)::numeric / 100, 2)
  ) t;

  RETURN round(coalesce(v_withhold_this_month, 0) * 100, 0)::bigint;
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION staffing.payroll_apply_iit(
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
  p_run_event_db_id bigint,
  p_now timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_period daterange;
  v_period_start date;
  v_period_end_excl date;
  v_tax_year integer;
  v_tax_month smallint;

  v_int64_max constant bigint := 9223372036854775807;
  v_net_guaranteed_payslip record;

  v_base_income_cents bigint;
  v_si_employee_cents bigint;
  v_sad_amount_cents bigint;
  v_first_tax_month smallint;

  v_prev_ytd_income_cents bigint;
  v_prev_ytd_tax_exempt_income_cents bigint;
  v_prev_ytd_special_deduction_cents bigint;
  v_prev_ytd_special_additional_deduction_cents bigint;
  v_prev_ytd_iit_withheld_cents bigint;
  v_prev_ytd_iit_credit_cents bigint;

  v_ytd_income_base_cents bigint;
  v_ytd_tax_exempt_income_cents bigint;
  v_ytd_standard_deduction_cents bigint;
  v_ytd_special_deduction_cents bigint;
  v_ytd_special_additional_deduction_cents bigint;
  v_effective_withheld_cents bigint;

  v_group_target_net_cents bigint;
  v_base_iit_withhold_cents bigint;
  v_test_iit_withhold_cents bigint;
  v_delta_iit_cents bigint;
  v_test_net_cents bigint;

  v_lo_cents bigint;
  v_hi_cents bigint;
  v_mid_cents bigint;
  v_expand int;
  v_iters int;
  v_solved_gross_cents bigint;
  v_group_delta_iit_cents bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_run_event_db_id IS NULL OR p_run_event_db_id <= 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_event_db_id is required';
  END IF;
  IF p_now IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'now is required';
  END IF;

  SELECT period INTO v_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', p_pay_period_id);
  END IF;

  v_period_start := lower(v_period);
  v_period_end_excl := upper(v_period);
  IF v_period_start IS NULL OR v_period_end_excl IS NULL
    OR date_trunc('month', v_period_start)::date <> v_period_start
    OR (v_period_start + interval '1 month')::date <> v_period_end_excl
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_PERIOD_NOT_MONTHLY',
      DETAIL = format('period=%s', v_period);
  END IF;

  v_tax_year := extract(year from v_period_start)::integer;
  v_tax_month := extract(month from v_period_start)::smallint;

  IF EXISTS (
    SELECT 1
    FROM staffing.payslips p
    JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND v_tax_month <= b.last_tax_month
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_BALANCES_MONTH_NOT_ADVANCING',
      DETAIL = format('tax_year=%s tax_month=%s', v_tax_year, v_tax_month);
  END IF;

  FOR v_net_guaranteed_payslip IN
    WITH
    si AS (
      SELECT
        i.payslip_id,
        COALESCE(sum(i.employee_amount), 0) AS employee_amount
      FROM staffing.payslip_social_insurance_items i
      WHERE i.tenant_id = p_tenant_id AND i.run_id = p_run_id
      GROUP BY i.payslip_id
    ),
    sad AS (
      SELECT
        c.person_uuid,
        c.amount
      FROM staffing.iit_special_additional_deduction_claims c
      WHERE c.tenant_id = p_tenant_id
        AND c.tax_year = v_tax_year
        AND c.tax_month = v_tax_month
    )
    SELECT
      p.id AS payslip_id,
      p.person_uuid,
      p.assignment_id,
      round(p.gross_pay * 100, 0)::bigint AS base_income_cents,
      round(COALESCE(si.employee_amount, 0) * 100, 0)::bigint AS si_employee_cents,
      round(COALESCE(sad.amount, 0) * 100, 0)::bigint AS sad_amount_cents,
      COALESCE(b.first_tax_month, v_tax_month) AS first_tax_month,
      round(COALESCE(b.ytd_income, 0) * 100, 0)::bigint AS prev_ytd_income_cents,
      round(COALESCE(b.ytd_tax_exempt_income, 0) * 100, 0)::bigint AS prev_ytd_tax_exempt_income_cents,
      round(COALESCE(b.ytd_special_deduction, 0) * 100, 0)::bigint AS prev_ytd_special_deduction_cents,
      round(COALESCE(b.ytd_special_additional_deduction, 0) * 100, 0)::bigint AS prev_ytd_special_additional_deduction_cents,
      round(COALESCE(b.ytd_iit_withheld, 0) * 100, 0)::bigint AS prev_ytd_iit_withheld_cents,
      round(COALESCE(b.ytd_iit_credit, 0) * 100, 0)::bigint AS prev_ytd_iit_credit_cents
    FROM staffing.payslips p
    LEFT JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    LEFT JOIN si ON si.payslip_id = p.id
    LEFT JOIN sad ON sad.person_uuid = p.person_uuid
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND EXISTS (
        SELECT 1
        FROM staffing.payslip_item_inputs i
        WHERE i.tenant_id = p.tenant_id
          AND i.run_id = p.run_id
          AND i.person_uuid = p.person_uuid
          AND i.assignment_id = p.assignment_id
          AND i.calc_mode = 'net_guaranteed_iit'
      )
  LOOP
    v_base_income_cents := v_net_guaranteed_payslip.base_income_cents;
    v_si_employee_cents := v_net_guaranteed_payslip.si_employee_cents;
    v_sad_amount_cents := v_net_guaranteed_payslip.sad_amount_cents;
    v_first_tax_month := v_net_guaranteed_payslip.first_tax_month;

    v_prev_ytd_income_cents := v_net_guaranteed_payslip.prev_ytd_income_cents;
    v_prev_ytd_tax_exempt_income_cents := v_net_guaranteed_payslip.prev_ytd_tax_exempt_income_cents;
    v_prev_ytd_special_deduction_cents := v_net_guaranteed_payslip.prev_ytd_special_deduction_cents;
    v_prev_ytd_special_additional_deduction_cents := v_net_guaranteed_payslip.prev_ytd_special_additional_deduction_cents;
    v_prev_ytd_iit_withheld_cents := v_net_guaranteed_payslip.prev_ytd_iit_withheld_cents;
    v_prev_ytd_iit_credit_cents := v_net_guaranteed_payslip.prev_ytd_iit_credit_cents;

    SELECT round(sum(i.amount) * 100, 0)::bigint INTO v_group_target_net_cents
    FROM staffing.payslip_item_inputs i
    WHERE i.tenant_id = p_tenant_id
      AND i.run_id = p_run_id
      AND i.person_uuid = v_net_guaranteed_payslip.person_uuid
      AND i.assignment_id = v_net_guaranteed_payslip.assignment_id
      AND i.calc_mode = 'net_guaranteed_iit';

    IF v_group_target_net_cents IS NULL OR v_group_target_net_cents <= 0 THEN
      CONTINUE;
    END IF;

    v_ytd_income_base_cents := v_prev_ytd_income_cents + v_base_income_cents;
    v_ytd_tax_exempt_income_cents := v_prev_ytd_tax_exempt_income_cents;
    v_ytd_special_deduction_cents := v_prev_ytd_special_deduction_cents + v_si_employee_cents;
    v_ytd_special_additional_deduction_cents := v_prev_ytd_special_additional_deduction_cents + v_sad_amount_cents;
    v_ytd_standard_deduction_cents := 5000 * 100 * (v_tax_month - v_first_tax_month + 1);
    v_effective_withheld_cents := v_prev_ytd_iit_withheld_cents + v_prev_ytd_iit_credit_cents;

    v_base_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );

    v_lo_cents := v_group_target_net_cents;
    v_hi_cents := v_group_target_net_cents;

    FOR v_expand IN 1..32 LOOP
      v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
        v_ytd_income_base_cents + v_hi_cents,
        v_ytd_tax_exempt_income_cents,
        v_ytd_standard_deduction_cents,
        v_ytd_special_deduction_cents,
        v_ytd_special_additional_deduction_cents,
        v_effective_withheld_cents
      );
      v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
      v_test_net_cents := v_hi_cents - v_delta_iit_cents;

      IF v_test_net_cents >= v_group_target_net_cents THEN
        EXIT;
      END IF;

      IF v_hi_cents > v_int64_max / 2 OR v_hi_cents > v_int64_max - v_ytd_income_base_cents THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_UPPER_BOUND_EXHAUSTED',
          DETAIL = format(
            'payslip_id=%s person_uuid=%s assignment_id=%s base_income=%s target_net=%s',
            v_net_guaranteed_payslip.payslip_id,
            v_net_guaranteed_payslip.person_uuid,
            v_net_guaranteed_payslip.assignment_id,
            (v_base_income_cents::numeric / 100)::text,
            (v_group_target_net_cents::numeric / 100)::text
          );
      END IF;

      v_hi_cents := v_hi_cents * 2;
    END LOOP;

    v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents + v_hi_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );
    v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
    v_test_net_cents := v_hi_cents - v_delta_iit_cents;
    IF v_test_net_cents < v_group_target_net_cents THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_UPPER_BOUND_EXHAUSTED',
        DETAIL = format(
          'payslip_id=%s person_uuid=%s assignment_id=%s base_income=%s target_net=%s hi=%s hi_net=%s',
          v_net_guaranteed_payslip.payslip_id,
          v_net_guaranteed_payslip.person_uuid,
          v_net_guaranteed_payslip.assignment_id,
          (v_base_income_cents::numeric / 100)::text,
          (v_group_target_net_cents::numeric / 100)::text,
          (v_hi_cents::numeric / 100)::text,
          (v_test_net_cents::numeric / 100)::text
        );
    END IF;

    v_iters := 0;
    WHILE v_lo_cents < v_hi_cents LOOP
      v_iters := v_iters + 1;
      v_mid_cents := (v_lo_cents + v_hi_cents) / 2;

      v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
        v_ytd_income_base_cents + v_mid_cents,
        v_ytd_tax_exempt_income_cents,
        v_ytd_standard_deduction_cents,
        v_ytd_special_deduction_cents,
        v_ytd_special_additional_deduction_cents,
        v_effective_withheld_cents
      );
      v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
      v_test_net_cents := v_mid_cents - v_delta_iit_cents;

      IF v_test_net_cents >= v_group_target_net_cents THEN
        v_hi_cents := v_mid_cents;
      ELSE
        v_lo_cents := v_mid_cents + 1;
      END IF;
    END LOOP;

    v_solved_gross_cents := v_lo_cents;
    v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents + v_solved_gross_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );
    v_group_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;

    IF v_solved_gross_cents - v_group_delta_iit_cents <> v_group_target_net_cents THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_CONTRACT_VIOLATION',
        DETAIL = format(
          'payslip_id=%s person_uuid=%s assignment_id=%s target_net=%s solved_gross=%s delta_iit=%s',
          v_net_guaranteed_payslip.payslip_id,
          v_net_guaranteed_payslip.person_uuid,
          v_net_guaranteed_payslip.assignment_id,
          (v_group_target_net_cents::numeric / 100)::text,
          (v_solved_gross_cents::numeric / 100)::text,
          (v_group_delta_iit_cents::numeric / 100)::text
        );
    END IF;

    WITH
    inputs AS (
      SELECT
        i.id AS input_id,
        i.item_code,
        round(i.amount * 100, 0)::bigint AS target_net_cents,
        i.last_event_id AS input_last_event_id
      FROM staffing.payslip_item_inputs i
      WHERE i.tenant_id = p_tenant_id
        AND i.run_id = p_run_id
        AND i.person_uuid = v_net_guaranteed_payslip.person_uuid
        AND i.assignment_id = v_net_guaranteed_payslip.assignment_id
        AND i.calc_mode = 'net_guaranteed_iit'
    ),
    alloc_base AS (
      SELECT
        i.*,
        (v_group_delta_iit_cents::numeric * i.target_net_cents::numeric) AS mul,
        floor((v_group_delta_iit_cents::numeric * i.target_net_cents::numeric) / v_group_target_net_cents::numeric)::bigint AS q
      FROM inputs i
    ),
    alloc AS (
      SELECT
        a.*,
        (a.mul - (a.q::numeric * v_group_target_net_cents::numeric))::bigint AS r
      FROM alloc_base a
    ),
    residual AS (
      SELECT
        v_group_delta_iit_cents - sum(a.q) AS residual
      FROM alloc a
    ),
    ranked AS (
      SELECT
        a.*,
        row_number() OVER (ORDER BY a.r DESC, a.item_code ASC) AS rn,
        (SELECT residual FROM residual) AS residual
      FROM alloc a
    )
    INSERT INTO staffing.payslip_items (
      tenant_id,
      payslip_id,
      item_code,
      item_kind,
      amount,
      meta,
      last_run_event_id,
      calc_mode,
      tax_bearer,
      target_net,
      iit_delta
    )
    SELECT
      p_tenant_id,
      v_net_guaranteed_payslip.payslip_id,
      r.item_code,
      'earning',
      round(((r.target_net_cents + (r.q + CASE WHEN r.rn <= r.residual THEN 1 ELSE 0 END))::numeric) / 100, 2),
      jsonb_build_object(
        'input_id', r.input_id::text,
        'input_last_event_id', r.input_last_event_id::text,
        'tax_year', v_tax_year::text,
        'tax_month', v_tax_month::text,
        'group_target_net', (v_group_target_net_cents::numeric / 100)::text,
        'group_solved_gross', (v_solved_gross_cents::numeric / 100)::text,
        'group_delta_iit', (v_group_delta_iit_cents::numeric / 100)::text,
        'base_income', (v_base_income_cents::numeric / 100)::text,
        'base_iit_withhold', (v_base_iit_withhold_cents::numeric / 100)::text,
        'iterations', v_iters::text
      ),
      p_run_event_db_id,
      'net_guaranteed_iit',
      'employer',
      round((r.target_net_cents::numeric) / 100, 2),
      round(((r.q + CASE WHEN r.rn <= r.residual THEN 1 ELSE 0 END)::numeric) / 100, 2)
    FROM ranked r;

    UPDATE staffing.payslips p
    SET
      gross_pay = p.gross_pay + round(v_solved_gross_cents::numeric / 100, 2),
      net_pay = p.net_pay + round(v_solved_gross_cents::numeric / 100, 2),
      last_run_event_id = p_run_event_db_id,
      updated_at = p_now
    WHERE p.tenant_id = p_tenant_id AND p.id = v_net_guaranteed_payslip.payslip_id;
  END LOOP;

  WITH
  si AS (
    SELECT
      i.payslip_id,
      COALESCE(sum(i.employee_amount), 0) AS employee_amount
    FROM staffing.payslip_social_insurance_items i
    WHERE i.tenant_id = p_tenant_id AND i.run_id = p_run_id
    GROUP BY i.payslip_id
  ),
  sad AS (
    SELECT
      c.person_uuid,
      c.amount
    FROM staffing.iit_special_additional_deduction_claims c
    WHERE c.tenant_id = p_tenant_id
      AND c.tax_year = v_tax_year
      AND c.tax_month = v_tax_month
  ),
  calc AS (
    SELECT
      p.id AS payslip_id,
      p.person_uuid,
      p.gross_pay AS income_this_month,
      COALESCE(si.employee_amount, 0) AS si_employee_amount,
      COALESCE(sad.amount, 0) AS sad_amount_this_month,
      COALESCE(b.first_tax_month, v_tax_month) AS first_tax_month,
      COALESCE(b.ytd_income, 0) AS prev_ytd_income,
      COALESCE(b.ytd_tax_exempt_income, 0) AS prev_ytd_tax_exempt_income,
      COALESCE(b.ytd_special_deduction, 0) AS prev_ytd_special_deduction,
      COALESCE(b.ytd_special_additional_deduction, 0) AS prev_ytd_special_additional_deduction,
      COALESCE(b.ytd_iit_withheld, 0) AS prev_ytd_iit_withheld,
      COALESCE(b.ytd_iit_credit, 0) AS prev_ytd_iit_credit
    FROM staffing.payslips p
    LEFT JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    LEFT JOIN si ON si.payslip_id = p.id
    LEFT JOIN sad ON sad.person_uuid = p.person_uuid
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
  ),
  ytd AS (
    SELECT
      c.*,
      (c.prev_ytd_income + c.income_this_month) AS ytd_income,
      c.prev_ytd_tax_exempt_income AS ytd_tax_exempt_income,
      (c.prev_ytd_special_deduction + c.si_employee_amount) AS ytd_special_deduction,
      (c.prev_ytd_special_additional_deduction + c.sad_amount_this_month) AS ytd_special_additional_deduction,
      (5000::numeric * (v_tax_month - c.first_tax_month + 1)::numeric) AS ytd_standard_deduction,
      (c.prev_ytd_iit_withheld + c.prev_ytd_iit_credit) AS effective_withheld
    FROM calc c
  ),
  iit AS (
    SELECT
      y.*,
      t.taxable_income,
      t.tax_liability,
      t.delta,
      t.withhold_this_month,
      t.credit,
      t.rate,
      t.quick_deduction
    FROM ytd y
    CROSS JOIN LATERAL staffing.iit_compute_cumulative_withholding(
      y.ytd_income,
      y.ytd_tax_exempt_income,
      y.ytd_standard_deduction,
      y.ytd_special_deduction,
      y.ytd_special_additional_deduction,
      y.effective_withheld
    ) t
  )
  INSERT INTO staffing.payslip_items (
    tenant_id,
    payslip_id,
    item_code,
    item_kind,
    amount,
    meta,
    last_run_event_id
  )
  SELECT
    p_tenant_id,
    iit.payslip_id,
    'DEDUCTION_IIT_WITHHOLDING',
    'deduction',
    iit.withhold_this_month,
    jsonb_build_object(
      'tax_year', v_tax_year::text,
      'tax_month', v_tax_month::text,
      'first_tax_month', iit.first_tax_month::text,
      'income_this_month', iit.income_this_month::text,
      'si_employee_amount', iit.si_employee_amount::text,
      'sad_amount_this_month', iit.sad_amount_this_month::text,
      'ytd_income', iit.ytd_income::text,
      'ytd_tax_exempt_income', iit.ytd_tax_exempt_income::text,
      'ytd_standard_deduction', iit.ytd_standard_deduction::text,
      'ytd_special_deduction', iit.ytd_special_deduction::text,
      'ytd_special_additional_deduction', iit.ytd_special_additional_deduction::text,
      'taxable_income', iit.taxable_income::text,
      'rate', iit.rate::text,
      'quick_deduction', iit.quick_deduction::text,
      'tax_liability', iit.tax_liability::text,
      'effective_withheld', iit.effective_withheld::text,
      'delta', iit.delta::text,
      'withhold_this_month', iit.withhold_this_month::text,
      'credit', iit.credit::text
    ),
    p_run_event_db_id
  FROM iit;

  UPDATE staffing.payslips p
  SET
    net_pay = p.net_pay - iit.withhold_this_month,
    last_run_event_id = p_run_event_db_id,
    updated_at = p_now
  FROM (
    SELECT
      p.id AS payslip_id,
      i.amount AS withhold_this_month
    FROM staffing.payslips p
    JOIN staffing.payslip_items i
      ON i.tenant_id = p.tenant_id
      AND i.payslip_id = p.id
      AND i.item_code = 'DEDUCTION_IIT_WITHHOLDING'
      AND i.last_run_event_id = p_run_event_db_id
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
  ) AS iit
  WHERE p.tenant_id = p_tenant_id AND p.id = iit.payslip_id;
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION staffing.submit_payslip_item_input_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_run_id uuid,
  p_person_uuid uuid,
  p_assignment_id uuid,
  p_event_type text,
  p_item_code text,
  p_item_kind text,
  p_currency char(3),
  p_calc_mode text,
  p_tax_bearer text,
  p_amount numeric(15,2),
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing_event staffing.payslip_item_input_events%ROWTYPE;
  v_run staffing.payroll_runs%ROWTYPE;
  v_existing_input staffing.payslip_item_inputs%ROWTYPE;

  v_now timestamptz;

  v_item_code text;
  v_item_kind text;
  v_currency char(3);
  v_calc_mode text;
  v_tax_bearer text;
  v_amount numeric(15,2);

  v_currency_trim text;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_event_type IS NULL OR p_event_type NOT IN ('UPSERT','DELETE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_item_code := COALESCE(p_item_code, '');
  IF btrim(v_item_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code is required';
  END IF;
  IF v_item_code <> btrim(v_item_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code must be trimmed';
  END IF;
  IF v_item_code <> upper(v_item_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code must be upper';
  END IF;
  IF v_item_code !~ '^[A-Z0-9_]+$' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code invalid';
  END IF;

  IF btrim(COALESCE(p_request_id, '')) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:payroll-run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF v_run.run_state = 'finalized' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_FINALIZED_READONLY',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM staffing.payslips p
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND p.person_uuid = p_person_uuid
      AND p.assignment_id = p_assignment_id
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
      DETAIL = format('payslip not found: run_id=%s person_uuid=%s assignment_id=%s', p_run_id, p_person_uuid, p_assignment_id);
  END IF;

  v_now := now();

  IF p_event_type = 'DELETE' THEN
    SELECT * INTO v_existing_input
    FROM staffing.payslip_item_inputs i
    WHERE i.tenant_id = p_tenant_id
      AND i.run_id = p_run_id
      AND i.person_uuid = p_person_uuid
      AND i.assignment_id = p_assignment_id
      AND i.item_code = v_item_code;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('input not found: run_id=%s person_uuid=%s assignment_id=%s item_code=%s', p_run_id, p_person_uuid, p_assignment_id, v_item_code);
    END IF;

    v_item_kind := v_existing_input.item_kind;
    v_currency := v_existing_input.currency;
    v_calc_mode := v_existing_input.calc_mode;
    v_tax_bearer := v_existing_input.tax_bearer;
    v_amount := v_existing_input.amount;
  ELSE
    IF p_item_kind IS NULL OR p_item_kind NOT IN ('earning','deduction','employer_cost') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported item_kind: %s', p_item_kind);
    END IF;

    IF p_calc_mode IS NULL OR p_calc_mode NOT IN ('amount','net_guaranteed_iit') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported calc_mode: %s', p_calc_mode);
    END IF;

    IF p_tax_bearer IS NULL OR p_tax_bearer NOT IN ('employee','employer') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported tax_bearer: %s', p_tax_bearer);
    END IF;

    IF p_currency IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = 'currency is required';
    END IF;

    v_currency_trim := btrim(p_currency::text);
    IF v_currency_trim = '' THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency is required';
    END IF;
    IF v_currency_trim <> upper(v_currency_trim) THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency must be upper';
    END IF;
    IF length(v_currency_trim) <> 3 THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency must be 3 letters';
    END IF;

    IF p_amount IS NULL OR p_amount <= 0 THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'amount must be > 0';
    END IF;

    IF p_calc_mode = 'net_guaranteed_iit' THEN
      IF v_currency_trim <> 'CNY' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_CURRENCY_MISMATCH',
          DETAIL = format('currency=%s', v_currency_trim);
      END IF;
      IF p_tax_bearer <> 'employer' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
          DETAIL = format('tax_bearer must be employer, got=%s', p_tax_bearer);
      END IF;
      IF p_item_kind <> 'earning' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
          DETAIL = format('item_kind must be earning, got=%s', p_item_kind);
      END IF;
    END IF;

    v_item_kind := p_item_kind;
    v_currency := p_currency;
    v_calc_mode := p_calc_mode;
    v_tax_bearer := p_tax_bearer;
    v_amount := p_amount;
  END IF;

  INSERT INTO staffing.payslip_item_input_events (
    event_id,
    tenant_id,
    run_id,
    person_uuid,
    assignment_id,
    event_type,
    item_code,
    item_kind,
    currency,
    calc_mode,
    tax_bearer,
    amount,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_run_id,
    p_person_uuid,
    p_assignment_id,
    p_event_type,
    v_item_code,
    v_item_kind,
    v_currency,
    v_calc_mode,
    v_tax_bearer,
    v_amount,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing_event
    FROM staffing.payslip_item_input_events
    WHERE event_id = p_event_id;

    IF v_existing_event.tenant_id <> p_tenant_id
      OR v_existing_event.run_id <> p_run_id
      OR v_existing_event.person_uuid <> p_person_uuid
      OR v_existing_event.assignment_id <> p_assignment_id
      OR v_existing_event.event_type <> p_event_type
      OR v_existing_event.item_code <> v_item_code
      OR v_existing_event.item_kind <> v_item_kind
      OR v_existing_event.currency <> v_currency
      OR v_existing_event.calc_mode <> v_calc_mode
      OR v_existing_event.tax_bearer <> v_tax_bearer
      OR v_existing_event.amount <> v_amount
      OR v_existing_event.request_id <> p_request_id
      OR v_existing_event.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_event.id);
    END IF;

    RETURN v_existing_event.id;
  END IF;

  IF p_event_type = 'UPSERT' THEN
    INSERT INTO staffing.payslip_item_inputs (
      tenant_id,
      run_id,
      person_uuid,
      assignment_id,
      item_code,
      item_kind,
      currency,
      calc_mode,
      tax_bearer,
      amount,
      last_event_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_person_uuid,
      p_assignment_id,
      v_item_code,
      v_item_kind,
      v_currency,
      v_calc_mode,
      v_tax_bearer,
      v_amount,
      v_event_db_id,
      v_now,
      v_now
    )
    ON CONFLICT ON CONSTRAINT payslip_item_inputs_natural_unique
    DO UPDATE SET
      item_kind = EXCLUDED.item_kind,
      currency = EXCLUDED.currency,
      calc_mode = EXCLUDED.calc_mode,
      tax_bearer = EXCLUDED.tax_bearer,
      amount = EXCLUDED.amount,
      last_event_id = EXCLUDED.last_event_id,
      updated_at = EXCLUDED.updated_at;
  ELSE
    DELETE FROM staffing.payslip_item_inputs
    WHERE tenant_id = p_tenant_id
      AND run_id = p_run_id
      AND person_uuid = p_person_uuid
      AND assignment_id = p_assignment_id
      AND item_code = v_item_code;
  END IF;

  IF v_run.run_state = 'calculated' THEN
    UPDATE staffing.payroll_runs
    SET
      needs_recalc = true,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  END IF;

  RETURN v_event_db_id;
END;
$$;
`,
		`ALTER TABLE staffing.pay_period_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.pay_period_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.pay_period_events;`,
		`CREATE POLICY tenant_isolation ON staffing.pay_period_events USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payroll_run_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payroll_run_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_run_events;`,
		`CREATE POLICY tenant_isolation ON staffing.payroll_run_events USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.pay_periods ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.pay_periods FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.pay_periods;`,
		`CREATE POLICY tenant_isolation ON staffing.pay_periods USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payroll_runs ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payroll_runs FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_runs;`,
		`CREATE POLICY tenant_isolation ON staffing.payroll_runs USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payslips ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payslips FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payslips;`,
		`CREATE POLICY tenant_isolation ON staffing.payslips USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payslip_items ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payslip_items FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_items;`,
		`CREATE POLICY tenant_isolation ON staffing.payslip_items USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payslip_social_insurance_items ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payslip_social_insurance_items FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_social_insurance_items;`,
		`CREATE POLICY tenant_isolation ON staffing.payslip_social_insurance_items USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.iit_special_additional_deduction_claims ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.iit_special_additional_deduction_claims FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.iit_special_additional_deduction_claims;`,
		`CREATE POLICY tenant_isolation ON staffing.iit_special_additional_deduction_claims USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payroll_balances ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payroll_balances FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_balances;`,
		`CREATE POLICY tenant_isolation ON staffing.payroll_balances USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payslip_item_input_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payslip_item_input_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_input_events;`,
		`CREATE POLICY tenant_isolation ON staffing.payslip_item_input_events USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`ALTER TABLE staffing.payslip_item_inputs ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.payslip_item_inputs FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_inputs;`,
		`CREATE POLICY tenant_isolation ON staffing.payslip_item_inputs USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);`,
		`GRANT USAGE ON SCHEMA staffing TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.pay_period_events TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.pay_period_events_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.pay_periods TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.payroll_run_events TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.payroll_run_events_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.payroll_runs TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.payslips TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.payslip_items TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.payslip_items_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.payslip_social_insurance_items TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.payslip_social_insurance_items_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.iit_special_additional_deduction_claims TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.payroll_balances TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.payslip_item_input_events TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.payslip_item_input_events_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON staffing.payslip_item_inputs TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.assert_current_tenant(uuid) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.iit_compute_cumulative_withholding(numeric, numeric, numeric, numeric, numeric, numeric) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.iit_withhold_this_month_cents(bigint, bigint, bigint, bigint, bigint, bigint) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.payroll_apply_iit(uuid, uuid, uuid, bigint, timestamptz) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.submit_payslip_item_input_event(uuid, uuid, uuid, uuid, uuid, text, text, text, char(3), text, text, numeric, text, uuid) TO ` + runtimeRole + `;`,
	}

	for _, q := range ddl {
		if _, err := conn.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
