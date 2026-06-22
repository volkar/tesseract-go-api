package validator

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var slugRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var reservedWords = map[string]struct{}{
	"superadmin": {},
	"admin":      {},
	"root":       {},
	"owner":      {},
	"dashboard":  {},
	"settings":   {},
	"albums":     {},
}

func New() *validator.Validate {
	v := validator.New()

	// Returns json tag name for struct fields
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Reserved words (black list)
	v.RegisterValidation("notreserved", func(fl validator.FieldLevel) bool {
		_, ok := reservedWords[fl.Field().String()]
		return !ok
	})

	// Slug rule
	v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return slugRegex.MatchString(fl.Field().String())
	})

	return v
}
