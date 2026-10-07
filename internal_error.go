// Go only.

package csstree

import (
	"fmt"
	"runtime/debug"
)

// InternalError is returned by Parse and Generate (and their Syntax
// methods) when a parse or generate function, a callback of the options or
// an extension panics with anything other than the syntax errors the
// parser recovers from: a bug, which the parser reports instead of hiding
// it behind a Raw fallback. errors.Is and errors.As see the panic value
// when it is an error.
type InternalError struct {
	// Value is the value passed to panic.
	Value any
	// Stack is the stack trace of the goroutine that panicked, as
	// runtime/debug.Stack formats it.
	Stack []byte
}

// Error returns "csstree: internal error: " and the panic value.
func (e *InternalError) Error() string {
	return fmt.Sprintf("csstree: internal error: %v", e.Value)
}

// Unwrap returns the panic value when it is an error, and nil otherwise.
func (e *InternalError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// newInternalError returns the InternalError for the recovered panic value
// r. It must be called by the deferred function that recovered r, so that
// the stack still holds the frames that panicked.
func newInternalError(r any) *InternalError {
	return &InternalError{Value: r, Stack: debug.Stack()}
}
