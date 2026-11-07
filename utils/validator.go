package utils

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var initOnce sync.Once

// initValidator configures Gin's built-in validator (go-playground/validator) once:
//   - error messages use the JSON field name ("title"), not the Go name ("Title")
//   - custom rules: notblank, notfuture
func initValidator() {
	initOnce.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			panic("utils: gin validator engine is not *validator.Validate")
		}

		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			for _, key := range []string{"json", "form", "uri"} {
				name := strings.SplitN(fld.Tag.Get(key), ",", 2)[0]
				if name != "" && name != "-" {
					return name
				}
			}
			return fld.Name
		})

		// notblank: not empty and not only whitespace.
		_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
			return strings.TrimSpace(fl.Field().String()) != ""
		})

		// notfuture: a year no later than next year.
		_ = v.RegisterValidation("notfuture", func(fl validator.FieldLevel) bool {
			return fl.Field().Int() <= int64(time.Now().Year()+1)
		})
	})
}

// mapValidationError turns validator output into our 422 error with per-field messages.
func mapValidationError(ves validator.ValidationErrors) *AppError {
	details := make([]ErrorDetail, 0, len(ves))
	for _, fe := range ves {
		details = append(details, ErrorDetail{Field: fe.Field(), Message: messageFor(fe)})
	}
	return NewValidationError(details)
}

// messageFor produces a human-readable message for one failed rule.
// Add a case here whenever you start using a new validation tag.
func messageFor(fe validator.FieldError) string {
	f := fe.Field()
	isString := fe.Kind() == reflect.String

	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", f)
	case "notblank":
		return fmt.Sprintf("%s must not be blank", f)
	case "min":
		if isString {
			return fmt.Sprintf("%s must be at least %s characters long", f, fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", f, fe.Param())
	case "max":
		if isString {
			return fmt.Sprintf("%s must be at most %s characters long", f, fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", f, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", f, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", f, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", f, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", f, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", f, fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", f)
	case "notfuture":
		return fmt.Sprintf("%s cannot be in the future", f)
	default:
		return fmt.Sprintf("%s is invalid", f)
	}
}
