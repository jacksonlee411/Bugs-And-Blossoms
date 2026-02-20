package server

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

func newBadRequestError(msg string) error {
	return httperr.NewBadRequest(msg)
}

func isBadRequestError(err error) bool {
	return httperr.IsBadRequest(err)
}

func pgErrorMessage(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		msg := strings.TrimSpace(pgErr.Message)
		if msg != "" {
			return msg
		}
	}
	return "UNKNOWN"
}

func pgErrorCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		return strings.TrimSpace(pgErr.Code)
	}
	return ""
}

func isPgInvalidInput(err error) bool {
	switch pgErrorCode(err) {
	case "22P02", "22003", "22007", "22008":
		return true
	default:
		return false
	}
}

func stablePgMessage(err error) string {
	msg := pgErrorMessage(err)
	if isStableDBCode(msg) {
		return msg
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		switch strings.TrimSpace(pgErr.ConstraintName) {
		case "assignment_versions_position_no_overlap":
			return "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF"
		case "assignment_events_one_per_day_unique":
			return "STAFFING_ASSIGNMENT_ONE_PER_DAY"
		}
	}
	return err.Error()
}

func isStableDBCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || code == "UNKNOWN" {
		return false
	}
	for i := 0; i < len(code); i++ {
		ch := code[i]
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '_' {
			continue
		}
		return false
	}
	return true
}
