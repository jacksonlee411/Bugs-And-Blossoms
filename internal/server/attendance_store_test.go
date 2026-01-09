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
