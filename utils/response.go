// Package utils holds the shared response envelope, error types and request binding/validation.
package utils

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestIDKey is the gin context key set by middleware.RequestID.
const RequestIDKey = "request_id"

// Response is the single JSON shape returned by every endpoint.
//
//	success: {"success":true,  "message":"...", "data":{...}, "meta":{...}, "request_id":"..."}
//	failure: {"success":false, "error":{"code":"...","message":"...","details":[...]}, "request_id":"..."}
type Response struct {
	Success   bool       `json:"success"`
	Message   string     `json:"message,omitempty"`
	Data      any        `json:"data,omitempty"`
	Meta      any        `json:"meta,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
}

type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// RequestID returns the ID of the current request ("" if the middleware isn't installed).
func RequestID(c *gin.Context) string {
	return c.GetString(RequestIDKey)
}

// Success writes a success response with any status code and a custom message.
func Success(c *gin.Context, status int, message string, data any) {
	SuccessWithMeta(c, status, message, data, nil)
}

// SuccessWithMeta is Success plus a "meta" object (pagination info, counts, ...).
func SuccessWithMeta(c *gin.Context, status int, message string, data, meta any) {
	c.JSON(status, Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Meta:      meta,
		RequestID: RequestID(c),
	})
}

func OK(c *gin.Context, message string, data any) {
	Success(c, http.StatusOK, message, data)
}

func Created(c *gin.Context, message string, data any) {
	Success(c, http.StatusCreated, message, data)
}

// Error writes an error response. Pass an *AppError for a specific status and message.
// Any other error is treated as an internal error: it is logged in full, but the
// client only receives a generic message.
func Error(c *gin.Context, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = NewInternal(err)
	}

	if appErr.Status >= http.StatusInternalServerError {
		cause := appErr.Err
		if cause == nil {
			cause = errors.New(appErr.Message)
		}
		slog.Error("request failed",
			"request_id", RequestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"code", appErr.Code,
			"error", cause,
		)
	}

	c.AbortWithStatusJSON(appErr.Status, Response{
		Success: false,
		Error: &ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
		RequestID: RequestID(c),
	})
}
