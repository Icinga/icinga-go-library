package logging

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// stackTracer is an interface used to identify errors that include a stack trace.
// This interface specifically targets errors created using the github.com/pkg/errors library,
// which can add stack traces to errors with functions like errors.Wrap().
type stackTracer interface {
	StackTrace() errors.StackTrace
}

// errNoStackTrace is a wrapper for errors that implements the error interface without exposing a stack trace.
type errNoStackTrace struct {
	e error
}

// Error returns the error message of the wrapped error.
func (e errNoStackTrace) Error() string {
	return e.e.Error()
}

// Error returns a zap.Field for logging the provided error.
// This function checks if the error includes a stack trace from the pkg/errors library.
// If a stack trace is present, it is suppressed in the log output because
// logging a stack trace is not necessary. Otherwise, the error is logged normally.
func Error(e error) zap.Field {
	if _, ok := e.(stackTracer); ok {
		return zap.Error(errNoStackTrace{e})
	}

	return zap.Error(e)
}
