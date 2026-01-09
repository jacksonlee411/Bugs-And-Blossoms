package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type timePunchRows struct {
	idx     int
	scanErr error
	err     error
	rows    []TimePunch
}

func (r *timePunchRows) Close()                        {}
func (r *timePunchRows) Err() error                    { return r.err }
func (r *timePunchRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *timePunchRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *timePunchRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *timePunchRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]
	*(dest[0].(*string)) = row.EventID
	*(dest[1].(*string)) = row.PersonUUID
	*(dest[2].(*time.Time)) = row.PunchTime
	*(dest[3].(*string)) = row.PunchType
	*(dest[4].(*string)) = row.SourceProvider
	*(dest[5].(*[]byte)) = append([]byte(nil), row.Payload...)
	*(dest[6].(*time.Time)) = row.TransactionTime
	return nil
}
func (r *timePunchRows) Values() ([]any, error) { return nil, nil }
func (r *timePunchRows) RawValues() [][]byte    { return nil }
func (r *timePunchRows) Conn() *pgx.Conn        { return nil }

type timePunchRow struct {
	scanErr error
	p       TimePunch
	payload []byte
}

func (r timePunchRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = r.p.EventID
	*(dest[1].(*string)) = r.p.PersonUUID
	*(dest[2].(*time.Time)) = r.p.PunchTime
	*(dest[3].(*string)) = r.p.PunchType
	*(dest[4].(*string)) = r.p.SourceProvider
	*(dest[5].(*[]byte)) = append([]byte(nil), r.payload...)
	*(dest[6].(*time.Time)) = r.p.TransactionTime
	return nil
}

type dailyAttendanceResultRows struct {
	idx     int
	scanErr error
	err     error
	rows    []DailyAttendanceResult
}

func (r *dailyAttendanceResultRows) Close()                        {}
func (r *dailyAttendanceResultRows) Err() error                    { return r.err }
func (r *dailyAttendanceResultRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *dailyAttendanceResultRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *dailyAttendanceResultRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *dailyAttendanceResultRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]

	*(dest[0].(*string)) = row.PersonUUID
	*(dest[1].(*string)) = row.WorkDate
	*(dest[2].(*string)) = row.RulesetVersion
	*(dest[3].(*string)) = row.Status
	*(dest[4].(*[]string)) = append([]string(nil), row.Flags...)

	*(dest[5].(**time.Time)) = row.FirstInTime
	*(dest[6].(**time.Time)) = row.LastOutTime

	*(dest[7].(*int)) = row.WorkedMinutes
	*(dest[8].(*int)) = row.LateMinutes
	*(dest[9].(*int)) = row.EarlyLeaveMinutes

	*(dest[10].(*int)) = row.InputPunchCount
	*(dest[11].(**int64)) = row.InputMaxPunchEventDBID
	*(dest[12].(**time.Time)) = row.InputMaxPunchTime

	*(dest[13].(*time.Time)) = row.ComputedAt
	*(dest[14].(*time.Time)) = row.CreatedAt
	*(dest[15].(*time.Time)) = row.UpdatedAt
	return nil
}
func (r *dailyAttendanceResultRows) Values() ([]any, error) { return nil, nil }
func (r *dailyAttendanceResultRows) RawValues() [][]byte    { return nil }
func (r *dailyAttendanceResultRows) Conn() *pgx.Conn        { return nil }

type dailyAttendanceResultRow struct {
	scanErr error
	r       DailyAttendanceResult
}

