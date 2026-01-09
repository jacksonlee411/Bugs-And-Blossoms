package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type timeBankCycleRow struct {
	scanErr                error
	cycle                  TimeBankCycle
	inputMaxPunchEventDBID *int64
	inputMaxPunchTime      *time.Time
}

func (r timeBankCycleRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}

	*(dest[0].(*string)) = r.cycle.PersonUUID
	*(dest[1].(*string)) = r.cycle.CycleType
	*(dest[2].(*string)) = r.cycle.CycleStartDate
	*(dest[3].(*string)) = r.cycle.CycleEndDate
	*(dest[4].(*string)) = r.cycle.RulesetVersion
	*(dest[5].(*int)) = r.cycle.WorkedMinutesTotal
	*(dest[6].(*int)) = r.cycle.OvertimeMinutes150
	*(dest[7].(*int)) = r.cycle.OvertimeMinutes200
	*(dest[8].(*int)) = r.cycle.OvertimeMinutes300
	*(dest[9].(*int)) = r.cycle.CompEarnedMinutes
	*(dest[10].(*int)) = r.cycle.CompUsedMinutes
	*(dest[11].(**int64)) = r.inputMaxPunchEventDBID
	*(dest[12].(**time.Time)) = r.inputMaxPunchTime
	*(dest[13].(*time.Time)) = r.cycle.ComputedAt
	*(dest[14].(*time.Time)) = r.cycle.CreatedAt
	*(dest[15].(*time.Time)) = r.cycle.UpdatedAt
	return nil
}

func TestStaffingPGStore_GetTimeBankCycleForMonth(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))

		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("person_uuid required", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", " ", "2026-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("month required", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", " ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("month invalid", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "BAD")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: pgx.ErrNoRows})
		_, found, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if found {
			t.Fatal("expected not found")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("boom")})
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row: timeBankCycleRow{
				cycle: TimeBankCycle{
					PersonUUID:     "p1",
					CycleType:      "MONTH",
					CycleStartDate: "2026-01-01",
					CycleEndDate:   "2026-01-31",
					RulesetVersion: "v1",
					ComputedAt:     time.Unix(1, 0).UTC(),
					CreatedAt:      time.Unix(2, 0).UTC(),
					UpdatedAt:      time.Unix(3, 0).UTC(),
				},
			},
			commitErr: errors.New("commit"),
		}
		store := newStaffingPGStore(tx)
		_, _, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		tmLocal := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("Z", 8*60*60))
		inputTimeLocal := time.Date(2026, 1, 2, 8, 0, 0, 0, time.FixedZone("Y", -7*60*60))
		inputMaxPunchEventDBID := int64(123)

		tx := &stubTx{
			row: timeBankCycleRow{
				cycle: TimeBankCycle{
					PersonUUID:         "p1",
					CycleType:          "MONTH",
					CycleStartDate:     "2026-01-01",
					CycleEndDate:       "2026-01-31",
					RulesetVersion:     "v1",
					WorkedMinutesTotal: 480,
					OvertimeMinutes200: 120,
					CompEarnedMinutes:  120,
					ComputedAt:         tmLocal,
					CreatedAt:          tmLocal,
					UpdatedAt:          tmLocal,
				},
				inputMaxPunchEventDBID: &inputMaxPunchEventDBID,
				inputMaxPunchTime:      &inputTimeLocal,
			},
		}

		store := newStaffingPGStore(tx)
		out, found, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if out.InputMaxPunchEventDBID == nil || *out.InputMaxPunchEventDBID != inputMaxPunchEventDBID {
			t.Fatalf("unexpected input max punch db id: %#v", out.InputMaxPunchEventDBID)
		}
		if out.InputMaxPunchTime == nil || out.InputMaxPunchTime.Location() != time.UTC {
			t.Fatalf("expected input max punch time in UTC: %#v", out.InputMaxPunchTime)
		}
		if out.ComputedAt.Location() != time.UTC || out.CreatedAt.Location() != time.UTC || out.UpdatedAt.Location() != time.UTC {
			t.Fatalf("expected UTC timestamps")
		}
	})
}

func TestStaffingMemoryStore_GetTimeBankCycleForMonth(t *testing.T) {
	store := newStaffingMemoryStore()
	_, found, err := store.GetTimeBankCycleForMonth(context.Background(), "t1", "p1", "2026-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}
