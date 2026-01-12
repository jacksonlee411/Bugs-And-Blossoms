package controllers

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsStableDBCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: "", want: false},
		{in: "UNKNOWN", want: false},
		{in: "  UNKNOWN  ", want: false},
		{in: "STAFFING_IDEMPOTENCY_REUSED", want: true},
		{in: "A", want: true},
		{in: "1ABC", want: false},
		{in: "AbC", want: false},
		{in: "ABC-def", want: false},
		{in: "A BC", want: false},
	}
	for _, tc := range cases {
		if got := isStableDBCode(tc.in); got != tc.want {
			t.Fatalf("in=%q got=%v want=%v", tc.in, got, tc.want)
		}
	}
}

func TestStablePgMessage_StableCodePassThrough(t *testing.T) {
	err := &pgconn.PgError{Message: "STAFFING_X"}
	if got := stablePgMessage(err); got != "STAFFING_X" {
		t.Fatalf("got=%q want=%q", got, "STAFFING_X")
	}
}

func TestStablePgMessage_KnownConstraintsAreMappedToStableCodes(t *testing.T) {
	{
		err := &pgconn.PgError{
			Message:          `conflicting key value violates exclusion constraint "assignment_versions_position_no_overlap"`,
			ConstraintName:   "assignment_versions_position_no_overlap",
			Code:             "23P01",
			Where:            "replay_assignment_versions",
			SchemaName:       "staffing",
			TableName:        "assignment_versions",
			ColumnName:       "",
			DataTypeName:     "",
			InternalQuery:    "",
			InternalPosition: 0,
		}
		if got := stablePgMessage(err); got != "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF" {
			t.Fatalf("got=%q want=%q", got, "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF")
		}
	}

	{
		err := &pgconn.PgError{
			Message:        `duplicate key value violates unique constraint "assignment_events_one_per_day_unique"`,
			ConstraintName: "assignment_events_one_per_day_unique",
			Code:           "23505",
		}
		if got := stablePgMessage(err); got != "STAFFING_ASSIGNMENT_ONE_PER_DAY" {
			t.Fatalf("got=%q want=%q", got, "STAFFING_ASSIGNMENT_ONE_PER_DAY")
		}
	}
}

func TestStablePgMessage_FallbackToErrorString(t *testing.T) {
	err := &pgconn.PgError{
		Message:        "some pg error",
		ConstraintName: "unknown_constraint",
	}
	if got := stablePgMessage(err); got != err.Error() {
		t.Fatalf("got=%q want=%q", got, err.Error())
	}
}
