package utils

import (
	"fmt"
	"net/http"
)

// Machine-readable error codes. Clients should switch on these, not on message text.
const (
	CodeBadRequest           = "BAD_REQUEST"
	CodeInvalidJSON          = "INVALID_JSON"
	CodeValidation           = "VALIDATION_ERROR"
	CodeNotFound             = "NOT_FOUND"
	CodeMethodNotAllowed     = "METHOD_NOT_ALLOWED"
	CodeConflict             = "CONFLICT"
	CodePayloadTooLarge      = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"
	CodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
	CodeInternal             = "INTERNAL_ERROR"
)

// ErrorDetail describes one problem, usually one invalid field.
type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// AppError is an error that knows its own HTTP status and client-facing message.
// Err holds the internal cause (DB error, etc.). It is logged but NEVER sent to clients.
type AppError struct {
	Status  int
	Code    string
	Message string
	Details []ErrorDetail
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// NewAppError builds a custom error, for cases the helpers below don't cover.
func NewAppError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func NewBadRequest(message string) *AppError {
	return NewAppError(http.StatusBadRequest, CodeBadRequest, message)
}

func NewInvalidJSON(message string) *AppError {
	return NewAppError(http.StatusBadRequest, CodeInvalidJSON, message)
}

// NewValidationError returns 422 with one entry per invalid field.
func NewValidationError(details []ErrorDetail) *AppError {
	return &AppError{
		Status:  http.StatusUnprocessableEntity,
		Code:    CodeValidation,
		Message: "Validation failed",
		Details: details,
	}
}

func NewNotFound(message string) *AppError {
	return NewAppError(http.StatusNotFound, CodeNotFound, message)
}

func NewMethodNotAllowed() *AppError {
	return NewAppError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, "Method not allowed for this route")
}

func NewConflict(message string) *AppError {
	return NewAppError(http.StatusConflict, CodeConflict, message)
}

func NewPayloadTooLarge(message string) *AppError {
	return NewAppError(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, message)
}

func NewUnsupportedMediaType(message string) *AppError {
	return NewAppError(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, message)
}

func NewServiceUnavailable(message string, err error) *AppError {
	return &AppError{Status: http.StatusServiceUnavailable, Code: CodeServiceUnavailable, Message: message, Err: err}
}

// NewInternal wraps an unexpected error. Clients only ever see the generic message.
func NewInternal(err error) *AppError {
	return &AppError{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "An unexpected error occurred. Please try again later.",
		Err:     err,
	}
}
