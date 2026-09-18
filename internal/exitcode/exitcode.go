// Package exitcode defines the process exit codes used by waitfor and a
// small error type that carries a code through the call stack.
package exitcode

import (
	"errors"
	"fmt"
)

const (
	Success    = 0
	Timeout    = 1
	BadArgs    = 2
	Permission = 3
)

// Error is an error with an associated process exit code.
//
// Silent is set when the failure details have already been printed by the
// output layer (e.g. the "✗ Timed out after 30s / Tip: ..." block), so main
// should not print an additional one-line message.
type Error struct {
	Code   int
	Msg    string
	Err    error
	Silent bool
}

func (e *Error) Error() string {
	switch {
	case e.Msg != "" && e.Err != nil:
		return e.Msg + ": " + e.Err.Error()
	case e.Msg != "":
		return e.Msg
	case e.Err != nil:
		return e.Err.Error()
	}
	return "error"
}

func (e *Error) Unwrap() error { return e.Err }

func New(code int, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

func BadArgsf(format string, args ...any) *Error {
	return &Error{Code: BadArgs, Msg: fmt.Sprintf(format, args...)}
}

func Permissionf(format string, args ...any) *Error {
	return &Error{Code: Permission, Msg: fmt.Sprintf(format, args...)}
}

// SilentTimeout returns an error signaling a timeout whose details have
// already been printed by the output layer.
func SilentTimeout() *Error {
	return &Error{Code: Timeout, Silent: true}
}

// CodeOf returns the exit code for an error, defaulting to BadArgs for
// errors that don't carry a code.
func CodeOf(err error) int {
	if err == nil {
		return Success
	}
	if ce, ok := errors.AsType[*Error](err); ok {
		return ce.Code
	}
	return BadArgs
}

// FromErr wraps err in an *Error, preserving an existing *Error if present.
// Untyped errors are assigned the given default code.
func FromErr(err error, defaultCode int) *Error {
	if err == nil {
		return nil
	}
	if ce, ok := errors.AsType[*Error](err); ok {
		return ce
	}
	return &Error{Code: defaultCode, Err: err}
}
