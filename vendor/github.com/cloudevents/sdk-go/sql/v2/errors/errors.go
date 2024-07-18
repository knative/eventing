/*
 Copyright 2024 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package errors

import (
	"fmt"
	"strings"
)

type errorKind int

const (
	parseError = iota
	mathError
	castError
	missingAttributeError
	missingFunctionError
	functionEvaluationError
	dummyLastError // always add new error classes ABOVE this error
)

type cesqlError struct {
	kind    errorKind
	message string
}

func (eKind errorKind) String() string {
	switch eKind {
	case parseError:
		return "parse error"
	case mathError:
		return "math error"
	case castError:
		return "cast error"
	case missingAttributeError:
		return "missing attribute error"
	case missingFunctionError:
		return "missing function error"
	case functionEvaluationError:
		return "function evaluation error"
	default:
		return "generic error"
	}

}

func (cerr cesqlError) Error() string {
	return fmt.Sprintf("%s: %s", cerr.kind, cerr.message)
}

func NewParseError(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	errorMessages := make([]string, 0, len(errs))
	for _, err := range errs {
		errorMessages = append(errorMessages, err.Error())
	}

	return cesqlError{
		kind:    parseError,
		message: strings.Join(errorMessages, "|"),
	}
}

func IsParseError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == parseError
	}
	return false
}

func IsMathError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == mathError
	}
	return false
}

func NewMathError(message string) error {
	return cesqlError{
		kind:    mathError,
		message: message,
	}
}

func IsCastError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == castError
	}
	return false
}

func NewCastError(err error) error {
	return cesqlError{
		kind:    castError,
		message: err.Error(),
	}
}

func IsMissingAttributeError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == missingAttributeError
	}
	return false
}

func NewMissingAttributeError(attribute string) error {
	return cesqlError{
		kind:    missingAttributeError,
		message: attribute,
	}
}

func IsMissingFunctionError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == missingFunctionError
	}
	return false
}

func NewMissingFunctionError(function string) error {
	return cesqlError{
		kind:    missingFunctionError,
		message: function,
	}
}

func IsFunctionEvaluationError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind == functionEvaluationError
	}
	return false
}

func NewFunctionEvaluationError(err error) error {
	return cesqlError{
		kind:    functionEvaluationError,
		message: err.Error(),
	}
}

func IsGenericError(err error) bool {
	if cesqlErr, ok := err.(cesqlError); ok {
		return cesqlErr.kind < 0 || cesqlErr.kind >= dummyLastError
	}
	return false
}
