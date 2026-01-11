package httperr

import "errors"

type BadRequestError struct {
	msg string
}

func (e *BadRequestError) Error() string { return e.msg }

func NewBadRequest(msg string) error { return &BadRequestError{msg: msg} }

func IsBadRequest(err error) bool {
	var badReq *BadRequestError
	return errors.As(err, &badReq) && badReq != nil
}