func (row dailyAttendanceResultRow) Scan(dest ...any) error {
	if row.scanErr != nil {
		return row.scanErr
	}

	*(dest[0].(*string)) = row.r.PersonUUID
	*(dest[1].(*string)) = row.r.WorkDate
	*(dest[2].(*string)) = row.r.RulesetVersion
	*(dest[3].(*string)) = row.r.Status
	*(dest[4].(*[]string)) = append([]string(nil), row.r.Flags...)
	*(dest[5].(**time.Time)) = row.r.FirstInTime
	*(dest[6].(**time.Time)) = row.r.LastOutTime
	*(dest[7].(*int)) = row.r.WorkedMinutes
	*(dest[8].(*int)) = row.r.LateMinutes
	*(dest[9].(*int)) = row.r.EarlyLeaveMinutes
	*(dest[10].(*int)) = row.r.InputPunchCount
	*(dest[11].(**int64)) = row.r.InputMaxPunchEventDBID
	*(dest[12].(**time.Time)) = row.r.InputMaxPunchTime
	*(dest[13].(*time.Time)) = row.r.ComputedAt
	*(dest[14].(*time.Time)) = row.r.CreatedAt
	*(dest[15].(*time.Time)) = row.r.UpdatedAt
	return nil
}

func TestStaffingPGStore_ListTimePunchesForPerson(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{queryErr: errors.New("query")})
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &timePunchRows{rows: []TimePunch{{EventID: "e1"}}, scanErr: errors.New("scan")}})
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &timePunchRows{rows: []TimePunch{{EventID: "e1"}}, err: errors.New("rows")}})
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &timePunchRows{rows: []TimePunch{{EventID: "e1"}}}, commitErr: errors.New("commit")})
		_, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC(), 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (limit clamp)", func(t *testing.T) {
		punchTimeLocal := time.Date(2026, 1, 1, 8, 0, 0, 0, time.FixedZone("X", 8*60*60))
		txTimeLocal := time.Date(2026, 1, 1, 8, 1, 0, 0, time.FixedZone("Y", -7*60*60))
		store := newStaffingPGStore(&stubTx{rows: &timePunchRows{rows: []TimePunch{{
			EventID:         "e1",
			PersonUUID:      "p1",
			PunchTime:       punchTimeLocal,
			PunchType:       "IN",
			SourceProvider:  "MANUAL",
			Payload:         json.RawMessage(`{"x":1}`),
			TransactionTime: txTimeLocal,
		}}}})
		out, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(999999, 0).UTC(), 0)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 1 {
			t.Fatalf("len=%d", len(out))
		}
		if out[0].PunchTime.Location() != time.UTC || out[0].TransactionTime.Location() != time.UTC {
			t.Fatalf("expected UTC times")
		}

		_, err = store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(999999, 0).UTC(), 5000)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestStaffingPGStore_SubmitTimePunch(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id generate error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("uuid")})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("person uuid required", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{EventID: "e1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("punch type required", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{EventID: "e1", PersonUUID: "p1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("kernel error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("kernel")})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{EventID: "e1", PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row:     fakeRow{},
			row2Err: errors.New("select"),
		})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{EventID: "e1", PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row:       fakeRow{},
			row2:      timePunchRow{p: TimePunch{EventID: "e1", PersonUUID: "p1", PunchTime: time.Unix(1, 0), PunchType: "IN", SourceProvider: "MANUAL", TransactionTime: time.Unix(2, 0)}, payload: []byte(`{}`)},
			commitErr: errors.New("commit"),
		})
		_, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{EventID: "e1", PersonUUID: "p1", PunchType: "IN"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (defaults)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row:  &stubRow{vals: []any{"e1"}},
			row2: fakeRow{},
			row3: timePunchRow{
				p: TimePunch{
					EventID:         "e1",
					PersonUUID:      "p1",
					PunchTime:       time.Date(2026, 1, 1, 8, 0, 0, 0, time.FixedZone("X", 8*60*60)),
					PunchType:       "IN",
					SourceProvider:  "MANUAL",
					TransactionTime: time.Date(2026, 1, 1, 9, 0, 0, 0, time.FixedZone("Y", -7*60*60)),
				},
				payload: []byte(`{"note":"x"}`),
			},
		})
		out, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{
			PersonUUID: "  p1 ",
			PunchTime:  time.Date(2026, 1, 1, 8, 0, 0, 0, time.FixedZone("X", 8*60*60)),
			PunchType:  " in ",
			Payload:    json.RawMessage(`{"note":"x"}`),
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if out.EventID != "e1" || out.PunchType != "IN" {
			t.Fatalf("out=%+v", out)
		}
		if out.PunchTime.Location() != time.UTC || out.TransactionTime.Location() != time.UTC {
			t.Fatalf("expected UTC times")
		}
		if string(out.Payload) != `{"note":"x"}` {
			t.Fatalf("payload=%s", string(out.Payload))
		}
	})
}

