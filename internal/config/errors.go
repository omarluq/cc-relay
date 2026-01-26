// Package config provides configuration loading, parsing, and validation for cc-relay.
package config

import (
	"fmt"
	"strings"
)

// ValidationError collects multiple validation errors.
// It implements the error interface and provides detailed error messages.
type ValidationError struct {
	Errors []string
}

// Error implements the error interface.
// Returns all validation errors as a formatted string.
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "config validation failed"
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("config validation failed: %s", e.Errors[0])
	}
	return fmt.Sprintf("config validation failed with %d errors:\n  - %s",
		len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// Addf appends a formatted error message to the validation errors.
func (e *ValidationError) Addf(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

// Add appends an error message to the validation errors.
func (e *ValidationError) Add(msg string) {
	e.Errors = append(e.Errors, msg)
}

// HasErrors returns true if there are any validation errors.
func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ToError returns the ValidationError as an error if there are errors, otherwise nil.
func (e *ValidationError) ToError() error {
	if e.HasErrors() {
		return e
	}
	return nil
}
