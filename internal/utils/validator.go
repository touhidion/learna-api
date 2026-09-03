package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// RegisterValidators teaches gin's validator to report the JSON field name
// rather than the Go struct field, so error messages match the request the
// client actually sent.
func RegisterValidators() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// BindJSON decodes and validates the request body into dst, converting any
// failure into an APIError with per-field detail.
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return bindingError(err)
	}
	return nil
}

// BindQuery does the same for query-string parameters.
func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		return bindingError(err)
	}
	return nil
}

// bindingError maps the three failure shapes gin produces onto APIError:
// a validator failure (per-field), a JSON type mismatch, and everything else.
func bindingError(err error) error {
	var vErrs validator.ValidationErrors
	if errors.As(err, &vErrs) {
		fields := make([]FieldError, 0, len(vErrs))
		for _, fe := range vErrs {
			fields = append(fields, FieldError{
				Field:   fe.Field(),
				Message: validationMessage(fe),
			})
		}
		return ErrValidation("The request contains invalid fields.").WithFields(fields...)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return ErrBadRequest("Field %q must be a %s.", typeErr.Field, typeErr.Type.String())
	}

	return ErrBadRequest("Request body is not valid JSON.").WithCause(err)
}

// validationMessage renders one validator tag as a sentence a user can act on.
func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required."
	case "email":
		return "Must be a valid email address."
	case "min":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("Must be at least %s characters.", fe.Param())
		}
		return fmt.Sprintf("Must be at least %s.", fe.Param())
	case "max":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("Must be at most %s characters.", fe.Param())
		}
		return fmt.Sprintf("Must be at most %s.", fe.Param())
	case "oneof":
		return fmt.Sprintf("Must be one of: %s.", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "uuid", "uuid4":
		return "Must be a valid UUID."
	case "url":
		return "Must be a valid URL."
	case "eqfield":
		return fmt.Sprintf("Must match %s.", fe.Param())
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s.", fe.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s.", fe.Param())
	default:
		return fmt.Sprintf("Failed the %q rule.", fe.Tag())
	}
}

// ParseUUIDParam reads a path parameter as a UUID, returning a 400 rather than
// letting a malformed id reach the database.
func ParseUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	raw := c.Param(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrBadRequest("Path parameter %q must be a valid UUID.", name).WithCause(err)
	}
	return id, nil
}