type importQueryRowTx struct {
	*stubTx
}

func (t *importQueryRowTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.rowErr != nil {
		return &stubRow{err: t.rowErr}
	}
	return fakeRow{}
}

func TestStaffingPGStore_ImportTimePunches(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id generate error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("uuid")})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{PersonUUID: "p1", PunchType: "IN"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("person uuid required", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1", PunchType: "IN"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("kernel error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("kernel")})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1", PersonUUID: "p1", PunchType: "IN"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&importQueryRowTx{stubTx: &stubTx{commitErr: errors.New("commit")}})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1", PersonUUID: "p1", PunchType: "IN"}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(&importQueryRowTx{stubTx: &stubTx{}})
		err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{EventID: "e1", PersonUUID: "p1", PunchType: "IN"}})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestAttendanceMemoryStore(t *testing.T) {
	store := newStaffingMemoryStore()

	if _, err := store.SubmitTimePunch(context.Background(), "t1", "", submitTimePunchParams{PersonUUID: "p1", PunchType: "IN"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PunchType: "IN"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1", PunchType: "BAD"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{PersonUUID: "p1", PunchType: "IN", SourceProvider: "BAD"}); err == nil {
		t.Fatal("expected error")
	}

	p1, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{
		PersonUUID: "p1",
		PunchTime:  time.Unix(100, 0).UTC(),
		PunchType:  "IN",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if p1.EventID == "" || p1.TransactionTime.IsZero() {
		t.Fatalf("p1=%+v", p1)
	}
	if string(p1.Payload) != "{}" {
		t.Fatalf("payload=%s", string(p1.Payload))
	}

	p2, err := store.SubmitTimePunch(context.Background(), "t1", "i1", submitTimePunchParams{
		EventID:        "e2",
		PersonUUID:     "p1",
		PunchTime:      time.Unix(200, 0).UTC(),
		PunchType:      "OUT",
		SourceProvider: "IMPORT",
		Payload:        json.RawMessage(`{"x":1}`),
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if p2.EventID != "e2" {
		t.Fatalf("p2=%+v", p2)
	}

	all, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(999, 0).UTC(), 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(all) != 2 || all[0].EventID != "e2" {
		t.Fatalf("all=%v", all)
	}

	limited, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(999, 0).UTC(), 1)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(limited) != 1 || limited[0].EventID != "e2" {
		t.Fatalf("limited=%v", limited)
	}

	empty, err := store.ListTimePunchesForPerson(context.Background(), "t2", "p1", time.Unix(0, 0).UTC(), time.Unix(999, 0).UTC(), 1)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil, got=%v", empty)
	}

	noSuchPerson, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p2", time.Unix(0, 0).UTC(), time.Unix(999, 0).UTC(), 1)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if noSuchPerson != nil {
		t.Fatalf("expected nil, got=%v", noSuchPerson)
	}

	_, err = store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(999, 0).UTC(), 5000)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	outOfRange1, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(999, 0).UTC(), time.Unix(1000, 0).UTC(), 200)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(outOfRange1) != 0 {
		t.Fatalf("outOfRange1=%v", outOfRange1)
	}
	outOfRange2, err := store.ListTimePunchesForPerson(context.Background(), "t1", "p1", time.Unix(0, 0).UTC(), time.Unix(50, 0).UTC(), 200)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(outOfRange2) != 0 {
		t.Fatalf("outOfRange2=%v", outOfRange2)
	}

	if err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{PersonUUID: "p1", PunchTime: time.Unix(300, 0).UTC(), PunchType: "IN"}}); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.ImportTimePunches(context.Background(), "t1", "i1", []submitTimePunchParams{{PersonUUID: "p1", PunchTime: time.Unix(0, 0).UTC(), PunchType: "BAD"}}); err == nil {
		t.Fatal("expected error")
	}

	if min(1, 2) != 1 || min(2, 1) != 1 {
		t.Fatal("min")
	}
}

