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

// Kind classifies the failure that produced an Error. The zero value means
// the failure is unclassified — typically an error propagated from a custom
// TextMarshaler/TextUnmarshaler or another external source; use Unwrap (via
// errors.As/Is) to reach the cause.
type Kind string

const (
	// KindSyntax reports input that is not well-formed
	// application/x-www-form-urlencoded data.
	KindSyntax Kind = "syntax"

	// KindParse reports a value that cannot be parsed as the destination
	// type (e.g. "abc" into an int, or an invalid time).
	KindParse Kind = "parse"

	// KindUnknownKey reports a key with no corresponding destination field;
	// see Decoder.IgnoreUnknownKeys.
	KindUnknownKey Kind = "unknown-key"

	// KindIndex reports an explicit index that is not a valid position in
	// the destination (malformed, negative, or beyond an array's bounds).
	KindIndex Kind = "index"

	// KindLimit reports input that exceeds a decoder resource bound; see
	// Decoder.MaxSize and Decoder.MaxDepth.
	KindLimit Kind = "limit"

	// KindUnsupported reports a destination or value that cannot be
	// represented as form data (an unsupported kind, an unsettable field, or
	// an invalid destination).
	KindUnsupported Kind = "unsupported"

	// KindCycle reports a self-referential value encountered while encoding.
	KindCycle Kind = "cycle"

	// KindIO reports a failure reading input or writing encoded output.
	KindIO Kind = "io"
)

// Error is the error type returned by Decode and Encode operations for
// malformed input or values that cannot be represented. Callers can use
// errors.As to detect it and inspect Op and Kind, and errors.Unwrap (via
// Err) to reach an underlying cause such as an error from a TextMarshaler or
// TextUnmarshaler.
//
// The text returned by the Error method is identical to that of earlier
// versions, so code matching on error strings is unaffected.
type Error struct {
	Op   Op    // the operation that failed
	Kind Kind  // the class of failure; empty when unclassified
	Err  error // the underlying cause, if any; nil when the message originates here
	msg  string
}

// Error returns the error message.
func (e *Error) Error() string { return e.msg }

// Unwrap returns the underlying cause, enabling errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Err }

// NewError returns an *Error with the given operation, kind, underlying
// cause (which may be nil) and message. It exists so that packages built on
// top of form (such as form/multipart) can return errors that participate in
// the same errors.As taxonomy.
func NewError(op Op, kind Kind, err error, msg string) *Error {
	return &Error{Op: op, Kind: kind, Err: err, msg: msg}
}

// errKind constructs an *Error carrying a kind and message; the operation is
// filled in by asError when the error crosses the recover boundary.
func errKind(kind Kind, msg string) *Error {
	return &Error{Kind: kind, msg: msg}
}

// wrapKind wraps an external error with an operation and kind, preserving
// its message.
func wrapKind(op Op, kind Kind, err error) error {
	return &Error{Op: op, Kind: kind, Err: err, msg: err.Error()}
}

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
