// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import "fmt"

// Op identifies the operation that produced an Error.
type Op string

const (
	OpDecode Op = "decode"
	OpEncode Op = "encode"
)

// Error is the error type returned by Decode and Encode operations for
// malformed input or values that cannot be represented. Callers can use
// errors.As to detect it and inspect Op, and errors.Unwrap (via Err) to reach
// an underlying cause such as an error from a TextMarshaler or TextUnmarshaler.
//
// The text returned by the Error method is identical to that of earlier
// versions, so code matching on error strings is unaffected.
type Error struct {
	Op  Op    // the operation that failed
	Err error // the underlying cause, if any; nil when the message originates here
	msg string
}

// Error returns the error message.
func (e *Error) Error() string { return e.msg }

// Unwrap returns the underlying cause, enabling errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Err }

// asError converts a recovered value into an *Error, preserving its message.
func asError(op Op, v interface{}) error {
	switch t := v.(type) {
	case *Error:
		if t.Op == "" {
			t.Op = op
		}
		return t
	case error:
		return &Error{Op: op, Err: t, msg: t.Error()}
	default:
		return &Error{Op: op, msg: fmt.Sprintf("%v", t)}
	}
}