func TestIsSTAFFING_IDEMPOTENCY_REUSED(t *testing.T) {
	if isSTAFFING_IDEMPOTENCY_REUSED(nil) {
		t.Fatal("expected false")
	}
	if isSTAFFING_IDEMPOTENCY_REUSED(errors.New("boom")) {
		t.Fatal("expected false")
	}
	if !isSTAFFING_IDEMPOTENCY_REUSED(errors.New("STAFFING_IDEMPOTENCY_REUSED: boom")) {
		t.Fatal("expected true")
	}
}

func TestStaffingPGStore_ListDailyAttendanceResultsForDate(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{queryErr: errors.New("query")})
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}, scanErr: errors.New("scan")}})
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}, err: errors.New("rows")}})
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}}, commitErr: errors.New("commit")})
		_, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (limit clamp + utc normalize)", func(t *testing.T) {
		firstInLocal := time.Date(2026, 1, 1, 9, 0, 0, 0, time.FixedZone("X", 8*60*60))
		lastOutLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Y", -7*60*60))
		maxID := int64(123)
		maxPunchTimeLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Z", 9*60*60))
		computedAtLocal := time.Date(2026, 1, 2, 0, 0, 0, 0, time.FixedZone("T", -8*60*60))
		createdAtLocal := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("U", 3*60*60))
		updatedAtLocal := time.Date(2026, 1, 1, 1, 0, 0, 0, time.FixedZone("V", -2*60*60))

		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{
			PersonUUID:             "p1",
			WorkDate:               "2026-01-01",
			RulesetVersion:         "R1",
			Status:                 "PRESENT",
			Flags:                  []string{"LATE"},
			FirstInTime:            &firstInLocal,
			LastOutTime:            &lastOutLocal,
			WorkedMinutes:          480,
			LateMinutes:            10,
			EarlyLeaveMinutes:      0,
			InputPunchCount:        2,
			InputMaxPunchEventDBID: &maxID,
			InputMaxPunchTime:      &maxPunchTimeLocal,
			ComputedAt:             computedAtLocal,
			CreatedAt:              createdAtLocal,
			UpdatedAt:              updatedAtLocal,
		}}}})

		out, err := store.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 0)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 1 {
			t.Fatalf("len=%d", len(out))
		}
		if out[0].FirstInTime == nil || out[0].LastOutTime == nil || out[0].InputMaxPunchTime == nil || out[0].InputMaxPunchEventDBID == nil {
			t.Fatalf("unexpected nil pointers: %+v", out[0])
		}
		if out[0].FirstInTime.Location() != time.UTC || out[0].LastOutTime.Location() != time.UTC || out[0].InputMaxPunchTime.Location() != time.UTC {
			t.Fatalf("expected UTC times")
		}
		if out[0].ComputedAt.Location() != time.UTC || out[0].CreatedAt.Location() != time.UTC || out[0].UpdatedAt.Location() != time.UTC {
			t.Fatalf("expected UTC computed/created/updated")
		}
		if *out[0].InputMaxPunchEventDBID != 123 {
			t.Fatalf("max id=%d", *out[0].InputMaxPunchEventDBID)
		}

		store2 := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1", WorkDate: "2026-01-01"}}}})
		if _, err := store2.ListDailyAttendanceResultsForDate(context.Background(), "t1", "2026-01-01", 5000); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestStaffingPGStore_GetDailyAttendanceResult(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, _, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, _, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no rows", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: pgx.ErrNoRows})
		_, ok, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("expected not ok")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{row: dailyAttendanceResultRow{scanErr: errors.New("scan")}})
		_, _, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{row: dailyAttendanceResultRow{r: DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01"}}, commitErr: errors.New("commit")})
		_, _, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (utc normalize)", func(t *testing.T) {
		firstInLocal := time.Date(2026, 1, 1, 9, 0, 0, 0, time.FixedZone("X", 8*60*60))
		lastOutLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Y", -7*60*60))
		maxID := int64(123)
		maxPunchTimeLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Z", 9*60*60))
		computedAtLocal := time.Date(2026, 1, 2, 0, 0, 0, 0, time.FixedZone("T", -8*60*60))
		store := newStaffingPGStore(&stubTx{row: dailyAttendanceResultRow{r: DailyAttendanceResult{
			PersonUUID:             "p1",
			WorkDate:               "2026-01-01",
			RulesetVersion:         "R1",
			Status:                 "PRESENT",
			Flags:                  []string{},
			FirstInTime:            &firstInLocal,
			LastOutTime:            &lastOutLocal,
			InputMaxPunchEventDBID: &maxID,
			InputMaxPunchTime:      &maxPunchTimeLocal,
			ComputedAt:             computedAtLocal,
			CreatedAt:              computedAtLocal,
			UpdatedAt:              computedAtLocal,
		}}})
		out, ok, err := store.GetDailyAttendanceResult(context.Background(), "t1", "p1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !ok {
			t.Fatal("expected ok")
		}
		if out.FirstInTime == nil || out.LastOutTime == nil || out.InputMaxPunchTime == nil || out.InputMaxPunchEventDBID == nil {
			t.Fatalf("unexpected nil pointers: %+v", out)
		}
		if out.FirstInTime.Location() != time.UTC || out.LastOutTime.Location() != time.UTC || out.InputMaxPunchTime.Location() != time.UTC {
			t.Fatalf("expected UTC pointer times")
		}
		if *out.InputMaxPunchEventDBID != 123 {
			t.Fatalf("max id=%d", *out.InputMaxPunchEventDBID)
		}
		if out.ComputedAt.Location() != time.UTC || out.CreatedAt.Location() != time.UTC || out.UpdatedAt.Location() != time.UTC {
			t.Fatalf("expected UTC timestamps")
		}
	})
}

