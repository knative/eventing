package ensure

import (
	"fmt"
	"github.com/pkg/errors"
	"regexp"
)

// NoError will panic if given an error, as it was unexpected
func NoError(err error) {
	if err != nil {
		panic(errors.WithMessage(err, "unexpected error"))
	}
}

// Error will panic if given no error, as it expected one
func Error(err error) {
	if err == nil {
		panic(errors.New("expecting error, but none given"))
	}
}

// ErrorWithMessage will panic if given no error, or error message don't match provided regexp
func ErrorWithMessage(err error, messageRegexp string) {
	Error(err)
	validErrorMessage := regexp.MustCompile(messageRegexp)
	if !validErrorMessage.MatchString(err.Error()) {
		panic(errors.WithMessage(
			err,
			fmt.Sprintf("given error doesn't match given regexp (%s)", messageRegexp),
		))
	}
}
