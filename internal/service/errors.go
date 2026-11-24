package service

import (
	"errors"
	"fmt"

	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

// ErrDeviceChanged means the HID target changed between the UI action and the
// write. No write was sent for the expectation that failed.
var ErrDeviceChanged = errors.New("service: device changed")

// ErrBusy means the connection lock is held by an initial poll or another
// operation. The caller should retry shortly instead of waiting for a timeout.
var ErrBusy = errors.New("service: device busy")

// ErrDeadlineExceeded means the request was queued or running past its
// deadline. A multi-step operation may already have applied some steps.
// It aliases the rapoo sentinel so service and client paths share one identity.
var ErrDeadlineExceeded = rapoo.ErrDeadlineExceeded

// PartialWriteError reports which write steps completed before a failure.
type PartialWriteError struct {
	Applied []string
	Err     error
}

func (e *PartialWriteError) Error() string {
	if e == nil {
		return "service: partial write"
	}
	if len(e.Applied) == 0 {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s (applied: %v)", e.Err.Error(), e.Applied)
}

func (e *PartialWriteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