func TestStaffingPGStore_ListDailyAttendanceResultsForPerson(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{queryErr: errors.New("query")})
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}, scanErr: errors.New("scan")}})
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}, err: errors.New("rows")}})
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1"}}}, commitErr: errors.New("commit")})
		_, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 200)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (limit clamp)", func(t *testing.T) {
		firstInLocal := time.Date(2026, 1, 1, 9, 0, 0, 0, time.FixedZone("X", 8*60*60))
		lastOutLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Y", -7*60*60))
		maxPunchTimeLocal := time.Date(2026, 1, 1, 18, 0, 0, 0, time.FixedZone("Z", 9*60*60))
		store := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{
			PersonUUID:        "p1",
			WorkDate:          "2026-01-01",
			FirstInTime:       &firstInLocal,
			LastOutTime:       &lastOutLocal,
			InputMaxPunchTime: &maxPunchTimeLocal,
			ComputedAt:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("T", -8*60*60)),
			CreatedAt:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("U", 3*60*60)),
			UpdatedAt:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("V", -2*60*60)),
		}}}})
		out, err := store.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 0)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 1 || out[0].FirstInTime == nil || out[0].InputMaxPunchTime == nil {
			t.Fatalf("out=%+v", out)
		}
		if out[0].FirstInTime.Location() != time.UTC || out[0].InputMaxPunchTime.Location() != time.UTC {
			t.Fatalf("expected UTC pointer times")
		}
		if out[0].ComputedAt.Location() != time.UTC || out[0].CreatedAt.Location() != time.UTC || out[0].UpdatedAt.Location() != time.UTC {
			t.Fatalf("expected UTC timestamps")
		}

		store2 := newStaffingPGStore(&stubTx{rows: &dailyAttendanceResultRows{rows: []DailyAttendanceResult{{PersonUUID: "p1", WorkDate: "2026-01-01"}}}})
		if _, err := store2.ListDailyAttendanceResultsForPerson(context.Background(), "t1", "p1", "2026-01-01", "2026-01-01", 5000); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}
