package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const dayLayout = "2006-01-02"

type dayFieldValidationError struct {
	Field   string
	Missing bool
}

func (e dayFieldValidationError) Error() string {
	field := strings.TrimSpace(e.Field)
	if e.Missing {
		return field + " required"
	}
	return "invalid " + field
}

func parseRequiredDay(raw string, field string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", dayFieldValidationError{Field: field, Missing: true}
	}
	if _, err := time.Parse(dayLayout, value); err != nil {
		return "", dayFieldValidationError{Field: field}
	}
	return value, nil
}

func parseRequiredQueryDay(r *http.Request, field string) (string, error) {
	return parseRequiredDay(r.URL.Query().Get(field), field)
}

func dayFieldErrorDetails(err error) (string, string, bool) {
	var dayErr dayFieldValidationError
	if !errors.As(err, &dayErr) {
		return "", "", false
	}

	code := "invalid_" + strings.TrimSpace(dayErr.Field)
	switch strings.TrimSpace(dayErr.Field) {
	case "as_of":
		code = "invalid_as_of"
	case "effective_date":
		code = "invalid_effective_date"
	}
	return code, dayErr.Error(), true
}

func writeInternalDayFieldError(w http.ResponseWriter, r *http.Request, err error) bool {
	code, message, ok := dayFieldErrorDetails(err)
	if !ok {
		return false
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, code, message)
	return true
}
