package validation

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func New() *validator.Validate { return validator.New() }

func Errors(err error) []FieldError {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []FieldError{{Message: "invalid request"}}
	}
	result := make([]FieldError, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		result = append(result, FieldError{Field: strings.ToLower(fieldErr.Field()), Message: "failed " + fieldErr.Tag() + " validation"})
	}
	return result
}
