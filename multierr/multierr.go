package multierr

import (
	"bytes"
	"sync"
)

// TODO(el): Docs, Tests. Annotate errs with stack?

func Combine(errs ...error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}

	return &multierr{errors: errs, sep: []byte("; ")}
}

func Wrap(err, w error) error {
	switch {
	case err == nil:
		return w
	case w == nil:
		return err
	}

	// TODO(el): Consider special type implementing Cause().
	return &multierr{errors: []error{w, err}, sep: []byte(": ")}
}

// buffers is a pool of bytes.Buffers.
var buffers = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

type multierr struct {
	errors []error
	sep    []byte
}

func (e *multierr) Error() string {
	buf := buffers.Get().(*bytes.Buffer)
	defer buffers.Put(buf)

	buf.WriteString(e.errors[0].Error())

	for _, err := range e.errors[1:] {
		buf.Write(e.sep)
		buf.WriteString(err.Error())
	}

	return buf.String()
}

func (e *multierr) Unwrap() []error {
	return e.errors
}
