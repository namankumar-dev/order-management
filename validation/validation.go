package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"order-management/apperror"
)

func Register() {
	binding.EnableDecoderDisallowUnknownFields = true
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
			if name == "" {
				name = strings.SplitN(f.Tag.Get("form"), ",", 2)[0]
			}
			if name == "" || name == "-" {
				return f.Name
			}
			return name
		})
	}
}

func Translate(err error) *apperror.Error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make(map[string]string, len(ve))
		for _, fe := range ve {
			details[fe.Field()] = message(fe)
		}
		return apperror.Validation(details)
	}
	return apperror.BadRequest("malformed request body").WithCause(err)
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be at least " + fe.Param()
	case "lte":
		return "must be at most " + fe.Param()
	case "min":
		return "must have at least " + fe.Param() + " characters"
	case "max":
		return "must have at most " + fe.Param() + " characters"
	case "email":
		return "must be a valid email address"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	default:
		return "is invalid"
	}
}
