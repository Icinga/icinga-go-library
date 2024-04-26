package logging

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type errNoStackTrace struct {
	e error
}

func (e errNoStackTrace) Error() string {
	return e.e.Error()
}

func Error(e error) zap.Field {
	if _, ok := e.(stackTracer); ok {
		return zap.Error(errNoStackTrace{e})
	}

	return zap.Error(e)
}
