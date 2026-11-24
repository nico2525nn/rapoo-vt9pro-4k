package rapoo

import "errors"

// ErrDeadlineExceeded is returned when a multi-step device operation is about
// to continue after the caller's deadline. The exchange that already completed
// may have changed the device, so callers must treat the result as partial.
var ErrDeadlineExceeded = errors.New("rapoo: operation deadline exceeded")

// PartialError describes a multi-step write that failed after at least one
// step had already been applied. Applied lists the steps that completed.
type PartialError struct {
	Applied []string
	Err     error
}

func (e *PartialError) Error() string {
	if e == nil || e.Err == nil {
		return "rapoo: partial write"
	}
	return e.Err.Error()
}

func (e *PartialError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
