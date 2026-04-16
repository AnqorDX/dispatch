package dispatch

import "errors"

// ErrEventNotFound is returned by Subscribe when the named event has not
// been declared on this bus.
var ErrEventNotFound = errors.New("dispatch: event not found")
