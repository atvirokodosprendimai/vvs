package domain

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidInput   = errors.New("invalid input")
	ErrForbidden      = errors.New("forbidden operation")
	ErrInvalidState   = errors.New("invalid state transition")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msg := "validation failed: "
	for i, v := range e {
		if i > 0 {
			msg += "; "
		}
		msg += v.Error()
	}
	return msg
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
