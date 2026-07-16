package apperror

import "net/http"

type Error struct {
	Code    string
	Message string
	Status  int
	Err     error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Err }

func New(status int, code, message string, err error) *Error {
	return &Error{Status: status, Code: code, Message: message, Err: err}
}
func BadRequest(message string, err error) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message, err)
}
func NotFound(resource string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", resource+" not found", nil)
}
