package httperr

import "errors"

type BadRequestError struct {
	msg string
}

func (e *BadRequestError) Error() string { return e.msg }

func NewBadRequest(msg string) error { return &BadRequestError{msg: msg} }

func IsBadRequest(err error) bool {
	_, ok := errors.AsType[*BadRequestError](err)
	return ok
}
