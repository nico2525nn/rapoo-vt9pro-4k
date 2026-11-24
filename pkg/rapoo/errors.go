package rapoo

import "errors"

// ErrDeadlineExceeded is returned when a multi-step device operation is about
// to continue after the caller's deadline. The exchange that already completed
// may have changed the device, so callers must treat the result as partial.
var ErrDeadlineExceeded = errors.New("rapoo: operation deadline exceeded")

// ErrResultUnknown means the command was sent, but the reply was not observed.
// The write may have been applied; callers must refresh rather than retry.
var ErrResultUnknown = errors.New("rapoo: command sent but result unknown")

// ErrAmbiguousDevice means multiple identical devices are attached and the OS
// did not expose a stable container identity, so command/report endpoints
// cannot be paired safely.
var ErrAmbiguousDevice = errors.New("rapoo: multiple identical devices cannot be distinguished")

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
