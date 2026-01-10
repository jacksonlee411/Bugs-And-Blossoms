package server

import (
	"context"
	"testing"
	"time"
)

type dummyTimePunchStore struct{}

func (dummyTimePunchStore) ListTimePunchesForPerson(context.Context, string, string, time.Time, time.Time, int) ([]TimePunch, error) {
	return nil, nil
}
func (dummyTimePunchStore) SubmitTimePunch(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
	return TimePunch{}, nil
}
func (dummyTimePunchStore) ImportTimePunches(context.Context, string, string, []submitTimePunchParams) error {
	return nil
}

type dummyDailyAttendanceResultStore struct{}

func (dummyDailyAttendanceResultStore) ListDailyAttendanceResultsForDate(context.Context, string, string, int) ([]DailyAttendanceResult, error) {
	return nil, nil
}
func (dummyDailyAttendanceResultStore) GetDailyAttendanceResult(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
	return DailyAttendanceResult{}, false, nil
}
func (dummyDailyAttendanceResultStore) ListDailyAttendanceResultsForPerson(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
	return nil, nil
}
func (dummyDailyAttendanceResultStore) GetAttendanceTimeProfileAndPunchesForWorkDate(context.Context, string, string, string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
	return AttendanceTimeProfileForWorkDate{}, nil, nil
}
func (dummyDailyAttendanceResultStore) ListAttendanceRecalcEventsForWorkDate(context.Context, string, string, string, int) ([]AttendanceRecalcEvent, error) {
	return nil, nil
}
func (dummyDailyAttendanceResultStore) SubmitTimePunchVoid(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
	return TimePunchVoidResult{}, nil
}
func (dummyDailyAttendanceResultStore) SubmitAttendanceRecalc(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
	return AttendanceRecalcResult{}, nil
}

func TestNewHandlerWithOptions_MissingAttendanceConfigStore(t *testing.T) {
	_, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: newStaticTenancyResolver(map[string]Tenant{
			"localhost": {ID: "t1", Domain: "localhost", Name: "T"},
		}),
		OrgUnitStore:                newOrgUnitMemoryStore(),
		JobCatalogStore:             newJobCatalogMemoryStore(),
		PersonStore:                 newPersonMemoryStore(),
		AttendanceStore:             dummyTimePunchStore{},
		AttendanceDailyResultsStore: dummyDailyAttendanceResultStore{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_AttendanceConfigStoreFromDailyResultsStore(t *testing.T) {
	_, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: newStaticTenancyResolver(map[string]Tenant{
			"localhost": {ID: "t1", Domain: "localhost", Name: "T"},
		}),
		OrgUnitStore:                newOrgUnitMemoryStore(),
		JobCatalogStore:             newJobCatalogMemoryStore(),
		PersonStore:                 newPersonMemoryStore(),
		AttendanceStore:             dummyTimePunchStore{},
		AttendanceDailyResultsStore: newStaffingMemoryStore(),
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}
