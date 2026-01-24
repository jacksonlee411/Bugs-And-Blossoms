package server

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBadRequestHelpers(t *testing.T) {
	err := newBadRequestError("bad request")
	if !isBadRequestError(err) {
		t.Fatal("expected bad request error")
	}
}

func TestPgErrorMessage(t *testing.T) {
	if got := pgErrorMessage(&pgconn.PgError{Message: "  bad  "}); got != "bad" {
		t.Fatalf("msg=%q", got)
	}
	if got := pgErrorMessage(&pgconn.PgError{Message: "   "}); got != "UNKNOWN" {
		t.Fatalf("empty msg=%q", got)
	}
	if got := pgErrorMessage(errors.New("boom")); got != "UNKNOWN" {
		t.Fatalf("non-pg msg=%q", got)
	}
}

func TestPgErrorCode(t *testing.T) {
	if got := pgErrorCode(&pgconn.PgError{Code: " 22P02 "}); got != "22P02" {
		t.Fatalf("code=%q", got)
	}
	if got := pgErrorCode(errors.New("boom")); got != "" {
		t.Fatalf("non-pg code=%q", got)
	}
}

func TestIsPgInvalidInput(t *testing.T) {
	for _, code := range []string{"22P02", "22003", "22007", "22008"} {
		if !isPgInvalidInput(&pgconn.PgError{Code: code}) {
			t.Fatalf("expected true for %s", code)
		}
	}
	if isPgInvalidInput(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected false for unrelated code")
	}
	if isPgInvalidInput(errors.New("boom")) {
		t.Fatal("expected false for non-pg error")
	}
}

func TestStablePgMessage(t *testing.T) {
	if got := stablePgMessage(&pgconn.PgError{Message: "STAFFING_ASSIGNMENT_ONE_PER_DAY"}); got != "STAFFING_ASSIGNMENT_ONE_PER_DAY" {
		t.Fatalf("stable msg=%q", got)
	}
	if got := stablePgMessage(&pgconn.PgError{ConstraintName: "assignment_versions_position_no_overlap"}); got != "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF" {
		t.Fatalf("constraint msg=%q", got)
	}
	if got := stablePgMessage(&pgconn.PgError{ConstraintName: "assignment_events_one_per_day_unique"}); got != "STAFFING_ASSIGNMENT_ONE_PER_DAY" {
		t.Fatalf("constraint msg=%q", got)
	}
	if got := stablePgMessage(errors.New("boom")); got != "boom" {
		t.Fatalf("fallback msg=%q", got)
	}
}

func TestIsStableDBCode(t *testing.T) {
	if isStableDBCode("") {
		t.Fatal("expected false for empty")
	}
	if isStableDBCode("UNKNOWN") {
		t.Fatal("expected false for UNKNOWN")
	}
	if !isStableDBCode("STAFFING_ASSIGNMENT_ONE_PER_DAY") {
		t.Fatal("expected true for stable code")
	}
	if !isStableDBCode("CODE_123") {
		t.Fatal("expected true for digits")
	}
	if isStableDBCode("bad-code") {
		t.Fatal("expected false for invalid code")
	}
}
