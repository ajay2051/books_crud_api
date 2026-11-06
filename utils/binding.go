package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

const maxBodyBytes = 1 << 20 // 1 MiB request body limit

// Normalizer can be implemented by request structs to clean their data
// (trim spaces, lowercase emails...) BEFORE validation runs.
type Normalizer interface {
	Normalize()
}

// BindJSON strictly decodes the request body into dst (a pointer to a struct) and validates it.
// It rejects: wrong Content-Type, oversized bodies, empty or malformed JSON, unknown fields,
// wrong field types, trailing data, and anything failing the `binding:"..."` tags.
// The returned error is always an *AppError ready for utils.Error.
func BindJSON(c *gin.Context, dst any) error {
	initValidator()

	if c.ContentType() != "application/json" {
		return NewUnsupportedMediaType("Content-Type must be application/json")
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return mapJSONError(err)
	}
	// Reject trailing data such as {"a":1}{"b":2}.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return NewInvalidJSON("Request body must contain a single JSON object")
	}

	if n, ok := dst.(Normalizer); ok {
		n.Normalize()
	}
	return validateStruct(dst)
}

// BindQuery binds and validates URL query parameters (?limit=10&offset=0).
func BindQuery(c *gin.Context, dst any) error {
	initValidator()

	if err := c.ShouldBindQuery(dst); err != nil {
		var ves validator.ValidationErrors
		if errors.As(err, &ves) {
			return mapValidationError(ves)
		}
		// Type-conversion failures, e.g. ?limit=abc
		return &AppError{
			Status:  http.StatusBadRequest,
			Code:    CodeBadRequest,
			Message: "One or more query parameters have an invalid value",
			Err:     err,
		}
	}
	return nil
}

func validateStruct(obj any) error {
	if err := binding.Validator.ValidateStruct(obj); err != nil {
		var ves validator.ValidationErrors
		if errors.As(err, &ves) {
			return mapValidationError(ves)
		}
		return NewInternal(err)
	}
	return nil
}

// mapJSONError converts encoding/json errors into precise, client-friendly errors.
func mapJSONError(err error) *AppError {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		tooLarge  *http.MaxBytesError
	)

	switch {
	case errors.Is(err, io.EOF):
		return NewInvalidJSON("Request body must not be empty")

	case errors.As(err, &syntaxErr):
		return NewInvalidJSON(fmt.Sprintf("Malformed JSON at character %d", syntaxErr.Offset))

	case errors.Is(err, io.ErrUnexpectedEOF):
		return NewInvalidJSON("Malformed JSON: the body ended unexpectedly")

	case errors.As(err, &tooLarge):
		return NewPayloadTooLarge(fmt.Sprintf("Request body must not exceed %d bytes", tooLarge.Limit))

	case errors.As(err, &typeErr):
		if typeErr.Field == "" {
			return NewInvalidJSON("Request body must be a JSON object")
		}
		return NewValidationError([]ErrorDetail{{
			Field:   typeErr.Field,
			Message: fmt.Sprintf("%s must be %s", typeErr.Field, jsonTypeName(typeErr.Type)),
		}})

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return NewValidationError([]ErrorDetail{{Field: field, Message: "unknown field"}})

	default:
		return NewInvalidJSON("Invalid JSON body")
	}
}

func jsonTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Bool:
		return "a boolean"
	default:
		return "of type " + t.String()
	}
}
